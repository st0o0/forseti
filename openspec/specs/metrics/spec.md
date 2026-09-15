# Metrics

## Purpose

Prometheus metrics endpoint: forseti_* reconciliation telemetry and stats, per-target labels, configurable scrape interval, per-area collector toggles.
## Requirements
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
- `forseti_reconcile_drift{target, type}` — gauge of items that differ from desired state at each cycle
- `forseti_target_reachable{target}` — gauge (1 = reachable, 0 = unreachable)
- `forseti_session_active` — gauge of active sessions in pool

`forseti_session_active` SHALL increment when a new session is created and decrement when a session is invalidated. It SHALL reset to 0 when the pool is closed. The gauge MUST accurately reflect the number of live sessions at all times.

After each reconcile cycle, `forseti_reconcile_drift` SHALL be set for all resource types (groups, adlists, deny, allow, local_dns, cname, clients). Resource types with no drift SHALL be set to 0. The gauge MUST NOT retain stale values from previous cycles.

#### Scenario: Successful reconcile updates metrics
- **WHEN** a reconcile cycle completes successfully against target `pihole-router` with 2 adlists added
- **THEN** `forseti_reconcile_runs_total{target="pihole-router", status="success"}` SHALL increment by 1 and `forseti_reconcile_changes_total{target="pihole-router", type="adlist", action="add"}` SHALL increment by 2

#### Scenario: Failed reconcile
- **WHEN** a reconcile cycle fails against target `pihole-pi`
- **THEN** `forseti_reconcile_runs_total{target="pihole-pi", status="error"}` SHALL increment by 1

#### Scenario: Drift resolves after apply
- **WHEN** a previous cycle reported `forseti_reconcile_drift{target="pizero", type="adlists"} = 1` and the next cycle finds no adlist drift
- **THEN** `forseti_reconcile_drift{target="pizero", type="adlists"}` SHALL be 0

#### Scenario: Session invalidation decrements gauge
- **WHEN** a session for target "alpha" is invalidated (auth failure, settings change restart)
- **THEN** `forseti_session_active` SHALL decrement by 1

#### Scenario: Session re-creation after invalidation
- **WHEN** a session is invalidated and then re-created on next reconcile
- **THEN** `forseti_session_active` SHALL first decrement then increment, ending at the same value

### Requirement: Forseti config metrics
The system SHALL expose static config gauges:

- `forseti_config_adlists` — number of configured adlists
- `forseti_config_deny_domains` — number of configured deny domains
- `forseti_config_allow_domains` — number of configured allow domains

#### Scenario: Config loaded
- **WHEN** config defines 10 adlists and 5 deny domains
- **THEN** `forseti_config_adlists` SHALL be 10 and `forseti_config_deny_domains` SHALL be 5

### Requirement: Pi-hole stats metrics
The system SHALL scrape Pi-hole stats endpoints and expose per-target metrics under the `forseti_*` namespace:

- `forseti_dns_queries{target}` — total queries
- `forseti_dns_queries_blocked{target}` — total blocked queries
- `forseti_blocked_percentage{target}` — block percentage
- `forseti_gravity_last_update_timestamp{target}` — unix timestamp of last gravity update
- `forseti_blocking_status{target}` — blocking status (1 = enabled, 0 = disabled)
- `forseti_domains_blocked{target}` — unique domains on blocklists

Extended metrics are defined in the `pihole-prometheus-metrics` and `pihole-stats-collection` specs.

#### Scenario: Stats from two targets
- **WHEN** two targets are configured and both are reachable
- **THEN** all `forseti_*` stats metrics SHALL carry the respective `target` label

#### Scenario: Target unreachable during scrape
- **WHEN** a target is unreachable during stats collection
- **THEN** `forseti_target_reachable{target}` SHALL be 0 and stale metrics for that target SHALL remain at their last known values

### Requirement: Configurable scrape interval
The `metrics.scrape_interval` config value SHALL control how frequently Pi-hole stats are fetched. This is independent of the reconcile interval.

#### Scenario: Separate from reconcile
- **WHEN** reconcile interval is `5m` and scrape interval is `30s`
- **THEN** stats SHALL be refreshed every 30 seconds regardless of reconcile timing

### Requirement: Build info metric
The system SHALL register `forseti_build_info` as a Gauge with value 1 and labels `{version, mode}`. This metric SHALL be set once at startup and remain constant for the lifetime of the process.

#### Scenario: Build info exposed
- **WHEN** forseti starts in config mode with version "1.2.3"
- **THEN** `forseti_build_info{version="1.2.3", mode="config"}` SHALL be 1

#### Scenario: Sync mode
- **WHEN** forseti starts in sync mode
- **THEN** `forseti_build_info{mode="sync"}` SHALL be 1

### Requirement: Collector toggles in metrics config
The metrics config section SHALL accept a `collectors` subsection with boolean toggles. Each toggle SHALL default to `true` when omitted. The toggles control which metric groups are registered and collected.

#### Scenario: Default config unchanged
- **WHEN** an existing config without `metrics.collectors` is loaded
- **THEN** all metrics SHALL be registered and collected (backwards compatible)

#### Scenario: Collectors section with one disabled
- **WHEN** config contains `metrics.collectors.dhcp: false`
- **THEN** DHCP metrics SHALL not be registered and all other metrics SHALL work normally

### Requirement: Settings drift metric integration
The metrics server SHALL support a `forseti_settings_drift{target, setting}` gauge as part of the metrics system. This metric is populated by the reconciler, not the collector.

#### Scenario: Drift metric available
- **WHEN** settings drift collector is enabled and reconciliation runs
- **THEN** `forseti_settings_drift` SHALL appear in `/metrics` output with per-setting labels

### Requirement: DHCP metrics integration
The metrics server SHALL support `forseti_dhcp_leases_active{target}` as a gauge populated by the collector when the DHCP toggle is enabled.

#### Scenario: DHCP metric in output
- **WHEN** DHCP collector is enabled and targets have active DHCP
- **THEN** `forseti_dhcp_leases_active` SHALL appear in `/metrics` output

