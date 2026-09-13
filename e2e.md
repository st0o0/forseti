# E2E Test Scenarios

Scenario coverage for the per-target API concurrency gate.
Run these after changes to `collector`, `session`, `worker`, `gravity`, or `metrics`.

## Unit test scenarios

Run with `go test ./internal/collector/ ./internal/session/ ./internal/worker/ -timeout 60s`

### Gate basics (session/pool_test.go)

| Test | Scenario | Verifies |
|------|----------|----------|
| `TestPoolAcquireRelease` | Acquire 2 slots, TryAcquire fails at capacity, Release opens slot | Basic semaphore acquire/release |
| `TestPoolTryAcquireAvailable` | TryAcquire on empty gate | Non-blocking acquire succeeds |
| `TestPoolReleaseUnknownTarget` | Release on nonexistent target | No panic on unknown target |
| `TestPoolGateLazyCreation` | TryAcquire without prior SetAPIConfig | Gate created with default capacity (4) |
| `TestPoolConcurrentAcquire` | Acquire blocks when full, unblocks on Release | Blocking behavior under contention |
| `TestPoolSetAPIConfigUpdatesGate` | Change max_concurrent from 2 to 4 at runtime | Hot-reload gate capacity change |
| `TestPoolGateIndependentTargets` | Two targets with max_concurrent=1, hold one | Target isolation -- holding alpha doesn't block beta |
| `TestPoolMultipleReleaseSafe` | Release called 3x after single Acquire | Extra releases don't corrupt gate state |

### Collector gate scenarios (collector/collector_test.go)

| Test | Scenario | Verifies |
|------|----------|----------|
| `TestCollectStaleOnBusy` | Gate held, collector scrapes | Stale cache served, no API call, stale metric increments |
| `TestCollectAfterBusy` | Gate released after stale serve | Collector fetches fresh data on next scrape |
| `TestCollectMultiTargetOneBusy` | 2 targets, one busy | Fast target: success. Busy target: stale. No spillover |
| `TestCollectConcurrentReconcileAndScrape` | Cache expired + gate held -> released -> scrape | Stale on first, fresh on second |
| `TestCollectPerTargetTimeout` | 50ms vs 5s timeout, server takes 200ms | Short: timeout error. Long: success |
| `TestCollectGateHighConcurrency` | max_concurrent=4, 3 slots externally held | Collector takes 4th slot, succeeds |
| `TestCollectGateFullHighConcurrency` | max_concurrent=2, both slots held | Collector serves stale |
| `TestCollectRepeatedStaleWhileBusy` | Gate held, 3 consecutive scrapes -> release -> scrape | 3x stale counter, then recovery fetch |
| `TestCollectDefaultConfigNoAPIBlock` | Target without `api` config block | Default timeout/concurrency works |
| `TestCollectGateReleasedOnSessionError` | Auth fails while gate is held | Gate released despite session error, slot available for next caller |
| `TestCollectThreeTargetsMixedState` | 3 targets: healthy + busy + cached (within TTL) | Each behaves independently: cache hit / stale / cache hit |
| `TestCollectBusyTargetCacheExpiredServeStale` | Fetch succeeds, TTL expires, gate held, scrape | Stale served from previous fetch, no new fetch |
| `TestCollectSlowServerDifferentTimeouts` | 3 targets with 50ms/300ms/5s, server takes 150ms | Fast: timeout. Medium: success. Slow: success |
| `TestCollectTimeout` | Per-target timeout 50ms, server sleeps 5s | Context expires, collector returns quickly |

### Worker gate scenarios (worker/worker_test.go)

| Test | Scenario | Verifies |
|------|----------|----------|
| `TestReconcileAcquiresAndReleasesGate` | Normal reconcile cycle | Acquire before API calls, Release after |
| `TestReconcileReleasesGateOnSessionError` | Session fails | Gate released on error path |
| `TestReconcileReleasesGateOnContentError` | Content reconcile fails | Gate released despite downstream error |
| `TestReconcileGateSequentialCycles` | 3 consecutive reconcile cycles | 3 acquires + 3 releases (no leaks) |

### Gravity gate scenarios (gravity/scheduler_test.go)

