package worker

import (
	"github.com/st0o0/forseti/internal/config"
	"github.com/st0o0/forseti/internal/pihole"
	"github.com/st0o0/forseti/internal/reconcile"
	"github.com/st0o0/forseti/internal/run"
)

type SessionManager interface {
	Get(target config.Target) (*pihole.Client, error)
	Invalidate(name string)
	Acquire(name string)
	TryAcquire(name string) bool
	Release(name string)
}

type SettingsReconciler interface {
	Diff(settings *config.Settings, client *pihole.Client) (*reconcile.SettingsDiff, error)
	Apply(name string, settings *config.Settings, client *pihole.Client) (*reconcile.SettingsDiff, error)
	BuildDesiredList(settings *config.Settings) []reconcile.SettingMapping
}

type ContentReconciler interface {
	Apply(rt *config.ResolvedTarget, client *pihole.Client, opts reconcile.ReconcileOptions) (*reconcile.ApplyReport, error)
}

type GravityTrigger interface {
	TriggerNow(target string, reason string) error
	TriggerAsync(target string, reason string)
}

// Recorder receives run outcomes and target state for metrics.
type Recorder interface {
	// RecordRun records a finished run; skipped runs are ignored.
	RecordRun(r run.Result)
	RecordDrift(target string, drift map[string]int)
	UpdateSettingsDrift(target string, drifted map[string]bool)
	RecordTargetState(target string, s run.TargetState)
}

// Observer is notified after every run, including skipped ones, with the
// target state that resulted from it.
type Observer interface {
	RunFinished(r run.Result, s run.TargetState)
}

// Reachability receives reachability observations (see targetstate.Tracker).
type Reachability interface {
	Observe(target, source string, err error)
}

// Runner performs the mode-specific step of a run with a ready client: it
// fills res.Resources and item errors and returns a fatal error, optionally
// wrapped with StageError to name the failing stage.
type Runner interface {
	Run(client *pihole.Client, res *run.Result) error
}
