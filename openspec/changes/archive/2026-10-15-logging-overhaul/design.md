## Context

See proposal.md. After `fix-observability-bugs`, run outcomes and metrics are correct, but:

- Logging uses `slog.Default()` everywhere; `setupLogger` (`cmd/forseti/main.go`) rebuilds the handler on every reload; config warnings (`internal/config/merge.go`) are emitted before `setupLogger` runs.
- Config mode runs targets through `worker.TargetWorker.Reconcile` (gate, session, readiness, settings, content, health, backoff). Sync mode runs `sync.Syncer.SyncAll` with none of that.
- Reachability is reported by four subsystems independently (worker, collector, new-domains, gravity), each logging its own errors on every attempt.
- `reconcile.Apply` returns `[]error` without resource/action, so config-mode change counters are built from the plan.
- `forseti healthcheck` logs into every Pi-hole.

## Goals / Non-Goals

**Goals:**
- One run envelope and one run result type for both modes.
- Every line has `component`; lifecycle events per the `lifecycle-events` spec.
- Transition-only logging for reachability and health.

**Non-Goals:**
- Run history, `/status` and new metrics (`run-status-endpoint` change).
- A swappable log format at runtime.
- Per-call context propagation (`context.Context` loggers).

## Decisions

### Logger construction: factory + LevelVar, captured at construction
`internal/logging` exposes `Setup(format string, level slog.Level, out io.Writer)`, `SetLevel(slog.Level)` and `Component(name) *slog.Logger`. `Setup` builds one handler bound to a package-level `slog.LevelVar` and calls `slog.SetDefault`. Components call `logging.Component("gravity")` in their constructors and keep the logger in a field.
- Because the handler reads the `LevelVar`, a reload only calls `SetLevel`; loggers captured earlier see the new level. The format is fixed after `Setup`; a reload that changes it logs a warning.
- *Alternative*: pass `*slog.Logger` through every constructor (full DI). Rejected for now: it changes ~40 call sites and test helpers for no behavioural gain, since the handler never changes after startup. Components still own their logger instance, so moving to DI later is mechanical.
- `logtest.New(t)` installs a recording handler as default for the test and returns a recorder with `Find(msg)`; tests using it are not parallel.

