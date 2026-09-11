## MODIFIED Requirements

### Requirement: Pi-hole stats metrics
The system SHALL scrape Pi-hole stats endpoints and expose per-target metrics under the `forseti_*` namespace:

- `forseti_dns_queries{target}` — total queries
- `forseti_dns_queries_blocked{target}` — total blocked queries
- `forseti_blocked_percentage{target}` — block percentage
- `forseti_gravity_last_update_timestamp{target}` — unix timestamp of last gravity update
- `forseti_blocking_status{target}` — blocking status (1 = enabled, 0 = disabled)
- `forseti_domains_blocked{target}` — unique domains on blocklists

Extended metrics are defined in the `pihole-prometheus-metrics` spec.

#### Scenario: Stats from two targets
- **WHEN** two targets are configured and both are reachable
- **THEN** all `forseti_*` stats metrics SHALL carry the respective `target` label

#### Scenario: Target unreachable during scrape
- **WHEN** a target is unreachable during stats collection
- **THEN** `forseti_target_reachable{target}` SHALL be 0 and stale metrics for that target SHALL remain at their last known values

## ADDED Requirements

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
