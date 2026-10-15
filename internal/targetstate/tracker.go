// Package targetstate tracks whether each Pi-hole target answers, shared by
// every subsystem that talks to targets, and logs only reachability changes.
package targetstate

import (
	"log/slog"
	"sync"

	"github.com/st0o0/forseti/internal/logging"
	"github.com/st0o0/forseti/internal/redact"
)

// Reachability receives the reachable gauge updates.
type Reachability interface {
	MarkTargetReachable(target string)
	MarkTargetUnreachable(target string)
}

type state int

const (
	unknown state = iota
	reachable
	unreachable
)

// Tracker records the last observed reachability per target. A nil *Tracker
// is valid and does nothing.
type Tracker struct {
	mu      sync.Mutex
	log     *slog.Logger
	metrics Reachability
	states  map[string]state
}

// New returns a tracker that updates m (may be nil).
func New(m Reachability) *Tracker {
	return &Tracker{
		log:     logging.Component("session"),
		metrics: m,
		states:  make(map[string]state),
	}
}

// Observe records the outcome of contacting target from source ("worker",
// "collector", "gravity", "newdomains"). err == nil means the target answered.
// It logs "target connected" or "target unreachable" only when the state
// changes, and always updates the reachable gauge.
func (t *Tracker) Observe(target, source string, err error) {
	if t == nil {
		return
	}
	next := reachable
	if err != nil {
		next = unreachable
	}

	t.mu.Lock()
	prev := t.states[target]
	t.states[target] = next
	t.mu.Unlock()

	if t.metrics != nil {
		if next == reachable {
			t.metrics.MarkTargetReachable(target)
		} else {
			t.metrics.MarkTargetUnreachable(target)
		}
	}

	if prev == next {
		return
	}
	if next == reachable {
		t.log.Info("target connected", "target", target)
		return
	}
	t.log.Warn("target unreachable", "target", target, "source", source, "error", redact.Text(err.Error()))
}

// Forget drops state for targets no longer configured.
func (t *Tracker) Forget(keep map[string]bool) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	for name := range t.states {
		if !keep[name] {
			delete(t.states, name)
		}
	}
}
