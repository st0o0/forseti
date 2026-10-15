package gravity

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/st0o0/forseti/internal/config"
	"github.com/st0o0/forseti/internal/logging"
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

// Reachability receives reachability observations (see targetstate.Tracker).
type Reachability interface {
	Observe(target, source string, err error)
}

type Scheduler struct {
	log       *slog.Logger
	reach     Reachability
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
		log:       logging.Component("gravity"),
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

// SetReachability sets the shared reachability tracker.
func (s *Scheduler) SetReachability(r Reachability) {
	s.reach = r
}

func (s *Scheduler) logScheduled(e entry) {
	s.log.Info("gravity scheduled", "target", e.target.Name, "schedule", e.target.Gravity.Schedule, "next_run", e.next.Format(time.RFC3339))
}

// UpdateTargets replaces the scheduler's target set after a config reload.
// Entries whose schedule is unchanged keep their next run time; new or
// changed schedules are recomputed; removed targets are dropped. In-flight
// gravity runs are not affected.
func (s *Scheduler) UpdateTargets(targets []config.Target) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing := make(map[string]entry, len(s.entries))
	for _, e := range s.entries {
		existing[e.target.Name] = e
	}

	s.targetMap = make(map[string]config.Target, len(targets))
	entries := make([]entry, 0, len(targets))
	now := time.Now()
	for _, t := range targets {
		s.targetMap[t.Name] = t
		if t.Gravity.Schedule == "" {
			continue
		}
		if old, ok := existing[t.Name]; ok && old.target.Gravity.Schedule == t.Gravity.Schedule {
			old.target = t
			entries = append(entries, old)
			continue
		}
		sched, err := ParseSchedule(t.Gravity.Schedule)
		if err != nil {
			s.log.Warn("gravity schedule invalid, keeping previous", "target", t.Name, "schedule", t.Gravity.Schedule, "error", err)
			if old, ok := existing[t.Name]; ok {
				entries = append(entries, old)
			}
			continue
		}
		e := entry{target: t, schedule: sched, next: sched.Next(now)}
		s.logScheduled(e)
		entries = append(entries, e)
	}
	s.entries = entries
}

func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	for _, e := range s.entries {
		s.logScheduled(e)
	}
	s.mu.Unlock()

	// The ticker always runs so that schedules added by a config reload are
	// honoured; with no entries a tick is a no-op.
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.wg.Wait()
			return
		case now := <-ticker.C:
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
		s.log.Warn("gravity request dropped, queue full", "target", targetName, "reason", reason)
	}
}

func (s *Scheduler) processAsync(req triggerRequest) {
	s.mu.Lock()
	defer s.mu.Unlock()

	target, ok := s.targetMap[req.target]
	if !ok {
		s.log.Error("gravity trigger for unknown target", "target", req.target)
		return
	}

	s.triggerLocked(target, req.reason)
}

func (s *Scheduler) TriggerNow(targetName string, reason string) error {
	s.mu.Lock()

	if s.inFlight[targetName] {
		s.mu.Unlock()
		s.log.Debug("gravity skipped, already running", "target", targetName, "reason", reason)
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
		s.log.Debug("gravity skipped, already running", "target", target.Name, "reason", reason)
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
		if s.reach != nil {
			s.reach.Observe(target.Name, "gravity", err)
		}
		s.log.Error("gravity finished", "target", target.Name, "reason", reason, "status", "failed", "error", err)
		if s.recorder != nil {
			s.recorder.RecordGravityRun(target.Name, reason, 0, err)
		}
		return fmt.Errorf("gravity trigger %s: session error: %w", target.Name, err)
	}

	s.log.Info("gravity started", "target", target.Name, "reason", reason)
	start := time.Now()
	err = client.TriggerGravity()
	duration := time.Since(start)

	if err != nil {
		s.log.Error("gravity finished", "target", target.Name, "reason", reason, "status", "failed", "duration", duration.Round(time.Millisecond), "error", err)
	} else {
		s.log.Info("gravity finished", "target", target.Name, "reason", reason, "status", "ok", "duration", duration.Round(time.Millisecond))
	}

	if s.recorder != nil {
		s.recorder.RecordGravityRun(target.Name, reason, duration, err)
	}

	if err != nil {
		return fmt.Errorf("gravity trigger %s: %w", target.Name, err)
	}
	return nil
}
