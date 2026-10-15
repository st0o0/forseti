## 1. Target state and observer

- [x] 1.1 Add `run.TargetState`; worker builds it after each run and calls `Recorder.RecordTargetState` (replacing `RecordTargetHealth`)
- [x] 1.2 Add optional `Dependencies.Observer` called from `finish` and `Skip`; tests

## 2. Metrics

- [x] 2.1 Register `forseti_reconcile_last_run_timestamp`, `forseti_reconcile_last_success_timestamp`, `forseti_target_consecutive_failures`, `forseti_reconcile_item_errors_total`
- [x] 2.2 Set them from `RecordRun` / `RecordTargetState`; tests
- [x] 2.3 `Server.Handle(pattern, handler)` for extra routes

## 3. Status store and API

- [x] 3.1 `internal/status`: ring buffer, `Entry`/`CycleEntry` projections, `Store` with `SetTargets`, `RunFinished`, `CycleFinished`, `ConfigReloaded`; tests incl. bounds and target removal
- [x] 3.2 `/status` and `/status/history` handlers (params, 400/404, no-store); tests incl. redaction
- [x] 3.3 Wire the store in `watch.go` for both modes (targets on startup/reload, cycles, reloads, observer)

## 4. Dashboard and docs

- [x] 4.1 `grafana/forseti.json` example dashboard
- [x] 4.2 README: `/status` API, new metrics with alert example, dashboard

## 5. Verification

- [x] 5.1 Run `go test -race ./...`, `go vet ./...`, `golangci-lint run`; smoke-test `/status` against a running daemon
