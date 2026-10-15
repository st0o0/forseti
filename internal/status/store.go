// Package status keeps a bounded, in-memory history of recent runs and cycle
// summaries and serves it as read-only JSON (/status, /status/history).
// History does not survive a restart.
package status

import (
	"sort"
	"sync"
	"time"

	"github.com/st0o0/forseti/internal/redact"
	"github.com/st0o0/forseti/internal/run"
)

// DefaultCapacity is the number of runs kept per target and of cycles kept.
const DefaultCapacity = 1440

// maxErrors bounds the error messages stored per entry.
const maxErrors = 3

// Target identifies a configured target.
type Target struct {
	Name string
	Role string
}

// Counts are per-resource write counts of one run.
type Counts struct {
	Added   int `json:"added"`
	Updated int `json:"updated"`
	Deleted int `json:"deleted"`
	Failed  int `json:"failed"`
}

// Entry is one run of one target.
type Entry struct {
	At         time.Time         `json:"at"`
	Status     run.Status        `json:"status"`
	Stage      string            `json:"stage,omitempty"`
	DurationMs int64             `json:"duration_ms"`
	Resources  map[string]Counts `json:"resources,omitempty"`
	Errors     []string          `json:"errors,omitempty"`
}

// CycleEntry is one cycle summary.
type CycleEntry struct {
	At         time.Time `json:"at"`
	DurationMs int64     `json:"duration_ms"`
	Targets    int       `json:"targets"`
	OK         int       `json:"ok"`
	Partial    int       `json:"partial"`
	Failed     int       `json:"failed"`
	Skipped    int       `json:"skipped"`
	Changes    int       `json:"changes"`
	NextRun    time.Time `json:"next_run_at"`
}

type targetState struct {
	role        string
	state       run.TargetState
	lastError   string
	lastErrorAt time.Time
	history     *ring[Entry]
}

type reloadInfo struct {
	at  time.Time
	ok  bool
	err string
}

// Store is safe for concurrent use: the watch loop writes, HTTP handlers read.
type Store struct {
	mu       sync.RWMutex
	capacity int
	version  string
	mode     string
	started  time.Time
	loadedAt time.Time
	interval time.Duration
	reload   *reloadInfo
	targets  map[string]*targetState
	cycles   *ring[CycleEntry]
}

// New returns an empty store.
func New(version, mode string, capacity int) *Store {
	if capacity <= 0 {
		capacity = DefaultCapacity
	}
	now := time.Now()
	return &Store{
		capacity: capacity,
		version:  version,
		mode:     mode,
		started:  now,
		loadedAt: now,
		targets:  make(map[string]*targetState),
		cycles:   newRing[CycleEntry](capacity),
	}
}

// SetTargets replaces the configured target set. History of removed targets
// is dropped; new targets start empty.
func (s *Store) SetTargets(targets []Target) {
	s.mu.Lock()
	defer s.mu.Unlock()
	keep := make(map[string]bool, len(targets))
	for _, t := range targets {
		keep[t.Name] = true
		if ts, ok := s.targets[t.Name]; ok {
			ts.role = t.Role
			continue
		}
		s.targets[t.Name] = &targetState{role: t.Role, state: run.TargetState{Health: "healthy"}, history: newRing[Entry](s.capacity)}
	}
	for name := range s.targets {
		if !keep[name] {
			delete(s.targets, name)
		}
	}
}

// SetInterval records the loop interval reported in /status.
func (s *Store) SetInterval(d time.Duration) {
	s.mu.Lock()
	s.interval = d
	s.mu.Unlock()
}

// RunFinished records a run result and the resulting target state. Results
// for unknown targets are ignored.
func (s *Store) RunFinished(r run.Result, st run.TargetState) {
	e := entryFrom(r)
	s.mu.Lock()
	defer s.mu.Unlock()
	ts, ok := s.targets[r.Target]
	if !ok {
		return
	}
	ts.history.push(e)
	ts.state = st
	ts.state.LastError = nil
	if r.Status == run.Failed || r.Status == run.Partial {
		if errs := r.ErrorStrings(1); len(errs) > 0 {
			ts.lastError = errs[0]
			ts.lastErrorAt = e.At
		}
	}
}

// CycleFinished records a cycle summary and the next run time.
func (s *Store) CycleFinished(c run.Cycle, nextRun time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cycles.push(CycleEntry{
		At:         time.Now(),
		DurationMs: c.Duration.Milliseconds(),
		Targets:    c.Targets,
		OK:         c.OK,
		Partial:    c.Partial,
		Failed:     c.Failed,
		Skipped:    c.Skipped,
		Changes:    c.Changes,
		NextRun:    nextRun,
	})
}

// ConfigReloaded records the outcome of a config reload.
func (s *Store) ConfigReloaded(ok bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	ri := &reloadInfo{at: now, ok: ok}
	if err != nil {
		ri.err = redact.Text(err.Error())
	}
	s.reload = ri
	if ok {
		s.loadedAt = now
	}
}

func entryFrom(r run.Result) Entry {
	e := Entry{
		At:         r.Started,
		Status:     r.Status,
		Stage:      r.Stage,
		DurationMs: r.Duration.Milliseconds(),
		Errors:     r.ErrorStrings(maxErrors),
	}
	if e.At.IsZero() {
		e.At = time.Now()
	}
	for name, c := range r.Resources {
		if c == (run.Counts{}) {
			continue
		}
		if e.Resources == nil {
			e.Resources = make(map[string]Counts)
		}
		e.Resources[name] = Counts(c)
	}
	return e
}

// History returns runs of target within since, oldest first, keeping at
// most limit of the most recent. ok is false for an unknown target.
func (s *Store) History(target string, since time.Duration, limit int) ([]Entry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ts, ok := s.targets[target]
	if !ok {
		return nil, false
	}
	cutoff := time.Now().Add(-since)
	return tail(ts.history.items(), func(e Entry) bool { return !e.At.Before(cutoff) }, limit), true
}

// Cycles returns cycle summaries within since, oldest first, at most limit.
func (s *Store) Cycles(since time.Duration, limit int) []CycleEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cutoff := time.Now().Add(-since)
	return tail(s.cycles.items(), func(c CycleEntry) bool { return !c.At.Before(cutoff) }, limit)
}

func tail[T any](items []T, keep func(T) bool, limit int) []T {
	out := make([]T, 0, len(items))
	for _, it := range items {
		if keep(it) {
			out = append(out, it)
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out
}

func (s *Store) targetNames() []string {
	names := make([]string, 0, len(s.targets))
	for n := range s.targets {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
