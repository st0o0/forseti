## ADDED Requirements

### Requirement: System info collector
The system SHALL collect Pi-hole host system information from `/api/info/system` when the `system` toggle is enabled. Metrics SHALL include CPU usage, memory usage, temperature, and load averages.

#### Scenario: System metrics collection
- **WHEN** the `system` toggle is enabled and the target is reachable
- **THEN** the system SHALL fetch `/api/info/system` and update the following gauges per target:
  - `forseti_pihole_cpu_percent`
  - `forseti_pihole_memory_percent`
  - `forseti_pihole_temperature_celsius`
  - `forseti_pihole_load_1m`
  - `forseti_pihole_load_5m`
  - `forseti_pihole_load_15m`

#### Scenario: System metrics disabled
- **WHEN** the `system` toggle is disabled
- **THEN** the system SHALL NOT call the system endpoint and SHALL NOT register the metrics

#### Scenario: Temperature unavailable
- **WHEN** the Pi-hole host does not report temperature data (e.g., VM or container)
- **THEN** the system SHALL omit the `forseti_pihole_temperature_celsius` metric for that target rather than reporting zero

#### Scenario: Endpoint not available
- **WHEN** the `/api/info/system` endpoint returns 404
- **THEN** the system SHALL log a warning and skip the collector for that cycle
