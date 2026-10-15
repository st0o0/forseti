package worker

import (
	"errors"
	"log/slog"
	"time"

	"github.com/st0o0/forseti/internal/config"
	"github.com/st0o0/forseti/internal/logging"
	"github.com/st0o0/forseti/internal/pihole"
	"github.com/st0o0/forseti/internal/redact"
	"github.com/st0o0/forseti/internal/run"
)

type HealthState int

const (
	Healthy HealthState = iota
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

// downThreshold is the number of consecutive failed runs after which a
// target is considered down.
const downThreshold = 5

const maxBackoff = 30 * time.Minute

type TargetHealth struct {
	State            HealthState
	ConsecutiveFails int
	LastError        error
	LastSuccess      time.Time
	BackoffUntil     time.Time
}

type Dependencies struct {
	Sessions        SessionManager
	Settings        SettingsReconciler
	Content         ContentReconciler
	Gravity         GravityTrigger
	Recorder        Recorder
	Reachability    Reachability
	Interval        time.Duration
	Marker          string
	LocalDNSPurge   bool
	CNAMEPurge      bool
	GravityOnChange bool
	// Log is the logger for run and health events; defaults to
	// component=worker.
	Log *slog.Logger
}

type TargetWorker struct {
	rt   config.ResolvedTarget
	deps Dependencies
	log  *slog.Logger

	health           HealthState
	consecutiveFails int
	lastError        error
	lastSuccess      time.Time
	failingSince     time.Time
	backoffUntil     time.Time
}

func NewTargetWorker(rt config.ResolvedTarget, deps Dependencies) *TargetWorker {
	log := deps.Log
	if log == nil {
		log = logging.Component("worker")
	}
	return &TargetWorker{
		rt:     rt,
		deps:   deps,
		log:    log,
		health: Healthy,
	}
}

// stageError names the stage at which a runner failed.
type stageError struct {
	stage       string
	unreachable bool
	err         error
}

func (e *stageError) Error() string { return e.err.Error() }
func (e *stageError) Unwrap() error { return e.err }

// StageError wraps err with the run stage it occurred in. If unreachable is
// true the target is reported as unreachable.
func StageError(stage string, unreachable bool, err error) error {
	return &stageError{stage: stage, unreachable: unreachable, err: err}
}

// Reconcile runs a config-mode cycle for this target and returns the fatal
// error of a failed run.
func (w *TargetWorker) Reconcile() error {
	res := w.Run(&ConfigRunner{w: w})
	if res.Status == run.Failed {
		return res.Err
	}
	return nil
}

// Run executes one run through the shared envelope: acquire gate, get
// session, check readiness, run the mode-specific step, then classify the
// outcome, record metrics, log and update health.
func (w *TargetWorker) Run(r Runner) run.Result {
	name := w.rt.Name
	res := run.Result{Target: name, Role: w.rt.Role, Started: time.Now()}

	w.deps.Sessions.Acquire(name)
	defer w.deps.Sessions.Release(name)

	client, err := w.deps.Sessions.Get(w.rt.Target)
	if err != nil {
		w.deps.Sessions.Invalidate(name)
		w.observe(err)
		res.Fail(run.StageSession, err)
		return w.finish(res, false)
	}

	if err := client.CheckReadiness(); err != nil {
		w.observe(err)
		res.Fail(run.StageReadiness, err)
		return w.finish(res, false)
	}
	w.observe(nil)

	if err := r.Run(client, &res); err != nil {
		stage := run.StageApply
		var se *stageError
		if errors.As(err, &se) {
			stage = se.stage
			if se.unreachable {
				w.observe(err)
			}
		}
		w.deps.Sessions.Invalidate(name)
		res.Fail(stage, err)
		return w.finish(res, pihole.IsGravityCorrupted(err))
	}
	return w.finish(res, false)
}

// Skip returns a skipped result (backoff, primary unavailable) without
// touching health. It is logged at debug level and not recorded in metrics.
func (w *TargetWorker) Skip(stage string) run.Result {
	res := run.Result{Target: w.rt.Name, Role: w.rt.Role, Started: time.Now(), Status: run.Skipped, Stage: stage}
	res.Finish()
	attrs := []any{"target", res.Target, "status", res.Status, "stage", stage}
	if stage == run.StageBackoff {
		attrs = append(attrs, "retry_at", w.backoffUntil.Format(time.RFC3339))
	}
	w.log.Debug("run finished", attrs...)
	return res
}

func (w *TargetWorker) observe(err error) {
	if w.deps.Reachability != nil {
		w.deps.Reachability.Observe(w.rt.Name, "worker", err)
	}
}

// finish classifies the result, records it, logs it and updates health.
func (w *TargetWorker) finish(res run.Result, corrupted bool) run.Result {
	res.Finish()
	w.deps.Recorder.RecordRun(res)
	w.logRun(res)

	if res.Status == run.Failed {
		w.recordFailure(res.Err, corrupted)
	} else {
		w.recordSuccess()
	}
	w.deps.Recorder.RecordTargetHealth(w.rt.Name, w.health.String())
	return res
}

func (w *TargetWorker) logRun(res run.Result) {
	attrs := []any{"target", res.Target}
	if res.Role != "" {
		attrs = append(attrs, "role", res.Role)
	}
	attrs = append(attrs, "status", res.Status, "duration", res.Duration.Round(time.Millisecond))
	if res.Stage != "" {
		attrs = append(attrs, "stage", res.Stage)
	}
	// Inline the counts (added=.. clients.added=..) as top-level attributes.
	attrs = append(attrs, slog.Attr{Key: "", Value: res.LogValue()})
	if errs := res.ErrorStrings(3); len(errs) > 0 {
		attrs = append(attrs, "errors", errs)
	}

	switch res.Status {
	case run.Failed:
		attrs = append(attrs, "consecutive_failures", w.consecutiveFails+1)
		w.log.Error("run finished", attrs...)
	case run.Partial:
		w.log.Warn("run finished", attrs...)
	default:
		if res.Totals().Changes() > 0 {
			w.log.Info("run finished", attrs...)
		} else {
			w.log.Debug("run finished", attrs...)
		}
	}
}

func (w *TargetWorker) UpdateConfig(rt config.ResolvedTarget) {
	w.rt = rt
}

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
	if w.health != Healthy {
		w.log.Info("target recovered",
			"target", w.rt.Name,
			"from", w.health.String(),
			"failures", w.consecutiveFails,
			"downtime", time.Since(w.failingSince).Round(time.Second),
		)
	}
	w.health = Healthy
	w.consecutiveFails = 0
	w.lastError = nil
	w.lastSuccess = time.Now()
	w.failingSince = time.Time{}
	w.backoffUntil = time.Time{}
}

func (w *TargetWorker) recordFailure(err error, corrupted bool) {
	prev := w.health
	w.consecutiveFails++
	w.lastError = err
	if w.consecutiveFails == 1 {
		w.failingSince = time.Now()
	}

	switch {
	case corrupted || w.consecutiveFails >= downThreshold:
		w.health = Down
	case w.health != Down:
		w.health = Degraded
	}

	backoff := time.Duration(w.consecutiveFails) * w.deps.Interval
	if backoff > maxBackoff {
		backoff = maxBackoff
	}
	w.backoffUntil = time.Now().Add(backoff)

	if prev == w.health {
		return
	}
	msg := ""
	if err != nil {
		msg = redact.Text(err.Error())
	}
	switch w.health {
	case Degraded:
		w.log.Warn("target degraded",
			"target", w.rt.Name,
			"consecutive_failures", w.consecutiveFails,
			"retry_at", w.backoffUntil.Format(time.RFC3339),
			"error", msg,
		)
	case Down:
		attrs := []any{"target", w.rt.Name, "consecutive_failures", w.consecutiveFails, "error", msg}
		if corrupted {
			attrs = append(attrs, "reason", "gravity_corrupted")
		}
		w.log.Error("target down", attrs...)
	}
}
