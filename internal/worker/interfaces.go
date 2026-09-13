package worker

import (
	"time"

	"github.com/st0o0/forseti/internal/config"
	"github.com/st0o0/forseti/internal/pihole"
	"github.com/st0o0/forseti/internal/reconcile"
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

type Recorder interface {
	RecordReconcileResult(target string, duration time.Duration, success bool, changes map[string]map[string]int, drift map[string]int)
	UpdateSettingsDrift(target string, drifted map[string]bool)
	MarkTargetUnreachable(target string)
	RecordTargetHealth(target string, state string)
}
