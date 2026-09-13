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
	pool       *session.Pool
	entries    []entry
	recorder   Recorder
	targetMap  map[string]config.Target
	mu         sync.Mutex
	inFlight   map[string]bool
	asyncCh    chan triggerRequest
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
			return
		case now := <-tickerCh:
			s.mu.Lock()
			for i := range s.entries {
				e := &s.entries[i]
				if e.next.IsZero() || now.Before(e.next) {
					continue
				}
				s.triggerLocked(e, ReasonScheduled)
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

	if s.inFlight[req.target] {
		slog.Warn("gravity already running, skipping", "target", req.target, "reason", req.reason)
		return
	}

	for i := range s.entries {
		if s.entries[i].target.Name == req.target {
			s.triggerLocked(&s.entries[i], req.reason)
			return
		}
	}

	target, ok := s.targetMap[req.target]
	if !ok {
		slog.Error("gravity trigger: unknown target", "target", req.target)
		return
	}

	s.inFlight[req.target] = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.inFlight, req.target)
	}()

	s.pool.Acquire(req.target)
	defer s.pool.Release(req.target)

	client, err := s.pool.Get(target)
	if err != nil {
		s.pool.Invalidate(req.target)
		slog.Error("gravity session error", "target", req.target, "error", err)
		return
	}

	slog.Info("triggering gravity", "target", req.target, "reason", req.reason)
	start := time.Now()
	err = client.TriggerGravity()
	duration := time.Since(start)

	if err != nil {
		slog.Error("gravity error", "target", req.target, "error", err)
	} else {
		slog.Info("gravity completed", "target", req.target, "duration", duration.Round(time.Millisecond))
	}

	if s.recorder != nil {
		s.recorder.RecordGravityRun(req.target, req.reason, duration, err)
	}
}

func (s *Scheduler) TriggerNow(targetName string, reason string) error {
	s.mu.Lock()

	if s.inFlight[targetName] {
		s.mu.Unlock()
		slog.Warn("gravity already running, skipping", "target", targetName, "reason", reason)
		return nil
	}

	for i := range s.entries {
		if s.entries[i].target.Name == targetName {
			s.triggerLocked(&s.entries[i], reason)
			s.mu.Unlock()
			return nil
		}
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

	s.pool.Acquire(targetName)
	defer s.pool.Release(targetName)

	client, err := s.pool.Get(target)
	if err != nil {
		s.pool.Invalidate(targetName)
		return fmt.Errorf("gravity trigger %s: session error: %w", targetName, err)
	}

	start := time.Now()
	err = client.TriggerGravity()
	duration := time.Since(start)
	if s.recorder != nil {
		s.recorder.RecordGravityRun(targetName, reason, duration, err)
	}
	if err != nil {
		return fmt.Errorf("gravity trigger %s: %w", targetName, err)
	}
	return nil
}

func (s *Scheduler) triggerLocked(e *entry, reason string) {
	if s.inFlight[e.target.Name] {
		slog.Warn("gravity already running, skipping", "target", e.target.Name, "reason", reason)
		return
	}

	slog.Info("triggering gravity", "target", e.target.Name, "reason", reason)
	s.inFlight[e.target.Name] = true

	defer func() {
		delete(s.inFlight, e.target.Name)
	}()

	s.pool.Acquire(e.target.Name)
	defer s.pool.Release(e.target.Name)

	client, err := s.pool.Get(e.target)
	if err != nil {
		slog.Error("gravity session error", "target", e.target.Name, "error", err)
		s.pool.Invalidate(e.target.Name)
		if s.recorder != nil {
			s.recorder.RecordGravityRun(e.target.Name, reason, 0, err)
		}
		return
	}

	start := time.Now()
	err = client.TriggerGravity()
	duration := time.Since(start)

	if err != nil {
		slog.Error("gravity error", "target", e.target.Name, "error", err)
	} else {
		slog.Info("gravity completed", "target", e.target.Name, "duration", duration.Round(time.Millisecond))
	}

	if s.recorder != nil {
		s.recorder.RecordGravityRun(e.target.Name, reason, duration, err)
	}
}
