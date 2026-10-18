## ADDED Requirements

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
