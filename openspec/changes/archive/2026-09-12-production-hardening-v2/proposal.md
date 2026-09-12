## Why

After the worker architecture refactor, several gaps remain: plan/apply commands lack readiness checks, the phantom-delete root cause is unresolved (only symptoms treated), gravity still blocks synchronously, write operations have no retry, the collector doesn't sync on hot-reload, readiness/error logic relies on string matching, and health states aren't exposed as metrics. This change addresses all remaining production gaps in one sweep.

## What Changes

- **Readiness for plan/apply**: Add CheckReadiness before reconcile in one-shot commands
- **Readiness gate tests**: Unit tests for CheckReadiness and CheckAlive on pihole.Client
- **Phantom-delete root cause**: Investigate why batchDelete returns 404 for adlists that ListAdlists returns; fix URL matching or fall back to individual deletes
- **Async gravity**: Gravity trigger becomes non-blocking via channel; worker doesn't wait for gravity completion
- **Plan/apply readiness**: One-shot commands get readiness check before operating
- **Write retry**: Create/update adlist operations retry on transient errors (readonly database)
- **Collector hot-reload**: Collector rebuilds on hot-reload when targets change
- **Integration test harness**: Docker-based tests with simulated FTL restarts and slow responses
- **Health state metrics**: Worker health exposed as Prometheus gauge per target
- **Structured error types**: Parse Pi-hole error JSON keys instead of string matching

## Capabilities

### New Capabilities
- `async-gravity`: Non-blocking gravity trigger with channel-based communication
- `health-metrics`: Worker health state exposed as Prometheus metrics
- `write-resilience`: Retry on transient write operations

### Modified Capabilities
- `pihole-api`: Structured error parsing, individual delete fallback
- `target-worker`: Readiness for plan/apply, health metrics exposure
- `config`: Collector hot-reload on target changes
- `error-resilience`: Structured error types replace string matching

## Impact

- `internal/pihole/client.go` — structured error parsing, individual delete, readiness tests
- `internal/worker/worker.go` — async gravity, health metrics
- `internal/gravity/scheduler.go` — channel-based trigger
- `internal/collector/collector.go` — hot-reload support
- `internal/metrics/metrics.go` — health state gauges
- `cmd/forseti/main.go` — readiness in plan/apply, collector sync on reload
- `internal/reconcile/reconciler.go` — write retry on transient errors
- New integration test infrastructure
