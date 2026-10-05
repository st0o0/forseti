## MODIFIED Requirements

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
