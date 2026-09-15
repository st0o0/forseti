## Why

The `forseti_reconcile_drift` Prometheus gauge is never reset to 0 once drift resolves. `buildDriftMap` only emits entries with `total > 0`, so `RecordReconcile` never calls `Set(0)` for resource types that return to sync — the gauge retains the last non-zero value indefinitely. This causes false drift alerts (e.g. `reconcile_drift{type="adlists"} = 1` persisting across cycles even though `forseti plan` shows `=18` unchanged).

Separately, the `forseti_target_health` metric uses `0 = healthy, 1 = degraded, 2 = down`, which is counterintuitive (most Prometheus gauges use `1 = good`). The Help text documents this, but the README metrics table does not explain the encoding, leading to confusion when dashboarding.

## What Changes

- **Fix stale drift gauge**: Reset all resource type drift gauges to 0 before applying the new drift values from each reconcile cycle. This ensures resolved drift is reflected immediately.
- **Document health metric encoding**: Add a clear note to the README metrics table explaining that `target_health` uses `0 = healthy, 1 = degraded, 2 = down` (enum-style, not boolean).

## Capabilities

### New Capabilities

_(none)_

### Modified Capabilities

- `metrics`: Drift gauge lifecycle — drift values must be reset to 0 when a resource type returns to sync.

## Impact

- `internal/metrics/metrics.go` or `internal/worker/worker.go` — drift reset logic
- `README.md` — metrics documentation table
- No API changes, no breaking changes, no new dependencies
