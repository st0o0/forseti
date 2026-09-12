## MODIFIED Requirements

### Requirement: Drift updated at reconcile time
The settings drift metric SHALL be updated during each reconcile cycle. The system SHALL call `DiffSettings()` before apply and update the gauge. After a successful `ApplySettings()` call, the system SHALL re-evaluate drift and update the gauge to reflect the post-apply state.

#### Scenario: Drift detected during reconcile
- **WHEN** a reconcile cycle runs for target `pihole-router` and `DiffSettings()` reports `dns.upstream` differs
- **THEN** `forseti_settings_drift{target="pihole-router", setting="dns.upstream"}` SHALL be set to 1

#### Scenario: Drift cleared after successful apply
- **WHEN** a reconcile apply corrects `dns.upstream` on target `pihole-router` and `ApplySettings()` returns no error
- **THEN** the system SHALL re-diff settings and `forseti_settings_drift{target="pihole-router", setting="dns.upstream"}` SHALL be set to 0 in the same reconcile cycle

#### Scenario: Drift preserved after failed apply
- **WHEN** a reconcile apply for target `pihole-router` returns an error
- **THEN** `forseti_settings_drift` gauges SHALL retain their pre-apply values (drift=1 for changed settings)
