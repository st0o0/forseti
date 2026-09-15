package gravity

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/st0o0/forseti/internal/config"
	"github.com/st0o0/forseti/internal/session"
)

const (
	ReasonScheduled    = "scheduled"
	ReasonAdlistChange = "adlist_change"
)

type Recorder interface {
	RecordGravityRun(target string, trigger string, duration time.Duration, err error)
}

type entry struct {
	target   config.Target
	schedule *Schedule
	next     time.Time
}

type triggerRequest struct {
	target string
	reason string
}

type Scheduler struct {
	pool      *session.Pool
	entries   []entry
	recorder  Recorder
	targetMap map[string]config.Target
	mu        sync.Mutex
	inFlight  map[string]bool
	asyncCh   chan triggerRequest
	wg        sync.WaitGroup
}

func NewScheduler(pool *session.Pool, targets []config.Target, recorder Recorder) (*Scheduler, error) {
	s := &Scheduler{
		pool:      pool,
		recorder:  recorder,
		targetMap: make(map[string]config.Target, len(targets)),
		inFlight:  make(map[string]bool),
		asyncCh:   make(chan triggerRequest, 16),
	}
	for _, t := range targets {
		s.targetMap[t.Name] = t
	}

	for _, t := range targets {
		if t.Gravity.Schedule == "" {
			continue
		}
		sched, err := ParseSchedule(t.Gravity.Schedule)
		if err != nil {
			return nil, fmt.Errorf("target %s: %w", t.Name, err)
		}
		s.entries = append(s.entries, entry{
			target:   t,
			schedule: sched,
			next:     sched.Next(time.Now()),
		})
	}

	return s, nil
}

func (s *Scheduler) Start(ctx context.Context) {
	for _, e := range s.entries {
		slog.Debug("gravity scheduled", "target", e.target.Name, "next_run", e.next.Format(time.RFC3339))
	}

	var tickerCh <-chan time.Time
	var ticker *time.Ticker
	if len(s.entries) > 0 {
		ticker = time.NewTicker(30 * time.Second)
		tickerCh = ticker.C
		defer ticker.Stop()
	}

	for {
		select {
		case <-ctx.Done():
			s.wg.Wait()
			return
		case now := <-tickerCh:
			s.mu.Lock()
			for i := range s.entries {
				e := &s.entries[i]
				if e.next.IsZero() || now.Before(e.next) {
					continue
				}
				s.triggerLocked(e.target, ReasonScheduled)
				e.next = e.schedule.Next(now)
			}
			s.mu.Unlock()
		case req := <-s.asyncCh:
			s.processAsync(req)
		}
	}
}

func (s *Scheduler) TriggerAsync(targetName string, reason string) {
	select {
	case s.asyncCh <- triggerRequest{target: targetName, reason: reason}:
	default:
		slog.Warn("gravity async channel full, dropping request", "target", targetName, "reason", reason)
	}
}

func (s *Scheduler) processAsync(req triggerRequest) {
	s.mu.Lock()
	defer s.mu.Unlock()

	target, ok := s.targetMap[req.target]
	if !ok {
		slog.Error("gravity trigger: unknown target", "target", req.target)
		return
	}

	s.triggerLocked(target, req.reason)
}

func (s *Scheduler) TriggerNow(targetName string, reason string) error {
	s.mu.Lock()

	if s.inFlight[targetName] {
		s.mu.Unlock()
		slog.Warn("gravity already running, skipping", "target", targetName, "reason", reason)
		return nil
	}

	target, ok := s.targetMap[targetName]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("gravity trigger: unknown target %q", targetName)
	}

	s.inFlight[targetName] = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.inFlight, targetName)
		s.mu.Unlock()
	}()

	return s.runGravity(target, reason)
}

// triggerLocked spawns an async gravity run. Caller must hold s.mu.
func (s *Scheduler) triggerLocked(target config.Target, reason string) {
	if s.inFlight[target.Name] {
		slog.Warn("gravity already running, skipping", "target", target.Name, "reason", reason)
		return
	}

	s.inFlight[target.Name] = true
	s.wg.Add(1)

	go func() {
		defer s.wg.Done()
		defer func() {
			s.mu.Lock()
			delete(s.inFlight, target.Name)
			s.mu.Unlock()
		}()

		_ = s.runGravity(target, reason)
	}()
}

func (s *Scheduler) runGravity(target config.Target, reason string) error {
	s.pool.Acquire(target.Name)
	defer s.pool.Release(target.Name)

	client, err := s.pool.Get(target)
	if err != nil {
		s.pool.Invalidate(target.Name)
		slog.Error("gravity session error", "target", target.Name, "error", err)
		if s.recorder != nil {
			s.recorder.RecordGravityRun(target.Name, reason, 0, err)
		}
		return fmt.Errorf("gravity trigger %s: session error: %w", target.Name, err)
	}

	slog.Info("triggering gravity", "target", target.Name, "reason", reason)
	start := time.Now()
	err = client.TriggerGravity()
	duration := time.Since(start)

	if err != nil {
		slog.Error("gravity error", "target", target.Name, "error", err)
	} else {
		slog.Info("gravity completed", "target", target.Name, "duration", duration.Round(time.Millisecond))
	}

	if s.recorder != nil {
		s.recorder.RecordGravityRun(target.Name, reason, duration, err)
	}

	if err != nil {
		return fmt.Errorf("gravity trigger %s: %w", target.Name, err)
	}
	return nil
}