### Bootstrap order
`main` calls `logging.Setup("text", info, stderr-or-stdout)` first, loads the config, then calls `Setup` again with the configured format/level before any goroutine starts. `config.Load` no longer logs; resolution warnings are collected per target in `ResolvedTarget.Warnings []string` (they arise while resolving a target's overrides) and logged by `main` as `config warning`. No signature change for `config.Load`.

### Pi-hole client logging
`pihole.Client` gets an exported `Log *slog.Logger` (default `logging.Component("pihole")`). `session.Pool.Get` sets `c.Log = component("pihole").With("target", name)`. The request debug log moves after the response and includes `status` and `duration`. `url=` attributes are replaced by the target attribute.

### Run result: `internal/run`
```go
type Status string // ok | partial | failed | skipped
type Counts struct{ Added, Updated, Deleted, Failed int }
type Result struct {
    Target, Role string; Trigger string
    Started time.Time; Duration time.Duration
    Status Status; Stage string; Err error
    Resources map[string]Counts; Errors []error
}
func (r Result) Totals() Counts
func (r Result) LogValue() slog.Value // non-zero counts only
func (r *Result) Record(resource, action string, ok bool, err error)
```
`run.Status` maps to the metric label (`ok→success`, `failed→error`, `partial→partial`); `skipped` is not recorded in metrics.

### Worker envelope + Runner
```go
type Runner interface {
    Run(client *pihole.Client, res *run.Result) (*pihole.Client, error)
}
func (w *TargetWorker) Run(r Runner, trigger string) run.Result
```
The envelope does gate → `Sessions.Get` (stage `session`) → `CheckReadiness` (stage `readiness`) → `r.Run` (stage set by runner, default `apply`) → classify → health transition → `Recorder.RecordRun(result)` → log `run finished`. A runner returns a replacement client when it refreshed the session (FTL restart).
- `ConfigRunner` holds today's settings/content/gravity logic from `worker.go`.
- `Reconcile()` stays as a thin wrapper `w.Run(ConfigRunner{...}, "interval")` for existing callers/tests.
- *Alternative*: keep sync separate and copy health/backoff into it. Rejected: two implementations of the same state machine and logging.

### Sync on workers
`sync.Cycle` replaces `SyncAll`:
1. `primaryWorker.Run(&SnapshotRunner{resources})` loads the primary state once.
2. If it is not `ok`, each replica gets `Result{Status: skipped, Stage: "primary"}` without touching health.
3. Otherwise each replica worker runs `&ReplicaRunner{snapshot, resources, marker, purgeDNS}`, whose body is today's `syncReplica` minus session/gate/metrics handling.
Replica workers respect `ShouldSkip` (backoff). The worker set is rebuilt with `worker.SyncWorkers` on reload, preserving state. The `[forseti-sync]` marker, group translation and DNS purge are unchanged.

### Health threshold
`recordFailure` transitions to `down` when `consecutiveFails >= 5` (constant `downThreshold`). Transitions are logged in one place (`setHealth(new, err)`), comparing old and new state. `recovered` includes `failures` and `downtime` (time since first failure).

### Reachability tracker: `internal/targetstate`
```go
type Tracker struct{ ... }
func (t *Tracker) Observe(target, source string, err error)
```
Keeps the last known reachability per target; logs `target connected` / `target unreachable` only on change and sets `forseti_target_reachable`. Worker envelope, collector, new-domains and gravity call `Observe` instead of `MarkTargetReachable/Unreachable` and their own error logs (those drop to debug). Created in `main` and injected into each subsystem (nil-safe no-op for tests).

### Collector negative caching
A failed fetch stores `cachedStats{failedAt: now}`; the next fetch for that target happens after the TTL. Previously exported metric values stay as they are. `blocking`/`upstreams`/`dhcp` sub-errors go to debug.

### Applied change counts in config mode
`reconcile.Apply` records errors as `*reconcile.ItemError{Resource, Action, Key string; Count int; Err error}` (implements `error`, unwraps). The worker computes applied counts as planned minus failed per resource/action, using `Count` (batch size for batch deletes). The `Errors []error` field type stays, so callers are unaffected.

### Event emission points
| Event | Emitted by |
|---|---|
| `forseti starting`, `config loaded`, `config warning`, `config reloaded`, `config reload failed`, `shutting down`, `stopped` | `cmd/forseti` |
| `http server listening` | `cmd/forseti` (component `http`) |
| `target connected` / `target unreachable` | `targetstate` (component `session`) |
| `run finished`, `target degraded/down/recovered` | `worker` (component `worker` in config mode, `sync` in sync mode) |
| `cycle complete` | `worker.RunCycle` / `sync.Cycle` |
| `gravity scheduled/started/finished` | `gravity` |

`next_run` is passed into the cycle functions by `main` as `time.Now().Add(interval)` at tick time.

### Config reload diff
`config.Diff(old, new *Config) ReloadDiff{Changed []string; Added, Removed []string}` compares top-level sections with `reflect.DeepEqual` keyed by their YAML tag, and target names. Passwords are compared but never printed.

### Redaction: `internal/redact`
- `redact.URL(s)`: drops userinfo, replaces each query value with `REDACTED`.
- `redact.Text(s)`: applies `URL` to every `http(s)://` substring, collapses newlines, truncates to 256 runes.
- `APIError.Error()` renders `pihole api <status> <key>: <message>` when the body parsed as JSON, otherwise the body truncated to 200 bytes. The `Message` field keeps the raw body so `IsGravityCorrupted`/`IsTransient` keep working.
- `config.Target` implements `LogValue()` (name + host only).
- `SettingMapping.Sensitive` + table test over all mappings.
- Adlist keys in reconcile/sync errors and logs pass through `redact.URL`.

### Liveness
`metrics.Server` gets `Heartbeat()` (atomic timestamp) and serves `/healthz` against a threshold set via `SetLivenessInterval(d)`. `main` calls `Heartbeat()` after every cycle. `forseti healthcheck` parses only `metrics.port` from the config with a minimal YAML struct (errors fall back to 9099) and probes `127.0.0.1:<port>/healthz`.

### Timezone
`import _ "time/tzdata"` in `cmd/forseti` (~450 KB).

### Implementation notes
- `worker.Recorder` is `RecordRun(run.Result)`, `RecordDrift`, `UpdateSettingsDrift`, `RecordTargetHealth`; reachability goes through `worker.Reachability` (`targetstate.Tracker`).
- `sync.Syncer` keeps the primary and replica workers across reloads via `Syncer.Update(cfg)`; `Syncer.Cycle(nextRun)` returns the cycle summary and per-target results.
- `cmd/forseti` is split into `main.go` (one-shot commands), `watch.go` (daemon loop) and `healthcheck.go`.
- Collectors, gravity and new-domains default to their own tracker and are switched to the shared one via `SetReachability`, keeping constructor signatures stable.

## Risks / Trade-offs

- [Log message changes break existing LogQL alerts] → BREAKING in commit and CHANGELOG; README documents the event catalog as the stable contract going forward.
- [Sync refactor regresses sync behaviour] → Existing `syncer_test.go` scenarios are kept against `ReplicaRunner`; the change is structural, not algorithmic.
- [Health threshold change makes `down` reachable for network outages] → Intended; `forseti_target_health` value 2 now also covers prolonged outages. Documented.
- [Container health no longer reflects Pi-hole reachability] → BREAKING; README points to `forseti_target_reachable` for that.
- [logtest uses the global default logger] → Tests that use it are not run in parallel.

## Migration Plan

Ship as one minor release with `feat!`. Users relying on old log messages switch to the documented events. No config changes required.
