## 1. Rename Gauge metrics (naming convention fix)

- [x] 1.1 Rename all Gauge metrics with `_total` suffix in `internal/metrics/metrics.go`: `pihole_queries_total` → `pihole_dns_queries`, `pihole_blocked_total` → `pihole_dns_queries_blocked`, `pihole_clients_total` → `pihole_clients_seen`, `pihole_upstream_queries_total` → `pihole_upstream_queries`, `forseti_reconcile_drift_total` → `forseti_reconcile_drift`, `forseti_config_adlists_total` → `forseti_config_adlists`, `forseti_config_deny_domains_total` → `forseti_config_deny_domains`, `forseti_config_allow_domains_total` → `forseti_config_allow_domains`
- [x] 1.2 Rename breakdown metrics: `pihole_query_types` → `pihole_dns_queries_by_type`, `pihole_query_status` → `pihole_dns_queries_by_status`, `pihole_reply_types` → `pihole_dns_replies_by_type`
- [x] 1.3 Update `internal/metrics/metrics_test.go` to use new metric names
- [x] 1.4 Verify `go test -race ./internal/metrics/...` passes

## 2. Add Collector observability metrics

- [x] 2.1 Add `forseti_collector_duration_seconds` (Histogram), `forseti_collector_fetches_total` (Counter, labels: target, status), `forseti_collector_cache_hits_total` (Counter, labels: target) to `metrics.Server` with registration and recording methods
- [x] 2.2 Instrument `collector.Collect()`: record duration of full cycle, increment cache hits for fresh targets, increment fetches with success/error status per target in `collectTarget()`
- [x] 2.3 Add tests in `internal/collector/collector_test.go` verifying cache hit and fetch counters increment correctly
- [x] 2.4 Verify `go test -race ./internal/collector/...` passes

## 3. Add Session observability metrics

- [x] 3.1 Add `OnReauth func()` field to `pihole.Client`, call it in `doJSON` after successful 401 re-authentication
- [x] 3.2 Add `forseti_session_reauth_total` (Counter, labels: target) and `forseti_session_active` (Gauge) to `metrics.Server` with recording methods
- [x] 3.3 Wire Pool.Get() to set `OnReauth` callback on new clients, increment `forseti_session_active` on new session, reset to 0 on Pool.Close()
- [x] 3.4 Add test in `internal/pihole/client_reauth_test.go` verifying `OnReauth` callback is called on successful reauth
- [x] 3.5 Verify `go test -race ./internal/session/... ./internal/pihole/...` passes

## 4. Add build info metric

- [x] 4.1 Add `forseti_build_info` (Gauge, labels: version, mode) to `metrics.Server` with `SetBuildInfo(version, mode string)` method
- [x] 4.2 Call `SetBuildInfo` from `cmd/forseti` startup after config is loaded
- [x] 4.3 Verify `go test -race ./...` passes and `go vet ./...` is clean

## 5. Final validation

- [x] 5.1 Run `golangci-lint run` and fix any findings
- [x] 5.2 Run `docker build -t forseti:ci .` to verify Docker build succeeds
