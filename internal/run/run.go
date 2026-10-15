// Package run defines the outcome of running one target, shared by config
// mode (reconcile) and sync mode (replica sync).
package run

import (
	"log/slog"
	"sort"
	"time"

	"github.com/st0o0/forseti/internal/redact"
)

// Status classifies a run.
type Status string

const (
	// OK: the run completed without errors (with or without changes).
	OK Status = "ok"
	// Partial: the target was reached but at least one item write failed.
	Partial Status = "partial"
	// Failed: the run could not complete.
	Failed Status = "failed"
	// Skipped: the run was not attempted (backoff, primary unavailable).
	Skipped Status = "skipped"
)

// Stages at which a run can fail or be skipped.
const (
	StageSession   = "session"
	StageReadiness = "readiness"
	StageSettings  = "settings"
	StageLoad      = "load"
	StageApply     = "apply"
	StageBackoff   = "backoff"
	StagePrimary   = "primary"
)

// maxErrors bounds how many item errors a result keeps.
const maxErrors = 10

// Counts holds per-resource change counts. Added/Updated/Deleted count
// applied writes only; Failed counts writes that failed.
type Counts struct {
	Added   int
	Updated int
	Deleted int
	Failed  int
}

// Changes returns the number of applied writes.
func (c Counts) Changes() int { return c.Added + c.Updated + c.Deleted }

// Result is the outcome of one run against one target.
type Result struct {
	Target   string
	Role     string
	Started  time.Time
	Duration time.Duration
	Status   Status
	// Stage names where a failed or skipped run stopped.
	Stage string
	// Err is the fatal error of a failed run.
	Err error
	// Resources maps resource type (groups, adlists, deny, ...) to counts.
	Resources map[string]Counts
	// Errors holds item-level errors (first maxErrors); ErrorCount has all.
	Errors     []error
	ErrorCount int
	// Unattributed counts item-level errors not tied to a resource write
	// (recorded via AddError rather than Record).
	Unattributed int
}

// Record accounts for one write of the given resource and action
// ("add", "update", "delete"). n is the number of items the write covered.
func (r *Result) Record(resource, action string, n int, err error) {
	if n <= 0 {
		return
	}
	if r.Resources == nil {
		r.Resources = make(map[string]Counts)
	}
	c := r.Resources[resource]
	if err != nil {
		c.Failed += n
		r.addError(err)
	} else {
		switch action {
		case "add":
			c.Added += n
		case "update":
			c.Updated += n
		case "delete":
			c.Deleted += n
		}
	}
	r.Resources[resource] = c
}

// AddError records an item-level error not tied to a counted write.
func (r *Result) AddError(err error) {
	r.Unattributed++
	r.addError(err)
}

func (r *Result) addError(err error) {
	r.ErrorCount++
	if len(r.Errors) < maxErrors {
		r.Errors = append(r.Errors, err)
	}
}

// Fail marks the run failed at the given stage.
func (r *Result) Fail(stage string, err error) {
	r.Status = Failed
	r.Stage = stage
	r.Err = err
}

// Finish classifies a run that reached its target: partial if any item
// errors were recorded, otherwise ok. It sets Duration from Started.
func (r *Result) Finish() {
	if r.Status == "" {
		r.Status = OK
		if r.ErrorCount > 0 {
			r.Status = Partial
		}
	}
	r.Duration = time.Since(r.Started)
}

// Totals sums counts across resources.
func (r Result) Totals() Counts {
	var t Counts
	for _, c := range r.Resources {
		t.Added += c.Added
		t.Updated += c.Updated
		t.Deleted += c.Deleted
		t.Failed += c.Failed
	}
	return t
}

// ChangeMap returns applied changes as resource -> action -> count, the shape
// used by forseti_reconcile_changes_total. Zero counts are omitted.
func (r Result) ChangeMap() map[string]map[string]int {
	m := make(map[string]map[string]int)
	for res, c := range r.Resources {
		actions := map[string]int{}
		if c.Added > 0 {
			actions["add"] = c.Added
		}
		if c.Updated > 0 {
			actions["update"] = c.Updated
		}
		if c.Deleted > 0 {
			actions["delete"] = c.Deleted
		}
		if len(actions) > 0 {
			m[res] = actions
		}
	}
	return m
}

// LogValue renders totals and non-zero per-resource counts, e.g.
// added=3 updated=0 deleted=1 failed=0 clients.added=2 adlists.added=1.
func (r Result) LogValue() slog.Value {
	t := r.Totals()
	attrs := []slog.Attr{
		slog.Int("added", t.Added),
		slog.Int("updated", t.Updated),
		slog.Int("deleted", t.Deleted),
		slog.Int("failed", t.Failed),
	}
	names := make([]string, 0, len(r.Resources))
	for name := range r.Resources {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		c := r.Resources[name]
		for _, kv := range []struct {
			k string
			v int
		}{{"added", c.Added}, {"updated", c.Updated}, {"deleted", c.Deleted}, {"failed", c.Failed}} {
			if kv.v > 0 {
				attrs = append(attrs, slog.Int(name+"."+kv.k, kv.v))
			}
		}
	}
	return slog.GroupValue(attrs...)
}

// ErrorStrings returns up to n sanitized error messages: the fatal error
// first, then item errors.
func (r Result) ErrorStrings(n int) []string {
	var out []string
	if r.Err != nil {
		out = append(out, redact.Text(r.Err.Error()))
	}
	for _, e := range r.Errors {
		if len(out) >= n {
			break
		}
		out = append(out, redact.Text(e.Error()))
	}
	return out
}

// MetricStatus maps a run status to the forseti_reconcile_runs_total label.
// Skipped runs are not recorded and map to "".
func (s Status) MetricStatus() string {
	switch s {
	case OK:
		return "success"
	case Partial:
		return "partial"
	case Failed:
		return "error"
	}
	return ""
}

// Cycle summarizes one reconcile or sync cycle across targets.
type Cycle struct {
	Targets, OK, Partial, Failed, Skipped, Changes int
	Duration                                       time.Duration
}

// Add accounts one run result in the cycle summary.
func (c *Cycle) Add(r Result) {
	c.Targets++
	switch r.Status {
	case OK:
		c.OK++
	case Partial:
		c.Partial++
	case Failed:
		c.Failed++
	case Skipped:
		c.Skipped++
	}
	c.Changes += r.Totals().Changes()
}

// TargetState is a target's health after a run, as tracked by its worker.
type TargetState struct {
	Health              string
	ConsecutiveFailures int
	LastSuccess         time.Time
	BackoffUntil        time.Time
	LastError           error
}
