## 1. Metric Registration

- [x] 1.1 Define Server struct with all metric vecs (counters, gauges, histograms)
- [x] 1.2 Register forseti_* metrics: reconcile_runs_total, reconcile_duration_seconds, reconcile_changes_total, reconcile_drift_total, target_reachable
- [x] 1.3 Register forseti_config_* gauges: adlists_total, deny_domains_total, allow_domains_total
- [x] 1.4 Register pihole_* gauges: queries_total, blocked_total, blocked_percentage, gravity_size, cache_size, status, ftl_memory_bytes

## 2. Recording Methods

- [x] 2.1 Implement RecordReconcile method: update runs counter, duration histogram, changes counter, drift gauge
- [x] 2.2 Implement UpdateStats method: update pihole_* gauges and target_reachable from Stats
- [x] 2.3 Implement SetConfigMetrics method: set config gauges from loaded Config

## 3. HTTP Server

- [x] 3.1 Implement NewServer(port, path) constructor
- [x] 3.2 Implement Start() and Shutdown() lifecycle methods

## 4. Tests

- [x] 4.1 Test metric registration (no panics on duplicate registration)
- [x] 4.2 Test RecordReconcile updates correct metrics
- [x] 4.3 Test UpdateStats updates pihole_* gauges
- [x] 4.4 Run go vet clean
