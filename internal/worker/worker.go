package worker

import (
	"log/slog"
	"time"

	"github.com/st0o0/forseti/internal/config"
	"github.com/st0o0/forseti/internal/pihole"
	"github.com/st0o0/forseti/internal/reconcile"
)

type HealthState int

const (
	Healthy  HealthState = iota
	Degraded
	Down
)

func (h HealthState) String() string {
	switch h {
	case Healthy:
		return "healthy"
	case Degraded:
		return "degraded"
	case Down:
		return "down"
	default:
		return "unknown"
	}
}

type TargetHealth struct {
	State            HealthState
	ConsecutiveFails int
	LastError        error
	LastSuccess      time.Time
	BackoffUntil     time.Time
}

type Dependencies struct {
	Sessions  SessionManager
	Settings  SettingsReconciler
	Content   ContentReconciler
	Gravity   GravityTrigger
	Recorder  Recorder
	Interval  time.Duration
	Marker    string
	LocalDNSPurge bool
	CNAMEPurge    bool
	GravityOnChange bool
}

type TargetWorker struct {
	rt   config.ResolvedTarget
	deps Dependencies

	health           HealthState
	consecutiveFails int
	lastError        error
	lastSuccess      time.Time
	backoffUntil     time.Time
}

func NewTargetWorker(rt config.ResolvedTarget, deps Dependencies) *TargetWorker {
	return &TargetWorker{
		rt:     rt,
		deps:   deps,
		health: Healthy,
	}
}

func (w *TargetWorker) Reconcile() error {
	start := time.Now()

	client, err := w.deps.Sessions.Get(w.rt.Target)
	if err != nil {
		slog.Error("session error", "target", w.rt.Name, "error", err)
		w.deps.Sessions.Invalidate(w.rt.Name)
		w.deps.Recorder.MarkTargetUnreachable(w.rt.Name)
		w.recordFailure(err)
		w.deps.Recorder.RecordTargetHealth(w.rt.Name, w.health.String())
		return err
	}

	if err := client.CheckReadiness(); err != nil {
		slog.Error("target not ready", "target", w.rt.Name, "error", err)
		w.deps.Recorder.MarkTargetUnreachable(w.rt.Name)
		w.recordFailure(err)
		return err
	}

	client, err = w.reconcileSettings(client)
	if err != nil {
		w.deps.Recorder.MarkTargetUnreachable(w.rt.Name)
		w.recordFailure(err)
		return err
	}

	report, err := w.deps.Content.Apply(&w.rt, client, reconcile.ReconcileOptions{
		Marker:        w.deps.Marker,
		LocalDNSPurge: w.deps.LocalDNSPurge,
		CNAMEPurge:    w.deps.CNAMEPurge,
	})
	duration := time.Since(start)

	if err != nil {
		if pihole.IsGravityCorrupted(err) {
			slog.Error("gravity database corrupted, skipping target", "target", w.rt.Name, "error", err)
			w.health = Down
		} else {
			slog.Error("reconcile error", "target", w.rt.Name, "error", err)
		}
		w.deps.Recorder.RecordReconcileResult(w.rt.Name, duration, false, nil, nil)
		w.deps.Sessions.Invalidate(w.rt.Name)
		w.deps.Recorder.MarkTargetUnreachable(w.rt.Name)
		w.recordFailure(err)
		return err
	}

	for _, e := range report.Errors {
		slog.Warn("reconcile warning", "target", w.rt.Name, "error", e)
	}

	changes := buildChangesMap(&report.Diff)
	drift := buildDriftMap(&report.Diff)
	w.deps.Recorder.RecordReconcileResult(w.rt.Name, duration, len(report.Errors) == 0, changes, drift)

	if report.Diff.NeedsGravity && w.deps.GravityOnChange {
		if err := client.CheckReadiness(); err != nil {
			slog.Warn("skipping gravity, target not ready", "target", w.rt.Name, "error", err)
		} else {
			w.deps.Gravity.TriggerAsync(w.rt.Name, "adlist_change")
		}
	}

	if hasDiff(&report.Diff) {
		slog.Info("reconciled", "target", w.rt.Name, "duration", duration.Round(time.Millisecond))
	} else {
		slog.Debug("no drift", "target", w.rt.Name, "duration", duration.Round(time.Millisecond))
	}

	w.recordSuccess()
	w.deps.Recorder.RecordTargetHealth(w.rt.Name, w.health.String())
	return nil
}

