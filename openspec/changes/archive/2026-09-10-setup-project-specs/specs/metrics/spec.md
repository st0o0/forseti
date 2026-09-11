## ADDED Requirements

### Requirement: Prometheus metrics endpoint
The system SHALL expose a Prometheus-compatible `/metrics` endpoint. Port and path are configurable via the `metrics` section in the config file (defaults: port `9099`, path `/metrics`).

#### Scenario: Default endpoint
- **WHEN** no metrics config is specified
- **THEN** the system SHALL serve metrics on `:9099/metrics`

#### Scenario: Custom port and path
- **WHEN** metrics config specifies port `8080` and path `/prom`
- **THEN** the system SHALL serve metrics on `:8080/prom`

### Requirement: Forseti reconciliation metrics
The system SHALL expose reconciliation telemetry:

- `forseti_reconcile_runs_total{target, status}` — counter of reconcile cycles (status: success/error)
- `forseti_reconcile_duration_seconds{target}` — histogram of reconcile cycle duration
- `forseti_reconcile_changes_total{target, type, action}` — counter of changes applied (type: adlist/deny/allow/dns/group/client, action: add/delete)
- `forseti_reconcile_drift_total{target, type}` — gauge of items that differ from desired state at each cycle
- `forseti_target_reachable{target}` — gauge (1 = reachable, 0 = unreachable)

#### Scenario: Successful reconcile updates metrics
- **WHEN** a reconcile cycle completes successfully against target `pihole-router` with 2 adlists added
- **THEN** `forseti_reconcile_runs_total{target="pihole-router", status="success"}` SHALL increment by 1 and `forseti_reconcile_changes_total{target="pihole-router", type="adlist", action="add"}` SHALL increment by 2

#### Scenario: Failed reconcile
- **WHEN** a reconcile cycle fails against target `pihole-pi`
- **THEN** `forseti_reconcile_runs_total{target="pihole-pi", status="error"}` SHALL increment by 1

### Requirement: Forseti config metrics
The system SHALL expose static config gauges:

- `forseti_config_adlists_total` — number of configured adlists
- `forseti_config_deny_domains_total` — number of configured deny domains
- `forseti_config_allow_domains_total` — number of configured allow domains

#### Scenario: Config loaded
- **WHEN** config defines 10 adlists and 5 deny domains
- **THEN** `forseti_config_adlists_total` SHALL be 10 and `forseti_config_deny_domains_total` SHALL be 5

### Requirement: Pi-hole stats metrics
The system SHALL scrape Pi-hole stats endpoints and expose per-target metrics:

- `pihole_queries_total{target}` — total queries
- `pihole_blocked_total{target}` — total blocked queries
- `pihole_blocked_percentage{target}` — block percentage
- `pihole_gravity_size{target}` — domains in gravity database
- `pihole_gravity_last_update{target}` — unix timestamp of last gravity update
- `pihole_cache_size{target}` — DNS cache size
- `pihole_cache_inserted{target}` — cache insertions
- `pihole_cache_evicted{target}` — cache evictions
- `pihole_status{target}` — Pi-hole status (1 = enabled, 0 = disabled)
- `pihole_ftl_memory_bytes{target}` — FTL process memory usage

#### Scenario: Stats from two targets
- **WHEN** two targets are configured and both are reachable
- **THEN** all `pihole_*` metrics SHALL carry the respective `target` label

#### Scenario: Target unreachable during scrape
- **WHEN** a target is unreachable during stats collection
- **THEN** `forseti_target_reachable{target}` SHALL be 0 and stale `pihole_*` metrics for that target SHALL remain at their last known values

### Requirement: Configurable scrape interval
The `metrics.scrape_interval` config value SHALL control how frequently Pi-hole stats are fetched. This is independent of the reconcile interval.

#### Scenario: Separate from reconcile
- **WHEN** reconcile interval is `5m` and scrape interval is `30s`
- **THEN** stats SHALL be refreshed every 30 seconds regardless of reconcile timing
