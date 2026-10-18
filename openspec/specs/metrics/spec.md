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

- `forseti_reconcile_runs_total{target, status}`  - counter of reconcile/sync runs per target (status: `success`, `partial`, `error`)
- `forseti_reconcile_duration_seconds{target}`  - histogram of run duration
- `forseti_reconcile_changes_total{target, type, action}`  - counter of changes applied (type: groups/adlists/deny/allow/local_dns/cname/clients, action: add/update/delete)
- `forseti_reconcile_drift{target, type}`  - gauge of items that differ from desired state at each cycle
- `forseti_target_reachable{target}`  - gauge (1 = reachable, 0 = unreachable)
- `forseti_target_health{target}`  - gauge of the worker health state (0 = healthy, 1 = degraded, 2 = down)
- `forseti_session_active`  - gauge of active sessions in pool

Run status values: `success` when the run completed without errors; `partial` when the target was reached and its state read, but at least one item-level write failed; `error` when the run could not complete (session, readiness, state read, settings or content apply failure).

`forseti_target_reachable` SHALL reflect only whether the target answered: it SHALL be set to 0 when a session cannot be obtained or the readiness check fails, and to 1 when a session and readiness check (or a stats scrape) succeed. The outcome of content or settings reconciliation SHALL NOT change it.

`forseti_target_health` SHALL be set after every reconcile cycle of a target, on every success and failure path.

`forseti_session_active` SHALL increment when a new session is created and decrement when a session is invalidated. It SHALL reset to 0 when the pool is closed. The gauge MUST accurately reflect the number of live sessions at all times.

After each reconcile cycle, `forseti_reconcile_drift` SHALL be set for all resource types (groups, adlists, deny, allow, local_dns, cname, clients). Resource types with no drift SHALL be set to 0. The gauge MUST NOT retain stale values from previous cycles.

#### Scenario: Successful reconcile updates metrics
- **WHEN** a reconcile cycle completes successfully against target `pihole-router` with 2 adlists added
- **THEN** `forseti_reconcile_runs_total{target="pihole-router", status="success"}` SHALL increment by 1 and `forseti_reconcile_changes_total{target="pihole-router", type="adlists", action="add"}` SHALL increment by 2

#### Scenario: Failed reconcile
- **WHEN** a reconcile cycle fails against target `pihole-pi`
- **THEN** `forseti_reconcile_runs_total{target="pihole-pi", status="error"}` SHALL increment by 1

#### Scenario: Partial reconcile
- **WHEN** a reconcile cycle against `pihole-pi` reads state successfully but one item write fails
- **THEN** `forseti_reconcile_runs_total{target="pihole-pi", status="partial"}` SHALL increment by 1 and `forseti_target_reachable{target="pihole-pi"}` SHALL remain 1

#### Scenario: Health gauge on readiness failure
- **WHEN** the readiness check for `pihole-pi` fails during a reconcile cycle
- **THEN** `forseti_target_health{target="pihole-pi"}` SHALL be 1 (degraded) and `forseti_target_reachable{target="pihole-pi"}` SHALL be 0

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

- `forseti_config_adlists`  - number of configured adlists
- `forseti_config_deny_domains`  - number of configured deny domains
- `forseti_config_allow_domains`  - number of configured allow domains

#### Scenario: Config loaded
- **WHEN** config defines 10 adlists and 5 deny domains
- **THEN** `forseti_config_adlists` SHALL be 10 and `forseti_config_deny_domains` SHALL be 5

### Requirement: Pi-hole stats metrics
The system SHALL scrape Pi-hole stats endpoints and expose per-target metrics under the `forseti_*` namespace:

- `forseti_dns_queries{target}`  - total queries
- `forseti_dns_queries_blocked{target}`  - total blocked queries
- `forseti_blocked_percentage{target}`  - block percentage
- `forseti_gravity_last_update_timestamp{target}`  - unix timestamp of last gravity update
- `forseti_blocking_status{target}`  - blocking status (1 = enabled, 0 = disabled)
- `forseti_domains_blocked{target}`  - unique domains on blocklists

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

