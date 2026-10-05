## Why

Operators have no visibility into which domains Pi-hole started blocking recently. Today Forseti only exposes aggregated per-target counters (total queries, total blocked, etc.) — there is no way to spot a newly-appearing blocked domain without manually trawling Pi-hole's query log. Adding this requires care: a naive `forseti_domain_blocked_total{domain="..."}` metric would introduce unbounded per-domain Prometheus label cardinality, and a naive implementation might require Forseti to keep its own "have I seen this domain" ledger, which would misfire on every container restart.

## What Changes

- New collector that queries Pi-hole's existing `/api/queries` history (via `GetQueries` on the Pi-hole client) each cycle, computing — per target — which blocked domains were first seen within a rolling 24h window.
- New `GetQueries` method on the Pi-hole API client (session-authenticated, same pattern as `GetStats`/`GetFTLInfo`). No new persistence layer: "first seen" is derived from Pi-hole's own query history on every run, not tracked by Forseti.
- New structured log event `new_blocked_domain` (fields: `target`, `domain`, `first_seen`, `count_24h`) emitted via the existing slog JSON handler, for consumption by the existing Alloy → VictoriaLogs → Grafana pipeline. No new logging pipeline.
- New Prometheus gauge `forseti_new_blocked_domains_24h{target}` — an aggregated count only, no `domain` label, consistent with every other collector metric.
- New `new_domains` toggle in `metrics.collectors`, following the existing toggle pattern, but with its **own** configurable interval (not tied to `scrape_interval`) since `/api/queries` over a 24h window is a heavier call than the existing stats endpoints and does not need to run on every scrape.
- No threshold or filtering logic in Forseti — every newly-seen domain is logged with its `count_24h`; filtering/alerting on that count is left to Grafana/LogQL.

## Capabilities

### New Capabilities
- `new-blocked-domains`: Detects and reports, per target, domains blocked for the first time within a rolling 24h window — via a structured log event (domain detail) and an aggregated Prometheus gauge (count only).

### Modified Capabilities
- `collector-toggles`: Add a `new_domains` toggle to `metrics.collectors`, distinct from the others in that it carries its own interval rather than following `scrape_interval`.
- `pihole-api`: Add a `GetQueries` client method for retrieving query history from Pi-hole v6's `/api/queries` endpoint.

## Impact

- `internal/pihole/client.go`: new `GetQueries` method and response types.
- `internal/collector/collector.go`: new collector function/cycle for new-domain detection, run on its own interval.
- `internal/metrics/metrics.go`: new `newBlockedDomains24h` GaugeVec keyed by `target`.
- `internal/config/config.go`: new `new_domains` toggle (and its interval) in `CollectorToggles`/`Metrics`.
- No new external dependencies; no new persistence.
