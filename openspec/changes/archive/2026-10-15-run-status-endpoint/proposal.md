## Why

Issue #23 asked for a way to see "the last 24 hours of syncs" at a glance. After `logging-overhaul`, every run produces a structured result, but it is only visible in logs and aggregate counters: there is no per-target "when did this last succeed?" signal, no history to look at, and no ready-made dashboard. A small read-only status API and a few targeted metrics close that gap without turning Forseti into a stateful web application.

## What Changes

- **Run history**: Forseti keeps the most recent runs per target (and recent cycle summaries) in memory. History is lost on restart and is not an audit log.
- **Status API** on the metrics port:
  - `GET /status`: version, mode, uptime, config reload state, last cycle and next run, and per target: role, health, consecutive failures, last success, backoff, last error and last run with per-resource counts.
  - `GET /status/history?target=<name>&since=<duration>&limit=<n>`: recent runs of one target, or recent cycle summaries without `target`.
  - Responses contain no passwords, session IDs or raw response bodies; error messages are redacted.
- **New metrics**:
  - `forseti_reconcile_last_success_timestamp{target}` and `forseti_reconcile_last_run_timestamp{target}`
  - `forseti_target_consecutive_failures{target}`
  - `forseti_reconcile_item_errors_total{target, type}`
- **Grafana dashboard**: an example dashboard in the repo (`grafana/forseti.json`) with run outcomes over the last 24h per target, time since last success, health/reachability and changes.

## Capabilities

### New Capabilities
- `run-status`: in-memory run history and the read-only `/status` API.

### Modified Capabilities
- `metrics`: last-success/last-run timestamps, consecutive failures and item error counters.

## Impact

- New package `internal/status`; `internal/worker` gains a run observer hook; `internal/metrics` gains the new collectors and a way to mount extra handlers; `cmd/forseti/watch.go` wires the store.
- New file `grafana/forseti.json`; README sections for `/status` and the dashboard.
- No config changes; memory use grows with the number of targets (bounded ring per target).