### Requirement: Change counters count applied changes
`forseti_reconcile_changes_total` SHALL count only changes that were successfully applied, in both modes. Planned changes whose write failed SHALL NOT be counted. For a failed batch delete, every item in the batch SHALL be treated as failed.

#### Scenario: Failed create in config mode
- **WHEN** config mode plans 3 adlist additions on `pihole-router` and one of the creates fails
- **THEN** `forseti_reconcile_changes_total{target="pihole-router", type="adlists", action="add"}` SHALL increment by 2

#### Scenario: Failed batch delete
- **WHEN** a batch delete of 4 deny domains fails
- **THEN** `forseti_reconcile_changes_total{type="deny", action="delete"}` SHALL NOT increment for that batch

### Requirement: Run freshness and failure metrics
The system SHALL expose, per target, in both modes:

- `forseti_reconcile_last_run_timestamp{target}`  - unix time of the last completed (not skipped) run
- `forseti_reconcile_last_success_timestamp{target}`  - unix time of the last run with status `success` or `partial`
- `forseti_target_consecutive_failures{target}`  - number of consecutive failed runs (0 after a success)
- `forseti_reconcile_item_errors_total{target, type}`  - counter of failed item writes per resource type (`other` for errors not tied to a resource)

#### Scenario: Alert on stale sync
- **WHEN** replica `pihole-2` has not completed a successful run for 30 minutes
- **THEN** `time() - forseti_reconcile_last_success_timestamp{target="pihole-2"}` SHALL exceed 1800

#### Scenario: Failures reset on success
- **WHEN** a target fails twice and then succeeds
- **THEN** `forseti_target_consecutive_failures` SHALL read 2 after the second failure and 0 after the success

#### Scenario: Item errors counted
- **WHEN** two client writes fail during a partial run on `pihole-2`
- **THEN** `forseti_reconcile_item_errors_total{target="pihole-2", type="clients"}` SHALL increase by 2

### Requirement: Top domains metrics
The metrics server SHALL register `forseti_top_queried_domains` and `forseti_top_blocked_domains` as GaugeVec with labels `target` and `domain` when the `top_domains` toggle is enabled.

#### Scenario: Top domains gauge update
- **WHEN** the collector fetches top domains data
- **THEN** the metrics server SHALL reset and re-populate the GaugeVec with current top-N domain counts

#### Scenario: Stale labels cleared
- **WHEN** a domain drops out of the top-N between collection cycles
- **THEN** the metrics server SHALL remove the stale label combination (via Reset before re-population)

### Requirement: Top clients metric
The metrics server SHALL register `forseti_top_clients` as a GaugeVec with labels `target` and `client` when the `top_clients` toggle is enabled.

#### Scenario: Top clients gauge update
- **WHEN** the collector fetches top clients data
- **THEN** the metrics server SHALL reset and re-populate the GaugeVec with current top-N client counts

### Requirement: History metrics
The metrics server SHALL register `forseti_query_history_total` and `forseti_query_history_blocked` as GaugeVec with labels `target` and `bucket` when the `history` toggle is enabled.

#### Scenario: History gauge update
- **WHEN** the history collector fetches bucketed data
- **THEN** the metrics server SHALL update the gauges with the latest bucket values

### Requirement: System metrics
The metrics server SHALL register system gauges when the `system` toggle is enabled: `forseti_pihole_cpu_percent`, `forseti_pihole_memory_percent`, `forseti_pihole_temperature_celsius`, `forseti_pihole_load_1m`, `forseti_pihole_load_5m`, `forseti_pihole_load_15m`. All SHALL have a `target` label.

#### Scenario: System gauge update
- **WHEN** the collector fetches system info
- **THEN** the metrics server SHALL update all system gauges for that target

#### Scenario: Missing temperature
- **WHEN** the system info response does not include temperature data
- **THEN** the metrics server SHALL NOT emit the temperature metric for that target (no zero default)
