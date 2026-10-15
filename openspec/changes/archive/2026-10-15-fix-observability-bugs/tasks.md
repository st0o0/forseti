## 1. Metrics: run status and reachability

- [x] 1.1 Replace `ReconcileResult.Success` with `Status` (`RunSuccess`/`RunPartial`/`RunError`) and update `RecordReconcile`/`RecordReconcileResult` to emit the status label
- [x] 1.2 Stop setting `forseti_target_reachable` in `RecordReconcile`; add `MarkTargetReachable(target)`
- [x] 1.3 Update metrics tests for `partial` status and the reachable semantics

## 2. Worker: record every cycle outcome

- [x] 2.1 Change `worker.Recorder.RecordReconcileResult` to take a status string and add `MarkTargetReachable` to the interface; update fakes in tests
- [x] 2.2 Add a single failure helper that records `error`, updates failure state and publishes health; use it on session, readiness, settings-restart and content error paths
- [x] 2.3 Mark target reachable after a successful readiness check; record `partial` when `report.Errors` is non-empty while keeping health healthy
- [x] 2.4 Add worker tests: readiness failure publishes degraded health and `error`; item errors yield `partial` without backoff

## 3. Sync: result accounting, gate, DNS purge

- [x] 3.1 Add `SyncConfig.LocalDNSPurge *bool` (nil = true) with config test for default and explicit false
- [x] 3.2 Make each `syncX` helper return per-resource added/deleted counts and item errors; collect them into `SyncReport` errors and a `Changes` map using config-mode type names
- [x] 3.3 Classify the run (`success`/`partial`/`error`), record it on every path including replica state-read failure (with session invalidation), and mark the replica reachable/unreachable
- [x] 3.4 Acquire/release the per-target gate around replica sync and around primary state load; mark the primary unreachable on primary session failure
- [x] 3.5 Honour `local_dns_purge: false` in `syncDNS` (add-only)
- [x] 3.6 Update/extend `syncer_test.go`: partial on create/delete error, error + invalidation on load failure, change map contents, gate acquired, DNS purge off

## 4. Hot-reload completeness

- [x] 4.1 Add `Scheduler.UpdateTargets` (rebuild target map and entries, keep `next` for unchanged schedules) and always run the scheduler ticker; add scheduler tests
- [x] 4.2 Extract a shared `applyTargets` helper in `main.go` (pool API config, scrape timeout, both collectors, gravity) and call it from both config-mode and sync-mode reload paths
- [x] 4.3 Advance the observed mtime before `config.Load` in `tryReloadConfig`; add test that a broken file is reported once and a later fix reloads

## 5. Docs and verification

- [x] 5.1 README: document `sync.local_dns_purge`, the `partial` run status, and the reachable/health metric semantics
- [x] 5.2 Run `go test -race ./...`, `go vet ./...` and `golangci-lint run`; fix findings
