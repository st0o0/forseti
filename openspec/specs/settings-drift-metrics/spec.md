# Settings Drift Metrics

Settings drift detection as a Prometheus gauge, comparing YAML desired state against actual Pi-hole config each reconcile cycle.

### Requirement: Settings drift gauge metric
The system SHALL expose `forseti_settings_drift{target, setting}` as a Prometheus gauge. For each declared setting in the effective config, the gauge SHALL be 1 when the actual Pi-hole value differs from the desired value, and 0 when they match.

#### Scenario: One setting drifted
- **WHEN** target `pihole-1` has `dns.cache.size` set to 5000 in YAML but the Pi-hole reports 10000
- **THEN** `forseti_settings_drift{target="pihole-1", setting="dns.cache.size"}` SHALL be 1

#### Scenario: All settings in sync
- **WHEN** all declared settings for target `pihole-1` match the Pi-hole's actual config
- **THEN** all `forseti_settings_drift{target="pihole-1", setting="..."}` gauges SHALL be 0

#### Scenario: Undeclared settings not tracked
- **WHEN** the Forseti config does not declare `privacy.level`
- **THEN** no `forseti_settings_drift` gauge with `setting="privacy.level"` SHALL exist

### Requirement: Drift updated at reconcile time
The settings drift metric SHALL be updated during each reconcile cycle, not at scrape time. The system SHALL call `DiffSettings()` and update the gauge based on the diff result.

#### Scenario: Drift detected during reconcile
- **WHEN** a reconcile cycle runs for target `pihole-router` and `DiffSettings()` reports `dns.upstream` differs
- **THEN** `forseti_settings_drift{target="pihole-router", setting="dns.upstream"}` SHALL be set to 1

#### Scenario: Drift cleared after apply
- **WHEN** a reconcile apply corrects `dns.upstream` on target `pihole-router`
- **THEN** `forseti_settings_drift{target="pihole-router", setting="dns.upstream"}` SHALL be set to 0 on the next reconcile cycle

### Requirement: Drift metric controlled by collector toggle
The `forseti_settings_drift` metric SHALL only be registered and updated when the `settings_drift` collector toggle is enabled.

#### Scenario: Toggle disabled
- **WHEN** `metrics.collectors.settings_drift` is `false`
- **THEN** no `forseti_settings_drift` metrics SHALL be registered or populated
