## ADDED Requirements

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
