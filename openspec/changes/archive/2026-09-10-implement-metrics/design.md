## Context

The metrics spec defines a comprehensive set of Prometheus metrics. The package needs to register them, expose methods for the reconciler to record results, and run an HTTP server.

## Goals / Non-Goals

**Goals:**
- All forseti_* and pihole_* metrics registered with proper types (counter, gauge, histogram)
- `Server` type with `Start()` / `Shutdown()` lifecycle
- `RecordReconcile()` and `UpdateStats()` methods for the reconciler to call
- Per-target labels on all metrics

**Non-Goals:**
- Custom collectors (use standard prometheus registry)
- Push gateway support
- Metric retention or aggregation

## Decisions

### Server struct holds metric references
All metric vec references live on the `Server` struct. Methods like `RecordReconcile` accept target name and results, then update the correct metric labels.

### Histogram for duration, counters for events, gauges for state
- `forseti_reconcile_duration_seconds` — histogram with default buckets
- `forseti_reconcile_runs_total`, `forseti_reconcile_changes_total` — counters
- `forseti_reconcile_drift_total`, `forseti_target_reachable`, config gauges — gauges
- `pihole_*` — all gauges (scraped from Pi-hole, not accumulated)

### Config metrics set once at startup
`forseti_config_*_total` gauges are set when the config is loaded. They don't change during runtime.

## Risks / Trade-offs
- **Stale pihole_* metrics on unreachable target** → By design, gauges retain last known value. `forseti_target_reachable` indicates staleness.
