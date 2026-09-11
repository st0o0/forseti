## MODIFIED Requirements

### Requirement: Forseti config metrics
The system SHALL expose static config gauges:

- `forseti_config_adlists` — number of configured adlists
- `forseti_config_deny_domains` — number of configured deny domains
- `forseti_config_allow_domains` — number of configured allow domains

#### Scenario: Config loaded
- **WHEN** config defines 10 adlists and 5 deny domains
- **THEN** `forseti_config_adlists` SHALL be 10 and `forseti_config_deny_domains` SHALL be 5

### Requirement: Forseti reconciliation metrics
The system SHALL expose reconciliation telemetry:

- `forseti_reconcile_runs_total{target, status}` — counter of reconcile cycles (status: success/error)
- `forseti_reconcile_duration_seconds{target}` — histogram of reconcile cycle duration
- `forseti_reconcile_changes_total{target, type, action}` — counter of changes applied (type: adlist/deny/allow/dns/group/client, action: add/delete)
- `forseti_reconcile_drift{target, type}` — gauge of items that differ from desired state at each cycle
- `forseti_target_reachable{target}` — gauge (1 = reachable, 0 = unreachable)

#### Scenario: Successful reconcile updates metrics
- **WHEN** a reconcile cycle completes successfully against target `pihole-router` with 2 adlists added
- **THEN** `forseti_reconcile_runs_total{target="pihole-router", status="success"}` SHALL increment by 1 and `forseti_reconcile_changes_total{target="pihole-router", type="adlist", action="add"}` SHALL increment by 2

#### Scenario: Failed reconcile
- **WHEN** a reconcile cycle fails against target `pihole-pi`
- **THEN** `forseti_reconcile_runs_total{target="pihole-pi", status="error"}` SHALL increment by 1

### Requirement: Pi-hole stats metrics
The system SHALL scrape Pi-hole stats endpoints and expose per-target metrics:

- `pihole_dns_queries{target}` — total queries
- `pihole_dns_queries_blocked{target}` — total blocked queries
- `pihole_blocked_percentage{target}` — block percentage
- `pihole_gravity_last_update{target}` — unix timestamp of last gravity update
- `pihole_status{target}` — Pi-hole status (1 = enabled, 0 = disabled)
- `pihole_domains_blocked{target}` — unique domains on blocklists

Extended metrics are defined in the `pihole-prometheus-metrics` and `pihole-stats-collection` specs.

#### Scenario: Stats from two targets
- **WHEN** two targets are configured and both are reachable
- **THEN** all `pihole_*` metrics SHALL carry the respective `target` label

#### Scenario: Target unreachable during scrape
- **WHEN** a target is unreachable during stats collection
- **THEN** `forseti_target_reachable{target}` SHALL be 0 and stale `pihole_*` metrics for that target SHALL remain at their last known values

## ADDED Requirements

### Requirement: Build info metric
The system SHALL register `forseti_build_info` as a Gauge with value 1 and labels `{version, mode}`. This metric SHALL be set once at startup and remain constant for the lifetime of the process.

#### Scenario: Build info exposed
- **WHEN** forseti starts in config mode with version "1.2.3"
- **THEN** `forseti_build_info{version="1.2.3", mode="config"}` SHALL be 1

#### Scenario: Sync mode
- **WHEN** forseti starts in sync mode
- **THEN** `forseti_build_info{mode="sync"}` SHALL be 1
