## ADDED Requirements

### Requirement: Top domains collector
The system SHALL collect top queried and top blocked domains from `/api/stats/top_domains` when the `top_domains` toggle is enabled. Results SHALL be exposed as Prometheus GaugeVec metrics with `target` and `domain` labels.

#### Scenario: Top domains collection enabled
- **WHEN** the `top_domains` toggle is enabled and the target is reachable
- **THEN** the system SHALL fetch `/api/stats/top_domains?count={n}` and update `forseti_top_queried_domains` and `forseti_top_blocked_domains` gauges

#### Scenario: Top domains collection disabled
- **WHEN** the `top_domains` toggle is disabled
- **THEN** the system SHALL NOT call the top_domains endpoint and SHALL NOT register the metrics

#### Scenario: Configurable count limit
- **WHEN** the `top_domains` toggle config specifies a `count` value
- **THEN** the system SHALL pass that value to the API (default: 10, max: 100)

#### Scenario: Endpoint not available
- **WHEN** the `/api/stats/top_domains` endpoint returns 404
- **THEN** the system SHALL log a warning and skip the collector for that cycle without failing the entire collection

### Requirement: Top clients collector
The system SHALL collect top querying clients from `/api/stats/top_clients` when the `top_clients` toggle is enabled. Results SHALL be exposed as a Prometheus GaugeVec metric with `target` and `client` labels.

#### Scenario: Top clients collection enabled
- **WHEN** the `top_clients` toggle is enabled and the target is reachable
- **THEN** the system SHALL fetch `/api/stats/top_clients?count={n}` and update the `forseti_top_clients` gauge

#### Scenario: Top clients collection disabled
- **WHEN** the `top_clients` toggle is disabled
- **THEN** the system SHALL NOT call the top_clients endpoint and SHALL NOT register the metric

#### Scenario: Label cardinality bounded
- **WHEN** the count exceeds 100
- **THEN** the system SHALL cap the count at 100 and log a warning
