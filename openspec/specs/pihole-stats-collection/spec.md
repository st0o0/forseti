# Pi-hole Stats Collection

Collecting and parsing Pi-hole v6 statistics (summary, upstreams, blocking status) into Go structs for metrics exposure.

### Requirement: Extended summary stats parsing
The Stats struct SHALL parse all available fields from the `/api/stats/summary` response: `queries.forwarded`, `queries.cached`, `queries.unique_domains`, `queries.frequency`, `queries.types` (map), `queries.status` (map), `queries.replies` (map), `clients.active`, and `clients.total`.

#### Scenario: Full summary parsing
- **WHEN** the client calls GetStats() against a Pi-hole v6 instance
- **THEN** the returned Stats struct SHALL contain forwarded count, cached count, unique domains, request frequency, client counts (active and total), and maps of query types, query statuses, and reply types

#### Scenario: Missing optional fields
- **WHEN** the summary response omits a field (e.g., an older Pi-hole v6 build)
- **THEN** the Stats struct SHALL use zero values for missing numeric fields and empty maps for missing map fields

### Requirement: Upstream stats retrieval
The client SHALL provide a `GetUpstreams() ([]UpstreamStats, error)` method that calls `GET /api/stats/upstreams` and returns per-upstream resolver statistics including IP, name, port, query count, average response time, and response time variance.

#### Scenario: Multiple upstreams
- **WHEN** Pi-hole is configured with two upstream resolvers (1.1.1.1 and 8.8.8.8)
- **THEN** GetUpstreams() SHALL return two UpstreamStats entries with their respective query counts and response statistics

#### Scenario: Virtual upstreams filtered
- **WHEN** the upstreams response includes virtual entries (ip="blocklist" or ip="cache" with port=-1)
- **THEN** GetUpstreams() SHALL include them in the results (they represent valid query destinations)

### Requirement: Blocking status retrieval
The client SHALL provide a `GetBlockingStatus() (bool, error)` method that calls `GET /api/dns/blocking` and returns whether Pi-hole blocking is currently enabled.

#### Scenario: Blocking enabled
- **WHEN** Pi-hole blocking is active
- **THEN** GetBlockingStatus() SHALL return true

#### Scenario: Blocking disabled
- **WHEN** Pi-hole blocking has been temporarily disabled
- **THEN** GetBlockingStatus() SHALL return false

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