func (w *TargetWorker) reconcileSettings(client *pihole.Client) (*pihole.Client, error) {
	if w.rt.Settings.IsEmpty() {
		return client, nil
	}

	settingsDiff, err := w.deps.Settings.Diff(&w.rt.Settings, client)
	if err != nil {
		slog.Error("settings diff error", "target", w.rt.Name, "error", err)
	} else {
		w.reportSettingsDrift(settingsDiff)
	}

	if _, err := w.deps.Settings.Apply(w.rt.Name, &w.rt.Settings, client); err != nil {
		slog.Error("settings reconcile error", "target", w.rt.Name, "error", err)
		return client, nil
	}

	if settingsDiff == nil || !settingsDiff.HasChanges() {
		return client, nil
	}

	slog.Info("waiting for FTL ready after settings change", "target", w.rt.Name)
	if err := client.WaitForReady(30*time.Second, 500*time.Millisecond); err != nil {
		slog.Error("FTL not ready after settings change", "target", w.rt.Name, "error", err)
		w.deps.Sessions.Invalidate(w.rt.Name)
		return nil, err
	}

	w.deps.Sessions.Invalidate(w.rt.Name)
	newClient, err := w.deps.Sessions.Get(w.rt.Target)
	if err != nil {
		slog.Error("session error after settings change", "target", w.rt.Name, "error", err)
		w.deps.Sessions.Invalidate(w.rt.Name)
		return nil, err
	}

	if err := newClient.CheckReadiness(); err != nil {
		slog.Error("target not ready after settings change", "target", w.rt.Name, "error", err)
		w.deps.Sessions.Invalidate(w.rt.Name)
		return nil, err
	}

	postDiff, err := w.deps.Settings.Diff(&w.rt.Settings, newClient)
	if err != nil {
		slog.Error("settings post-apply diff error", "target", w.rt.Name, "error", err)
	} else {
		w.reportSettingsDrift(postDiff)
	}

	return newClient, nil
}

func (w *TargetWorker) reportSettingsDrift(diff *reconcile.SettingsDiff) {
	drifted := make(map[string]bool)
	for _, m := range w.deps.Settings.BuildDesiredList(&w.rt.Settings) {
		drifted[m.ForsetiPath] = false
	}
	for _, c := range diff.Changes {
		drifted[c.Name] = true
	}
	w.deps.Recorder.UpdateSettingsDrift(w.rt.Name, drifted)
}

func (w *TargetWorker) UpdateConfig(rt config.ResolvedTarget) {
	w.rt = rt
}

const maxBackoff = 30 * time.Minute

func (w *TargetWorker) ShouldSkip(now time.Time) bool {
	return !w.backoffUntil.IsZero() && now.Before(w.backoffUntil)
}

func (w *TargetWorker) Health() TargetHealth {
	return TargetHealth{
		State:            w.health,
		ConsecutiveFails: w.consecutiveFails,
		LastError:        w.lastError,
		LastSuccess:      w.lastSuccess,
		BackoffUntil:     w.backoffUntil,
	}
}

func (w *TargetWorker) Name() string {
	return w.rt.Name
}

func (w *TargetWorker) recordSuccess() {
	w.health = Healthy
	w.consecutiveFails = 0
	w.lastError = nil
	w.lastSuccess = time.Now()
	w.backoffUntil = time.Time{}
}

func (w *TargetWorker) recordFailure(err error) {
	w.consecutiveFails++
	w.lastError = err
	if w.health != Down {
		w.health = Degraded
	}
	backoff := time.Duration(w.consecutiveFails) * w.deps.Interval
	if backoff > maxBackoff {
		backoff = maxBackoff
	}
	w.backoffUntil = time.Now().Add(backoff)
}

func buildChangesMap(diff *reconcile.DiffReport) map[string]map[string]int {
	m := make(map[string]map[string]int)
	add := func(name string, d reconcile.ResourceDiff) {
		if !d.HasChanges() {
			return
		}
		m[name] = map[string]int{
			"add":    len(d.Adds),
			"update": len(d.Updates),
			"delete": len(d.Deletes),
		}
	}
	add("groups", diff.Groups)
	add("adlists", diff.Adlists)
	add("deny", diff.Deny)
	add("allow", diff.Allow)
	add("local_dns", diff.LocalDNS)
	add("cname", diff.CNAME)
	add("clients", diff.Clients)
	return m
}

func buildDriftMap(diff *reconcile.DiffReport) map[string]int {
	m := make(map[string]int)
	count := func(name string, d reconcile.ResourceDiff) {
		total := len(d.Adds) + len(d.Updates) + len(d.Deletes)
		if total > 0 {
			m[name] = total
		}
	}
	count("groups", diff.Groups)
	count("adlists", diff.Adlists)
	count("deny", diff.Deny)
	count("allow", diff.Allow)
	count("local_dns", diff.LocalDNS)
	count("cname", diff.CNAME)
	count("clients", diff.Clients)
	return m
}

func hasDiff(diff *reconcile.DiffReport) bool {
	return diff.Groups.HasChanges() || diff.Adlists.HasChanges() ||
		diff.Deny.HasChanges() || diff.Allow.HasChanges() ||
		diff.LocalDNS.HasChanges() || diff.CNAME.HasChanges() ||
		diff.Clients.HasChanges()
}
