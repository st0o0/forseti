## Context

See proposal.md for motivation. Relevant current state:

- `internal/sync/syncer.go`: `SyncReport.Errors` is declared but never appended; `syncX` helpers only log failures and return `(added, deleted int)`. `RecordReconcile` is called with `Success: len(report.Errors) == 0` (always true) and without `Changes`. Replica load failure returns before any metric is recorded and without invalidating the session. Sync calls `pool.Get` without `Acquire`/`Release`.
- `internal/metrics/metrics.go`: `RecordReconcile` sets `forseti_target_reachable` from `Success`; the collector sets it back to 1 on every successful scrape (`UpdateStats`), so the gauge flaps after a partial run.
- `internal/worker/worker.go`: `RecordTargetHealth` is only called on the session-error path and after success; readiness/settings failures neither record a run result nor publish health.
- `cmd/forseti/main.go`: config-mode reload updates pool API config, scrape timeout and both collectors; sync-mode reload only swaps `cfg` and rebuilds the syncer. Neither mode updates the gravity scheduler, which is built once. `tryReloadConfig` only advances `lastMtime` on success, so a broken file is re-loaded and re-reported on every tick.
- `internal/gravity/scheduler.go`: only creates its 30s ticker when entries exist at startup.

This change is deliberately narrow: it fixes signals in place. The structural rework (shared run model, injected loggers, event catalog) is the follow-up change `logging-overhaul`; decisions here must not block it.

## Goals / Non-Goals

**Goals:**
- Correct run status, change counts, reachability and health metrics in both modes.
- Complete hot-reload in both modes, including gravity.
- Make the sync local-DNS exception explicit and configurable.

**Non-Goals:**
- No new log messages/event catalog, no logger injection (follow-up change).
- No health/backoff for sync mode, no "down after N failures" (follow-up change).
- Config-mode change counters keep counting planned changes; switching them to applied changes needs structured item errors from `reconcile.Apply` and belongs to the follow-up.

## Decisions

### Run status as a string, not a bool
`metrics.ReconcileResult.Success bool` becomes `Status string` with constants `metrics.RunSuccess`, `RunPartial`, `RunError`. `worker.Recorder.RecordReconcileResult` takes the status instead of `success bool`.
- *Alternative*: keep `Success` and add `Partial bool` - two booleans with an invalid combination; rejected.

### Reachability set explicitly, never from run outcome
`RecordReconcile` stops touching `forseti_target_reachable`. A new `MarkTargetReachable(target)` is called by the worker after a successful readiness check and by sync after a replica session plus state read succeed; `MarkTargetUnreachable` stays on session/readiness failure. The collector keeps setting 1 on a successful scrape. All writers now agree on one meaning: "target answered".

### Worker: single failure helper
Introduce `w.fail(start, err)` used by every error exit (session, readiness, settings, content). It records run status `error`, calls `recordFailure`, and publishes `RecordTargetHealth`. Session/readiness failures additionally mark the target unreachable. The success path records `success` or `partial` (`len(report.Errors) > 0`); both call `recordSuccess`. Settings failures that `reconcileSettings` currently swallows (diff/apply error returns `client, nil`) stay non-fatal - unchanged behaviour.

### Sync: per-resource results instead of counters
Each `syncX` helper returns a small `resourceResult{added, deleted int; errs []error}`. `syncReplica` collects them into `SyncReport` (now with `Changes map[string]map[string]int` keyed by the config-mode type names) and appends errors. Status: `error` on session/state-read failure, `partial` if any errors, else `success`. A failed re-fetch of replica groups after group sync is an item error (subsequent group translation may be incomplete), not a run error.
- *Alternative*: pass a `*SyncReport` into each helper - fewer return values but hides mutation; rejected for testability.

### Sync gating
`syncReplica` wraps its whole body in `pool.Acquire(name)` / `defer pool.Release(name)`, as the worker does. The primary is gated only while its state is loaded in `SyncAll`, so replicas are not serialised behind the primary. Primary session failure marks the primary unreachable; replica runs are not recorded for that cycle (classification as `skipped` comes with the follow-up run model).

### `sync.local_dns_purge` as `*bool`, default true
`SyncConfig.LocalDNSPurge *bool`; nil means true, applied in `syncDNS` deletion loop. A pointer keeps "omitted" distinguishable so the default can be documented and changed later without ambiguity. Default `true` preserves current behaviour (no breaking change); config-mode `local_dns_purge` defaults to `false` - the asymmetry is intentional (sync mirrors a source of truth, config mode co-exists with manual entries) and documented in README.

### Reload: advance mtime before loading
`tryReloadConfig` records the observed mtime before calling `config.Load`, so a failing modification is attempted and reported exactly once. The failure metric therefore increments once per bad modification.

### Shared runtime update on reload
Extract `applyTargets(targets)` in `main.go` that updates pool API config, scrape timeout, both collectors and `gravSched.UpdateTargets`. Both mode loops call it after a successful reload.

### Gravity `UpdateTargets`
Under `s.mu`: rebuild `targetMap`; rebuild `entries`, keeping the existing `next` for targets whose schedule string is unchanged and recomputing it otherwise; drop removed targets. Invalid schedules are already rejected by config validation, but a parse error still keeps the old entry and logs a warning. `Start` always runs the 30s ticker (cheap) so schedules added by reload are honoured; with no entries the tick is a no-op. In-flight runs for removed targets complete normally.

## Risks / Trade-offs

- [New `partial` status breaks alerts matching `status!="success"` semantics differently] → Called out as BREAKING in proposal and CHANGELOG; `partial` is a strict subset of what was (wrongly) `success` before, so `status="error"` alerts are unaffected.
- [Sync runs now record `error` on session failure, increasing error counts] → Intended; previously these failures were invisible.
- [Holding the replica gate for the whole sync run can delay collector scrapes on `max_concurrent: 1` targets] → Same trade-off the config-mode worker already makes; collector serves stale cache via `TryAcquire`.
- [Advancing mtime before load means a transiently unreadable file (partial write) is not retried until the next modification] → Editors write the file again on save; mtime changes on every write. Acceptable.
- [Gravity entries rebuilt on reload could double-fire if `next` is recomputed to "now"] → `next` is computed with `schedule.Next(now)`, always strictly in the future.
