## 1. Config: Add APIConfig to Target

- [x] 1.1 Add `APIConfig` struct with `Timeout` (Duration) and `MaxConcurrent` (int) fields to `internal/config/config.go`, add `API APIConfig` field to `Target` struct
- [x] 1.2 Add defaults (Timeout=30s, MaxConcurrent=4) applied during parsing when the `api` block is omitted
- [x] 1.3 Add validation: timeout must be positive duration, max_concurrent must be >= 1
- [x] 1.4 Add tests for APIConfig parsing, defaults, and validation in `internal/config/config_test.go`

## 2. Pi-hole Client: Configurable Timeout and Gravity Race Fix

- [x] 2.1 Change `pihole.NewClient` to accept a timeout parameter, use it for `httpClient.Timeout` (default 30s when zero)
- [x] 2.2 Fix `TriggerGravity` data race: use a local `*http.Client` with extended timeout instead of swapping `c.httpClient`
- [x] 2.3 Update all `NewClient` call sites (session pool, tests) to pass the timeout
- [x] 2.4 Add test for concurrent gravity + stats call safety

## 3. Session Pool: Per-Target Gate

- [x] 3.1 Add `gates map[string]chan struct{}` and `apiConfig map[string]config.APIConfig` to `session.Pool`
- [x] 3.2 Implement `Acquire(name string)` — blocking acquire on the target's semaphore channel
- [x] 3.3 Implement `TryAcquire(name string) bool` — non-blocking acquire via select/default
- [x] 3.4 Implement `Release(name string)` — idempotent release, no-op for unknown targets
- [x] 3.5 Create gate lazily on first `Acquire`/`TryAcquire` using the target's `MaxConcurrent` config
- [x] 3.6 Add `SetAPIConfig(configs map[string]config.APIConfig)` for hot-reload updates
- [x] 3.7 Pass target's `api.timeout` to `pihole.NewClient` inside `Pool.Get`
- [x] 3.8 Add tests: acquire/release, try-acquire full/available, concurrent access, release unknown target

## 4. Worker Interfaces: Extend SessionManager

- [x] 4.1 Add `Acquire(name string)`, `TryAcquire(name string) bool`, `Release(name string)` to `SessionManager` interface in `worker/interfaces.go`
- [x] 4.2 Update any mock/fake implementations used in tests

## 5. Worker: Gate Integration

- [x] 5.1 Wrap `TargetWorker.Reconcile()` with `Acquire`/`defer Release` around the full reconcile sequence
- [x] 5.2 Ensure gate is released on all error paths (session error, settings error, content error)
- [x] 5.3 Add test: gate is acquired before API calls and released after (including on error)

## 6. Collector: Stale-on-Busy and Per-Target Timeout

- [x] 6.1 Store target API configs in the collector (pass via constructor or update method)
- [x] 6.2 Change `collectTarget` to call `TryAcquire` before API calls — on failure, serve stale cache and return
- [x] 6.3 Replace the single `ctx` with per-target `context.WithTimeout` using each target's `api.timeout`
- [x] 6.4 Add `defer Release` after successful `TryAcquire`
- [x] 6.5 Add tests: stale-on-busy behavior, per-target timeout applied, gate released after collect

## 7. Collector Observability: Stale Cache Metric

- [x] 7.1 Register `forseti_collector_cache_stale_total` counter with `{target}` label in `internal/metrics/metrics.go`
- [x] 7.2 Add `RecordCollectorCacheStale(target string)` method to metrics server
- [x] 7.3 Call `RecordCollectorCacheStale` from collector when serving stale due to busy gate
- [x] 7.4 Add test: stale counter increments on busy, does not increment on normal cache hit

## 8. Gravity Scheduler: Gate Integration

- [x] 8.1 Add gate acquire/release around gravity trigger in `processAsync` and `triggerLocked`
- [x] 8.2 Replace internal `inFlight` map with the pool's gate (or keep as supplementary guard)
- [x] 8.3 Add test: gravity acquires gate before triggering, releases after

## 9. Metrics Scrape Handler: Dynamic Timeout

- [x] 9.1 Replace hardcoded `context.WithTimeout(10s)` in `metrics.go` scrape handler with `max(target.api.timeout) + 5s`
- [x] 9.2 Add method to compute scrape timeout from target configs
- [x] 9.3 Update scrape timeout on hot-reload when target configs change

## 10. Wiring and Integration

- [x] 10.1 Wire APIConfig from parsed config to session pool in `cmd/forseti/main.go`
- [x] 10.2 Wire APIConfig to collector for per-target timeouts
- [x] 10.3 Update hot-reload path to call `pool.SetAPIConfig` and `collector.UpdateTargets` with new configs
- [x] 10.4 Run `go test ./...` to verify all tests pass
- [x] 10.5 Run `go vet ./...` to verify (golangci-lint not available locally)
