## 1. Logging foundation

- [x] 1.1 Add `internal/logging` (`Setup`, `SetLevel`, `Component`, `ParseLevel`) backed by a shared `slog.LevelVar`, with tests
- [x] 1.2 Add `internal/logging/logtest` recording handler with `Find(msg)` helpers
- [x] 1.3 Replace `setupLogger` in `main`: bootstrap logger before config load, reconfigure after load, `SetLevel` on reload, warn on `log_format` change; `plan`/`apply` log to stderr
- [x] 1.4 Collect config resolution warnings in `Config.Warnings` instead of logging in `merge.go`; log them as `config warning` in `main`
- [x] 1.5 Give every component a `logging.Component(...)` logger field (session, gravity, collector, new-domains, worker, sync, reconcile, config, metrics/http) and replace `slog.` calls
- [x] 1.6 Pi-hole client: `Log` field set by the pool with `target`; request debug log after the response with status and duration; drop `url=` attributes
- [x] 1.7 Embed `time/tzdata`

## 2. Redaction

- [x] 2.1 Add `internal/redact` (`URL`, `Text`) with tests
- [x] 2.2 Render `APIError.Error()` from parsed key/message or truncated body; keep `Message` raw; tests
- [x] 2.3 `config.Target.LogValue()` without password; test
- [x] 2.4 `SettingMapping.Sensitive` and redacted output in `setting applied` and `printSettingsDiff`; guard test over all mappings
- [x] 2.5 Redact adlist URLs in reconcile and sync errors/logs

## 3. Run model and worker envelope

- [x] 3.1 Add `internal/run` (`Status`, `Counts`, `Result`, `Record`, `Totals`, `LogValue`) with tests
- [x] 3.2 Add `reconcile.ItemError` and use it for every item/batch error in `reconcile.Apply`; tests
- [x] 3.3 Introduce `Runner` and `TargetWorker.Run` envelope; move config logic into `ConfigRunner`; keep `Reconcile()` as wrapper; applied change counts from plan minus `ItemError`s
- [x] 3.4 Recorder takes `run.Result` (`RecordRun`); map statuses to metric labels; skipped not recorded
- [x] 3.5 Health: `down` after 5 consecutive failures; single `setHealth` logging `target degraded/down/recovered` with failures and downtime
- [x] 3.6 `run finished` event with level by status; `RunCycle` returning a cycle summary and logging `cycle complete` with `next_run`
- [x] 3.7 Update worker tests for envelope, transitions and events (logtest)

## 4. Sync on workers

- [x] 4.1 `SnapshotRunner` (primary) and `ReplicaRunner` (replica) implementing `Runner` from today's `syncReplica`/`state.load`
- [x] 4.2 `sync.Cycle` replacing `SyncAll`: primary run, skipped replicas on primary failure, replica backoff via `ShouldSkip`, `cycle complete`
- [x] 4.3 Wire sync mode in `main` with worker sets rebuilt on reload
- [x] 4.4 Port `syncer_test.go`/`report_test.go` to the runners and add primary-failure/skipped and replica-backoff tests

## 5. Reachability tracker and noise control

- [x] 5.1 Add `internal/targetstate.Tracker` (`Observe`) logging transitions and setting `forseti_target_reachable`; tests
- [x] 5.2 Use the tracker in worker envelope, collector, new-domains and gravity; demote their per-attempt error logs to debug
- [x] 5.3 Collector negative caching of failed fetches; sub-fetch errors to debug; tests
- [x] 5.4 New-domains: log `new_blocked_domain` once per (target, domain) per window with in-memory pruning; tests
- [x] 5.5 Demote reconcile retry logs to debug

## 6. Lifecycle events in main and gravity

- [x] 6.1 `forseti starting`, `config loaded` (mode-specific summary), `http server listening`, `shutting down`, `stopped`
- [x] 6.2 `config.Diff` and `config reloaded` with changed sections and added/removed targets; tests
- [x] 6.3 Gravity: `gravity scheduled` at info, `gravity started`/`gravity finished`, `already running` at debug

## 7. Liveness

- [x] 7.1 `/healthz` with heartbeat and stall threshold on the metrics server; tests
- [x] 7.2 Heartbeat after each cycle in both modes; liveness interval set from config (and on reload)
- [x] 7.3 Rewrite `forseti healthcheck` to probe `/healthz` (`--addr`, `--config` port lookup, default 9099); tests

## 8. Docs and verification

- [x] 8.1 README: logging section with event catalog and level policy, healthcheck semantics, `down` threshold, CHANGELOG-relevant breaking notes
- [x] 8.2 Run `go test -race ./...`, `go vet ./...`, `golangci-lint run`, `docker build`; fix findings
