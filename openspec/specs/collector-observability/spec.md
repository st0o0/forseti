# Collector Observability

Prometheus metrics for the stats collector: scrape duration, fetch counts, and cache hit tracking.

### Requirement: Collector scrape duration metric
The metrics server SHALL register `forseti_collector_duration_seconds` as a Histogram that records the wall-clock duration of each `Collect()` cycle (covering all targets, including cache checks and parallel fetches).

#### Scenario: Duration recorded on scrape
- **WHEN** a Prometheus scrape triggers Collect() and the cycle completes in 1.2 seconds
- **THEN** `forseti_collector_duration_seconds` SHALL observe 1.2

### Requirement: Collector fetch counter
The metrics server SHALL register `forseti_collector_fetches_total` as a Counter with labels `{target, status}` where status is `success` or `error`. Each actual API fetch (not cache hit) SHALL increment the counter for the respective target and outcome.

#### Scenario: Successful fetch
- **WHEN** target "pihole-router" stats are stale and the API call succeeds
- **THEN** `forseti_collector_fetches_total{target="pihole-router", status="success"}` SHALL increment by 1

#### Scenario: Failed fetch
- **WHEN** target "pihole-pi" stats are stale and the API call fails
- **THEN** `forseti_collector_fetches_total{target="pihole-pi", status="error"}` SHALL increment by 1

#### Scenario: Cache hit does not increment
- **WHEN** target "pihole-router" stats are still within TTL
- **THEN** `forseti_collector_fetches_total` for that target SHALL NOT be incremented

### Requirement: Collector cache hit counter
The metrics server SHALL register `forseti_collector_cache_hits_total` as a Counter with label `{target}`. Each time a target's cached stats are still within TTL and no fetch is performed, the counter SHALL increment.

#### Scenario: Cache hit counted
- **WHEN** Collect() is called and target "pihole-router" has fresh cached stats
- **THEN** `forseti_collector_cache_hits_total{target="pihole-router"}` SHALL increment by 1
