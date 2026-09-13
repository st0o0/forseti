## ADDED Requirements

### Requirement: Collector serves stale cache when target is busy
When the collector cannot acquire the target's concurrency gate (target is busy with reconciliation or gravity), it SHALL serve the most recent cached stats for that target instead of attempting an API call. This applies regardless of cache TTL — stale data is preferred over no data.

#### Scenario: Target busy during scrape
- **WHEN** the reconciler holds the gate for target "pizero" and a Prometheus scrape triggers collection
- **THEN** the collector SHALL skip the API call for "pizero", serve its last cached stats, and record a stale cache event

#### Scenario: Target busy with no prior cache
- **WHEN** the gate is held for a target that has never been successfully collected
- **THEN** the collector SHALL skip the target and record a stale cache event (no metrics emitted for that target this scrape)

#### Scenario: Target available after being busy
- **WHEN** the gate becomes available on the next scrape after a stale-served cycle
- **THEN** the collector SHALL acquire the gate, fetch fresh stats, and update the cache normally

### Requirement: Per-target timeout for collection
Each target's collection API calls SHALL use the target's configured `api.timeout` as the context deadline, instead of a hardcoded global timeout. The scrape handler's overall timeout SHALL be derived from the maximum configured target timeout plus a buffer.

#### Scenario: Slow target with long timeout
- **WHEN** target "pizero" has `api.timeout: 45s` and its stats call takes 20s
- **THEN** the collection SHALL succeed (within the 45s budget)

#### Scenario: Fast target with short timeout
- **WHEN** target "mikrotik" has `api.timeout: 10s` and its stats call takes 12s
- **THEN** the collection SHALL timeout and record an error
