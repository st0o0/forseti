# Collector Toggles

## Purpose

Per-area boolean switches in `metrics.collectors` that control which metric groups are registered and which Pi-hole API calls are made during scrape.

## Requirements

### Requirement: Collector toggles configuration
The system SHALL accept a `metrics.collectors` section in the config file with boolean toggles for each metric area. Each toggle controls whether the corresponding metrics are registered and whether the associated Pi-hole API calls are made during collection.

The supported toggles SHALL be:
- `stats` — summary query metrics (dns_queries, blocked, forwarded, cached, clients, etc.)
- `upstreams` — upstream resolver metrics
- `query_types` — query type/status/reply breakdown metrics
- `blocking` — blocking status metric
- `reconcile` — reconciliation telemetry metrics
- `gravity` — gravity update metrics
- `sessions` — session pool metrics
- `settings_drift` — settings drift detection metric
- `dhcp` — DHCP lease metrics
- `new_domains` — new blocked domain detection (structured log events plus an aggregated count gauge); unlike the other toggles, this collector runs on its own configurable interval rather than `metrics.scrape_interval`

#### Scenario: All collectors enabled by default
- **WHEN** the config file omits the `metrics.collectors` section entirely
- **THEN** all collector toggles SHALL default to `true`

#### Scenario: Disable a specific collector
- **WHEN** the config contains `metrics.collectors.upstreams: false`
- **THEN** upstream metrics SHALL not be registered with Prometheus and upstream API calls SHALL be skipped during collection

#### Scenario: Partial toggles specified
- **WHEN** the config contains only `metrics.collectors.dhcp: false` and no other toggles
- **THEN** all other collectors SHALL remain enabled (default true) and only DHCP collection SHALL be disabled

#### Scenario: Disable new-blocked-domains collector
- **WHEN** the config contains `metrics.collectors.new_domains: false`
- **THEN** the system SHALL NOT register `forseti_new_blocked_domains_24h`, SHALL NOT emit `new_blocked_domain` log events, and SHALL NOT call the query-history API on any target

### Requirement: Conditional metric registration
When a collector toggle is disabled, the system SHALL NOT register the corresponding Prometheus metrics with the registry. Disabled collectors SHALL produce no metrics in the `/metrics` output — not zero-value metrics, but absent metrics.

#### Scenario: Disabled collector produces no output
- **WHEN** `metrics.collectors.query_types` is `false`
- **THEN** `forseti_dns_queries_by_type`, `forseti_dns_queries_by_status`, and `forseti_dns_replies_by_type` SHALL not appear in the `/metrics` response

#### Scenario: Enabled collector registers normally
- **WHEN** `metrics.collectors.stats` is `true`
- **THEN** all stats-group metrics SHALL be registered and populated on scrape

### Requirement: Conditional API call skipping
When a collector toggle is disabled, the system SHALL skip the associated Pi-hole API calls during stats collection. This reduces API load on the Pi-hole instance.

#### Scenario: Disabled upstreams skips API call
- **WHEN** `metrics.collectors.upstreams` is `false`
- **THEN** the collector SHALL NOT call `GetUpstreams()` on any target

#### Scenario: Disabled stats skips summary fetch
- **WHEN** `metrics.collectors.stats` is `false`
- **THEN** the collector SHALL NOT call `GetStats()` on any target

### Requirement: New collector toggle keys
The collector toggles system SHALL recognize four new toggle keys: `top_domains`, `top_clients`, `history`, and `system`. Each SHALL default to disabled (opt-in). Each SHALL gate both the API calls and the Prometheus metric registration.

#### Scenario: Enable top_domains toggle
- **WHEN** the config sets `collector_toggles.top_domains: true`
- **THEN** the top domains collector SHALL be active and metrics SHALL be registered

#### Scenario: Enable top_clients toggle
- **WHEN** the config sets `collector_toggles.top_clients: true`
- **THEN** the top clients collector SHALL be active and metrics SHALL be registered

#### Scenario: Enable history toggle
- **WHEN** the config sets `collector_toggles.history: true`
- **THEN** the history collector SHALL start its tick loop and metrics SHALL be registered

#### Scenario: Enable system toggle
- **WHEN** the config sets `collector_toggles.system: true`
- **THEN** the system info collector SHALL be active and metrics SHALL be registered

#### Scenario: All new toggles disabled by default
- **WHEN** no collector toggle config is provided
- **THEN** all four new toggles SHALL be disabled, no new API calls SHALL be made, and no new metrics SHALL be registered
