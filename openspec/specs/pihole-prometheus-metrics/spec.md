# Pi-hole Prometheus Metrics

Registering and populating the full set of pihole_* Prometheus metrics from collected Pi-hole stats.

### Requirement: Summary-derived query metrics
The metrics server SHALL register and populate the following gauges from Stats data:

- `pihole_queries_forwarded{target}` — forwarded query count
- `pihole_queries_cached{target}` — cached query count
- `pihole_unique_domains{target}` — distinct domains queried
- `pihole_request_frequency{target}` — DNS requests per second

#### Scenario: Summary metrics populated
- **WHEN** UpdateStats is called with Stats containing forwarded=8000, cached=3000, unique_domains=5678, frequency=1.5
- **THEN** the corresponding gauges SHALL reflect those values with the target label

### Requirement: Client count metrics
The metrics server SHALL register and populate:

- `pihole_clients_active{target}` — currently active clients (24h window)
- `pihole_clients_seen{target}` — cumulative unique clients ever seen

#### Scenario: Client metrics populated
- **WHEN** UpdateStats is called with Stats containing clients active=15, total=42
- **THEN** `pihole_clients_active{target}` SHALL be 15 and `pihole_clients_seen{target}` SHALL be 42

### Requirement: Query type breakdown metrics
The metrics server SHALL register `pihole_dns_queries_by_type{target, query_type}` as a GaugeVec and populate one gauge per DNS record type (A, AAAA, PTR, etc.) from the Stats query types map.

#### Scenario: Query types populated
- **WHEN** Stats contains query types {"A": 5000, "AAAA": 3000, "PTR": 200}
- **THEN** three gauge values SHALL be set with the respective query_type labels

#### Scenario: Stale query types cleared
- **WHEN** a query type that was present in the previous scrape is absent in the current scrape
- **THEN** the metric for that query type SHALL be removed (not left at stale value)

### Requirement: Query status breakdown metrics
The metrics server SHALL register `pihole_dns_queries_by_status{target, status}` as a GaugeVec and populate one gauge per resolution status (GRAVITY, FORWARDED, CACHE, REGEX, etc.).

#### Scenario: Query status populated
- **WHEN** Stats contains status {"GRAVITY": 800, "FORWARDED": 8000, "CACHE": 3000}
- **THEN** three gauge values SHALL be set with the respective status labels

### Requirement: Reply type breakdown metrics
The metrics server SHALL register `pihole_dns_replies_by_type{target, reply_type}` as a GaugeVec and populate one gauge per reply type (CNAME, IP, NXDOMAIN, etc.).

#### Scenario: Reply types populated
- **WHEN** Stats contains replies {"CNAME": 3000, "IP": 7000, "NXDOMAIN": 200}
- **THEN** three gauge values SHALL be set with the respective reply_type labels

### Requirement: Upstream resolver metrics
The metrics server SHALL provide an `UpdateUpstreams(target string, upstreams []UpstreamStats)` method that registers and populates:

- `pihole_upstream_queries{target, upstream, name, port}` — queries routed to each upstream
- `pihole_upstream_response_seconds{target, upstream, name, port}` — average response time
- `pihole_upstream_response_variance{target, upstream, name, port}` — response time variance

#### Scenario: Upstream metrics populated
- **WHEN** UpdateUpstreams is called with two upstreams (1.1.1.1:53 avg 25ms, 8.8.8.8:53 avg 30ms)
- **THEN** six gauge values SHALL be set (3 metrics x 2 upstreams) with correct labels

#### Scenario: Stale upstreams cleared
- **WHEN** an upstream that was present in the previous scrape is absent in the current scrape
- **THEN** metrics for that upstream SHALL be removed

### Requirement: Blocking status metric
The metrics server SHALL populate `pihole_status{target}` with 1 when blocking is enabled and 0 when disabled, sourced from GetBlockingStatus().

#### Scenario: Blocking enabled
- **WHEN** GetBlockingStatus returns true
- **THEN** `pihole_status{target}` SHALL be 1

#### Scenario: Blocking disabled
- **WHEN** GetBlockingStatus returns false
- **THEN** `pihole_status{target}` SHALL be 0