| Test | Scenario | Verifies |
|------|----------|----------|
| `TestTriggerNowRecordsMetrics` | TriggerNow with gate | Gravity acquires gate, runs, releases, records metrics |
| `TestTriggerNowSessionError` | Session fails during gravity | Gate released despite error |
| `TestTriggerNowSkipsInFlight` | Duplicate gravity trigger | inFlight guard prevents double execution |
| `TestTriggerNowIndependentTargets` | Two targets trigger simultaneously | Independent gates, no blocking between targets |

### Pihole client (pihole/errors_test.go)

| Test | Scenario | Verifies |
|------|----------|----------|
| `TestGravityConcurrentSafety` | Gravity + stats call on same client simultaneously | Local http.Client for gravity, no race on httpClient field |
| `TestGravityExtendedTimeout` | Gravity uses separate timeout | httpClient.Timeout unchanged after gravity call |

### Config (config/config_test.go)

| Test | Scenario | Verifies |
|------|----------|----------|
| `TestAPIConfigDefaults` | No api block | TimeoutOrDefault()=30s, MaxConcurrentOrDefault()=4 |
| `TestAPIConfigExplicit` | api.timeout=45s, api.max_concurrent=1 | Parsed correctly from YAML |
| `TestAPIConfigNegativeTimeout` | api.timeout=-5s | Validation error |
| `TestAPIConfigNegativeMaxConcurrent` | api.max_concurrent=-1 | Validation error |

## Docker E2E scenarios

Run with `docker compose -f docker-compose.dev.yml --env-file .env.dev up --build`

### Scenario: Normal operation (no api block)

**Config:** Two targets, default settings.

**Expected:**
- Both targets reconcile every 30s
- Collector fetches stats on first scrape, cache hits on subsequent
- Gravity triggers on adlist changes
- `forseti_collector_cache_stale_total` is not emitted
- `forseti_target_health` = 0 (healthy) for both

**Verify:**
```bash
curl -s localhost:9099/metrics | grep forseti_collector_fetches_total
curl -s localhost:9099/metrics | grep forseti_target_health
```

### Scenario: Per-target timeout too short

**Config:**
```yaml
targets:
  - name: pihole-beta
    api:
      timeout: 5ms
```

**Expected:**
- Beta login fails: `Client.Timeout exceeded while awaiting headers`
- Beta: `target_reachable=0`, `target_health=1` (degraded)
- Alpha unaffected: `target_reachable=1`, `target_health=0`
- Collector records `fetches_total{status="error"}` for beta

**Verify:**
```bash
docker logs forseti 2>&1 | grep "session error.*pihole-beta"
curl -s localhost:9099/metrics | grep forseti_target_reachable
```

### Scenario: max_concurrent=1 serialized access

**Config:**
```yaml
targets:
  - name: pihole-beta
    api:
      timeout: 500ms
      max_concurrent: 1
```

**Expected:**
- Beta reconcile succeeds (500ms is enough for local containers)
- If a scrape happens during reconcile (~500ms window), collector serves stale
- `forseti_collector_cache_stale_total{target="pihole-beta"}` may increment
- Alpha completely independent

**Verify:**
```bash
curl -s localhost:9099/metrics | grep forseti_collector_cache_stale
```

### Scenario: Graceful degradation on slow target

**Config:**
```yaml
targets:
  - name: slow-target
    api:
      timeout: 45s
      max_concurrent: 1
  - name: fast-target
    api:
      timeout: 10s
      max_concurrent: 4
```

**Expected:**
- Fast target: normal operation, high concurrency
- Slow target: serialized access, longer timeouts tolerated
- If slow target's reconcile takes >10s, collector serves stale (gate held)
- Fast target never blocked by slow target
- Scrape handler timeout adjusts to max(45s, 10s) + 5s = 50s

### Scenario: Hot-reload changes API config

**Steps:**
1. Start with default config (no api block)
2. Edit config file: add `api.max_concurrent: 1` to a target
3. Wait for hot-reload (next reconcile tick)
4. Verify new concurrency limit applies

**Expected:**
- `config reloaded successfully` in logs
- Gate capacity changes from 4 to 1
- Collector may start seeing stale serves if reconcile and scrape overlap

### Scenario: Recovery after target comes back

**Steps:**
1. Start with a target that's unreachable (wrong URL or down)
2. Target starts returning errors, health degrades
3. Fix the target (bring it up)
4. Worker backoff expires, next reconcile succeeds

**Expected:**
- `target_health` goes from 0 -> 1 (degraded) on failures
- Backoff timer activates (skipped cycles in logs)
- After recovery: `target_health` returns to 0
- Gate was properly released on every failed attempt
