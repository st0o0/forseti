## ADDED Requirements

### Requirement: Collector stale cache counter
The metrics server SHALL register `forseti_collector_cache_stale_total` as a Counter with label `{target}`. Each time a target's cached stats are served because the target's concurrency gate was unavailable, the counter SHALL increment.

#### Scenario: Stale served due to busy target
- **WHEN** the collector cannot acquire the gate for target "pizero" and serves cached data
- **THEN** `forseti_collector_cache_stale_total{target="pizero"}` SHALL increment by 1

#### Scenario: Normal cache hit does not increment stale counter
- **WHEN** the collector serves cached data because the TTL has not expired (normal cache hit)
- **THEN** `forseti_collector_cache_stale_total` SHALL NOT be incremented (use `forseti_collector_cache_hits_total` instead)
