# History Collector

## Purpose

Collector for time-bucketed Pi-hole query history, gated by the history toggle.

## Requirements

### Requirement: Query history collector
The system SHALL collect time-bucketed query history from `/api/history` when the `history` toggle is enabled. The collector SHALL run on a configurable tick interval (default: 5 minutes) rather than on every Prometheus scrape. Results SHALL be exposed as Prometheus metrics.

#### Scenario: History collection on tick
- **WHEN** the `history` toggle is enabled and the tick interval elapses
- **THEN** the system SHALL fetch `/api/history` and update `forseti_query_history_total` and `forseti_query_history_blocked` gauges with the latest bucket values per target

#### Scenario: History collection disabled
- **WHEN** the `history` toggle is disabled
- **THEN** the system SHALL NOT start the history tick loop and SHALL NOT register the metrics

#### Scenario: Scrape reads latest snapshot
- **WHEN** Prometheus scrapes `/metrics` between history ticks
- **THEN** the system SHALL serve the most recently fetched history values without making a new API call

#### Scenario: Endpoint not available
- **WHEN** the `/api/history` endpoint returns 404
- **THEN** the system SHALL log a warning and skip the collection for that cycle
