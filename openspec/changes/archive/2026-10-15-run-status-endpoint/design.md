## Context

See proposal.md. After `logging-overhaul`, `worker.TargetWorker.finish` produces a `run.Result` and updates health for every run in both modes; `worker.Recorder` forwards results to `metrics.Server`; `metrics.Server` owns the HTTP mux (`/metrics`, `/healthz`). Nothing keeps results after they are recorded.

## Goals / Non-Goals

**Goals:**
- One place that remembers recent runs and current target state, readable concurrently by HTTP handlers.
- Freshness metrics usable for alerting without PromQL gymnastics.

**Non-Goals:**
- Persistence, authentication, a web UI, or write operations.
- Reading worker fields from HTTP goroutines (workers are not synchronized).

## Decisions

### Target state travels with the result
`run.TargetState{Health string; ConsecutiveFailures int; LastSuccess, BackoffUntil time.Time; LastError error}` is built by the worker after the health update. `worker.Recorder.RecordTargetHealth(target, state string)` becomes `RecordTargetState(target string, s run.TargetState)` so metrics can set health and consecutive failures from one call.
- *Alternative*: let `/status` query workers. Rejected: worker state is mutated by the loop goroutine without locks.

### Observer hook
`worker.Dependencies.Observer` (`interface{ RunFinished(run.Result, run.TargetState) }`, optional) is called at the end of `finish` and from `Skip`. `status.Store` implements it. Cycle summaries are pushed by the loops (`Store.CycleFinished(cycle, nextRun)`), config reloads by `main` (`Store.ConfigReloaded(ok, err)`).

### Store layout
`internal/status.Store` holds, under one `sync.RWMutex`: per-target `ring[Entry]` (cap 1440), a `ring[CycleEntry]` (cap 1440), per-target latest `run.TargetState` and role, and process info. `Entry` is a compact projection: time, status, stage, duration, non-zero per-resource counts, up to 3 sanitized errors. Rings are fixed-size slices with a head index; readers receive copies.
- 1440 covers 24h at a 1-minute interval and longer for slower intervals. Worst case ~1440 × ~300 B ≈ 0.4 MB per target.
- *Alternative*: run-length encode idle runs to cover 24h at any interval. Rejected for now: more complex history semantics; revisit if short intervals become common.

### Target set
`Store.SetTargets([]Target{Name, Role})` is called at startup and on reload; unknown targets are dropped with their history, new ones appear with empty state. `/status` lists exactly the configured targets.

### HTTP
`metrics.Server.Handle(pattern, http.Handler)` mounts extra routes on the existing mux before `Start`. `status.Handler(store)` serves `/status` and `/status/history` (GET only, JSON, `Cache-Control: no-store`). Errors are `redact.Text`-sanitized when entries are created, so handlers never see raw errors.

### Metrics
In `metrics.RecordRun`: `last_run_timestamp` for every recorded run; `last_success_timestamp` for success/partial; `item_errors_total{type}` from `Resources[*].Failed` plus `ErrorCount - sum(Failed)` as `type="other"`. `RecordTargetState` sets `forseti_target_health` and `forseti_target_consecutive_failures`. Names follow the existing `forseti_gravity_last_run_timestamp` convention (no `_seconds` suffix). All new series are registered under the existing `reconcile` collector toggle.

### Dashboard
`grafana/forseti.json` (Grafana 10+ schema, datasource variable `DS_PROMETHEUS`, `target` variable) with: a state timeline of run outcomes per target over 24h (`increase(forseti_reconcile_runs_total[$__interval])` by status), stat panels for time since last success and consecutive failures, health/reachability status, and changes per resource type.

## Risks / Trade-offs

- [History lost on restart] → Documented; Prometheus metrics remain the durable record.
- [Unauthenticated status on the metrics port] → Contains only names, counts and redacted errors; README already advises not to expose the port.
- [Recorder interface change] → Internal only; fakes in tests updated.
