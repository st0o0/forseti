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

type TriggerReason string

const (
	ReasonScheduled    TriggerReason = "scheduled"
	ReasonAdlistChange TriggerReason = "adlist_change"
)

type Recorder interface {
	RecordGravityRun(target string, trigger string, duration time.Duration, err error)
}

type entry struct {
	target   config.Target
	schedule *Schedule
	next     time.Time
}

type Scheduler struct {
	pool       *session.Pool
	entries    []entry
	recorder   Recorder
	targetMap  map[string]config.Target
	mu         sync.Mutex
}

func NewScheduler(pool *session.Pool, targets []config.Target, recorder Recorder) (*Scheduler, error) {
	s := &Scheduler{
		pool:      pool,
		recorder:  recorder,
		targetMap: make(map[string]config.Target, len(targets)),
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
	if len(s.entries) == 0 {
		return
	}

	for _, e := range s.entries {
		slog.Debug("gravity scheduled", "target", e.target.Name, "next_run", e.next.Format(time.RFC3339))
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			s.mu.Lock()
			for i := range s.entries {
				e := &s.entries[i]
				if e.next.IsZero() || now.Before(e.next) {
					continue
				}
				s.trigger(e, ReasonScheduled)
				e.next = e.schedule.Next(now)
			}
			s.mu.Unlock()
		}
	}
}

func (s *Scheduler) TriggerNow(targetName string, reason TriggerReason) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.entries {
		if s.entries[i].target.Name == targetName {
			s.trigger(&s.entries[i], reason)
			return nil
		}
	}

	target, ok := s.targetMap[targetName]
	if !ok {
		return fmt.Errorf("gravity trigger: unknown target %q", targetName)
	}

	client, err := s.pool.Get(target)
	if err != nil {
		return fmt.Errorf("gravity trigger %s: session error: %w", targetName, err)
	}

	start := time.Now()
	err = client.TriggerGravity()
	duration := time.Since(start)
	if s.recorder != nil {
		s.recorder.RecordGravityRun(targetName, string(reason), duration, err)
	}
	if err != nil {
		return fmt.Errorf("gravity trigger %s: %w", targetName, err)
	}
	return nil
}

func (s *Scheduler) trigger(e *entry, reason TriggerReason) {
	slog.Info("triggering gravity", "target", e.target.Name, "reason", reason)

	client, err := s.pool.Get(e.target)
	if err != nil {
		slog.Error("gravity session error", "target", e.target.Name, "error", err)
		if s.recorder != nil {
			s.recorder.RecordGravityRun(e.target.Name, string(reason), 0, err)
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
		s.recorder.RecordGravityRun(e.target.Name, string(reason), duration, err)
	}
}
