## Why

Forseti needs a Prometheus `/metrics` endpoint exposing both reconciliation telemetry (`forseti_*`) and Pi-hole stats (`pihole_*`). This is the observability layer that enables alerting and dashboards for the reconciler's operation and Pi-hole health.

## What Changes

- Implement the metrics server in `internal/metrics/metrics.go`
- Register all `forseti_*` metrics: reconcile runs, duration, changes, drift, target reachability, config gauges
- Register all `pihole_*` metrics: queries, blocked, cache, gravity, FTL memory, status
- Per-target label support for all metrics
- HTTP server lifecycle for the `/metrics` endpoint
- Methods for recording reconcile results and updating Pi-hole stats

## Capabilities

### New Capabilities
None. Implements the existing `metrics` spec.

### Modified Capabilities
None.

## Impact
- `internal/metrics/metrics.go` — full implementation
- `internal/metrics/metrics_test.go` — unit tests
- `go.mod` — `prometheus/client_golang` dependency restored
