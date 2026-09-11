## Why

Forseti exposes Pi-hole stats via Prometheus, but only 5 of 11 registered pihole_* gauges are actually populated. Meanwhile, dedicated Pi-hole exporters (eko/pihole-exporter, bazmonk/pihole6_exporter) expose 20+ operational metrics that popular Grafana dashboards depend on. Users running Forseti as their controller shouldn't need a separate exporter sidecar just for observability parity.

## What Changes

- **Fix dead metrics**: Populate the 6 registered-but-empty gauges (gravity_size, cache_size, cache_inserted, cache_evicted, status, ftl_memory_bytes) or remove them if the Pi-hole v6 API doesn't expose the data
- **Expand Stats struct**: Parse all fields from `/api/stats/summary` — forwarded, cached, unique_domains, frequency, clients (active/total), query types map, query status map, reply types map
- **New summary-based metrics**: `pihole_queries_forwarded`, `pihole_queries_cached`, `pihole_unique_domains`, `pihole_clients_active`, `pihole_clients_total`, `pihole_query_types` (by DNS type), `pihole_query_status` (by status), `pihole_reply_types` (by reply type), `pihole_request_frequency`
- **New upstream metrics**: Call `/api/stats/upstreams` to expose `pihole_upstream_queries_total`, `pihole_upstream_response_seconds`, `pihole_upstream_response_variance` per upstream resolver
- **Deliberately excluded**: Top queries/blocked/clients — high-cardinality metrics that are a Prometheus anti-pattern

## Capabilities

### New Capabilities
- `pihole-stats-collection`: Collecting and parsing extended Pi-hole v6 stats (summary + upstreams) into Go structs
- `pihole-prometheus-metrics`: Registering and populating the full set of pihole_* Prometheus metrics from collected stats

### Modified Capabilities

_(none — no existing spec-level behavior changes)_

## Impact

- `internal/pihole/client.go`: Stats struct expansion, new UpstreamStats type, new GetUpstreams() method, possible GetBlockingStatus() method
- `internal/metrics/metrics.go`: New metric registrations, expanded UpdateStats(), new UpdateUpstreams() method
- `internal/pihole/client_test.go`, `internal/metrics/metrics_test.go`: New test coverage
- Caller code (watch loop): Add GetUpstreams call to stats collection cycle
- Metric consumers (Grafana dashboards): New metrics available for dashboarding
