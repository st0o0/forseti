## MODIFIED Requirements

### Requirement: Summary-derived query metrics
The metrics server SHALL register and populate the following gauges from Stats data:

- `forseti_queries_forwarded{target}` — forwarded query count
- `forseti_queries_cached{target}` — cached query count
- `forseti_unique_domains{target}` — distinct domains queried
- `forseti_request_frequency{target}` — DNS requests per second

#### Scenario: Summary metrics populated
- **WHEN** UpdateStats is called with Stats containing forwarded=8000, cached=3000, unique_domains=5678, frequency=1.5
- **THEN** the corresponding gauges SHALL reflect those values with the target label

### Requirement: Client count metrics
The metrics server SHALL register and populate:

- `forseti_clients_active{target}` — currently active clients (24h window)
- `forseti_clients_seen{target}` — cumulative unique clients ever seen

#### Scenario: Client metrics populated
- **WHEN** UpdateStats is called with Stats containing clients active=15, total=42
- **THEN** `forseti_clients_active{target}` SHALL be 15 and `forseti_clients_seen{target}` SHALL be 42

### Requirement: Query type breakdown metrics
The metrics server SHALL register `forseti_dns_queries_by_type{target, query_type}` as a GaugeVec and populate one gauge per DNS record type (A, AAAA, PTR, etc.) from the Stats query types map.

#### Scenario: Query types populated
- **WHEN** Stats contains query types {"A": 5000, "AAAA": 3000, "PTR": 200}
- **THEN** three gauge values SHALL be set with the respective query_type labels

#### Scenario: Stale query types cleared
- **WHEN** a query type that was present in the previous scrape is absent in the current scrape
- **THEN** the metric for that query type SHALL be removed (not left at stale value)

### Requirement: Query status breakdown metrics
The metrics server SHALL register `forseti_dns_queries_by_status{target, status}` as a GaugeVec and populate one gauge per resolution status (GRAVITY, FORWARDED, CACHE, REGEX, etc.).

#### Scenario: Query status populated
- **WHEN** Stats contains status {"GRAVITY": 800, "FORWARDED": 8000, "CACHE": 3000}
- **THEN** three gauge values SHALL be set with the respective status labels

### Requirement: Reply type breakdown metrics
The metrics server SHALL register `forseti_dns_replies_by_type{target, reply_type}` as a GaugeVec and populate one gauge per reply type (CNAME, IP, NXDOMAIN, etc.).

#### Scenario: Reply types populated
- **WHEN** Stats contains replies {"CNAME": 3000, "IP": 7000, "NXDOMAIN": 200}
- **THEN** three gauge values SHALL be set with the respective reply_type labels

### Requirement: Upstream resolver metrics
The metrics server SHALL provide an `UpdateUpstreams(target string, upstreams []UpstreamStats)` method that registers and populates:

- `forseti_upstream_queries{target, upstream, name, port}` — queries routed to each upstream
- `forseti_upstream_response_seconds{target, upstream, name, port}` — average response time
- `forseti_upstream_response_variance{target, upstream, name, port}` — response time variance

#### Scenario: Upstream metrics populated
- **WHEN** UpdateUpstreams is called with two upstreams (1.1.1.1:53 avg 25ms, 8.8.8.8:53 avg 30ms)
- **THEN** six gauge values SHALL be set (3 metrics x 2 upstreams) with correct labels

#### Scenario: Stale upstreams cleared
- **WHEN** an upstream that was present in the previous scrape is absent in the current scrape
- **THEN** metrics for that upstream SHALL be removed

### Requirement: Blocking status metric
The metrics server SHALL populate `forseti_blocking_status{target}` with 1 when blocking is enabled and 0 when disabled, sourced from GetBlockingStatus().

#### Scenario: Blocking enabled
- **WHEN** GetBlockingStatus returns true
- **THEN** `forseti_blocking_status{target}` SHALL be 1

#### Scenario: Blocking disabled
- **WHEN** GetBlockingStatus returns false
- **THEN** `forseti_blocking_status{target}` SHALL be 0

## RENAMED Requirements

### Requirement: Pi-hole stats metrics prefix
- **FROM:** `pihole_dns_queries`, `pihole_dns_queries_blocked`, `pihole_blocked_percentage`, `pihole_gravity_last_update`, `pihole_status`, `pihole_domains_blocked`, `pihole_queries_forwarded`, `pihole_queries_cached`, `pihole_unique_domains`, `pihole_request_frequency`, `pihole_clients_active`, `pihole_clients_seen`, `pihole_dns_queries_by_type`, `pihole_dns_queries_by_status`, `pihole_dns_replies_by_type`, `pihole_upstream_queries`, `pihole_upstream_response_seconds`, `pihole_upstream_response_variance`
- **TO:** `forseti_dns_queries`, `forseti_dns_queries_blocked`, `forseti_blocked_percentage`, `forseti_gravity_last_update_timestamp`, `forseti_blocking_status`, `forseti_domains_blocked`, `forseti_queries_forwarded`, `forseti_queries_cached`, `forseti_unique_domains`, `forseti_request_frequency`, `forseti_clients_active`, `forseti_clients_seen`, `forseti_dns_queries_by_type`, `forseti_dns_queries_by_status`, `forseti_dns_replies_by_type`, `forseti_upstream_queries`, `forseti_upstream_response_seconds`, `forseti_upstream_response_variance`
