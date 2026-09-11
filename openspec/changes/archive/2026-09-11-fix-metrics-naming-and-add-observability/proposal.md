## Why

Eight Gauge metrics use the `_total` suffix which Prometheus reserves for Counters. PromQL linters flag these, and `rate()` / `increase()` applied by mistake on a Gauge silently produces wrong results. Additionally, the Collector and Session subsystems have zero instrumentation — when a scrape is slow or sessions churn, there is no way to diagnose the cause from metrics alone.

## What Changes

- **Rename 11 metrics** to comply with Prometheus naming conventions: remove `_total` from Gauges, add `dns_` namespace to Pi-hole query metrics, improve breakdown metric names (`query_types` -> `dns_queries_by_type`)
- **Add 3 Collector metrics**: scrape duration histogram, fetch counter (success/error per target), cache hit counter per target
- **Add 2 Session metrics**: reauth counter per target, active session gauge
- **Add build info metric**: `forseti_build_info` (standard exporter info pattern with version and mode labels)
- **BREAKING**: All renamed metrics change their Prometheus metric name. Existing Grafana dashboards and alert rules referencing old names will break and need updating.

## Capabilities

### New Capabilities
- `collector-observability`: Prometheus metrics for the stats collector subsystem (duration, fetches, cache hits)
- `session-observability`: Prometheus metrics for the session pool (reauth events, active sessions)

### Modified Capabilities
- `metrics`: Rename Gauge metrics that violate `_total` convention, add `forseti_build_info`
- `pihole-prometheus-metrics`: Rename `pihole_query_types`, `pihole_query_status`, `pihole_reply_types`, `pihole_upstream_queries_total`, `pihole_clients_total`

## Impact

- `internal/metrics/metrics.go` — all renames, new metric registrations
- `internal/metrics/metrics_test.go` — update expected metric names
- `internal/collector/collector.go` — instrument Collect() with duration, fetch, and cache-hit metrics
- `internal/collector/collector_test.go` — test new instrumentation
- `internal/pihole/client.go` — add `OnReauth` callback hook
- `internal/session/pool.go` — set callback, expose active count
- `cmd/forseti/` — set build info metric at startup
- Grafana dashboards / alert rules referencing old metric names (external, documented in breaking change)
