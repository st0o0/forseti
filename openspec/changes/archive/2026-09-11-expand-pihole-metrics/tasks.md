## 1. Remove dead metrics

- [x] 1.1 Remove `pihole_gravity_size`, `pihole_cache_size`, `pihole_cache_inserted`, `pihole_cache_evicted`, `pihole_ftl_memory_bytes` from metrics.go (struct fields, registration, any references)
- [x] 1.2 Update metrics_test.go to remove expectations for deleted metrics

## 2. Expand Stats struct and summary parsing

- [x] 2.1 Add fields to Stats struct: Forwarded, Cached, UniqueDomains, Frequency (float64), ClientsActive, ClientsTotal, QueryTypes (map[string]int), QueryStatus (map[string]int), ReplyTypes (map[string]int)
- [x] 2.2 Extend UnmarshalJSON to parse queries.forwarded, queries.cached, queries.unique_domains, queries.frequency, queries.types, queries.status, queries.replies, clients.active, clients.total
- [x] 2.3 Add unit tests for expanded Stats parsing with full summary JSON fixture

## 3. Add GetBlockingStatus method

- [x] 3.1 Add `GetBlockingStatus() (bool, error)` method to Client calling `GET /api/dns/blocking`
- [x] 3.2 Add unit test for GetBlockingStatus (enabled and disabled scenarios)

## 4. Add GetUpstreams method

- [x] 4.1 Add UpstreamStats struct (IP, Name string, Port int, Count int, ResponseTime, ResponseVariance float64)
- [x] 4.2 Add `GetUpstreams() ([]UpstreamStats, error)` method to Client calling `GET /api/stats/upstreams`
- [x] 4.3 Add unit tests for GetUpstreams with multi-upstream JSON fixture

## 5. Register new Prometheus metrics

- [x] 5.1 Add GaugeVec fields to Server struct: piholeQueriesForwarded, piholeQueriesCached, piholeUniqueDomains, piholeRequestFrequency, piholeClientsActive, piholeClientsTotal
- [x] 5.2 Add GaugeVec fields: piholeQueryTypes (labels: target, query_type), piholeQueryStatus (labels: target, status), piholeReplyTypes (labels: target, reply_type)
- [x] 5.3 Add GaugeVec fields: piholeUpstreamQueries, piholeUpstreamResponse, piholeUpstreamVariance (labels: target, upstream, name, port)
- [x] 5.4 Register all new metrics in NewServer

## 6. Populate new metrics

- [x] 6.1 Expand UpdateStats to set all new summary-based metrics (forwarded, cached, unique_domains, frequency, clients, query types, query status, reply types) with Reset() on dynamic GaugeVecs before re-setting
- [x] 6.2 Add UpdateBlockingStatus(target string, enabled bool) method
- [x] 6.3 Add UpdateUpstreams(target string, upstreams []UpstreamStats) method with Reset() before re-setting
- [x] 6.4 Add unit tests verifying all new metrics are correctly set after UpdateStats, UpdateBlockingStatus, and UpdateUpstreams calls

## 7. Wire up in caller

- [x] 7.1 Find the stats collection call site and add GetUpstreams + UpdateUpstreams alongside existing GetStats + UpdateStats
- [x] 7.2 Add GetBlockingStatus + UpdateBlockingStatus to the stats collection cycle
- [x] 7.3 Verify end-to-end with `go test -race ./...` and `go vet ./...`

## 8. Update main specs

- [x] 8.1 Sync delta specs to openspec/specs/pihole-api/spec.md and openspec/specs/metrics/spec.md via opsx:sync
