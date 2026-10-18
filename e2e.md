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

## Metrics verification checklist

After at least one reconcile cycle, verify these metrics exist at `/metrics`:

### Always present

| Metric | Check |
|--------|-------|
| `forseti_build_info` | Has `version` and `mode` labels |
| `forseti_target_reachable{target="pihole-alpha"}` | Value = 1 |
| `forseti_target_reachable{target="pihole-beta"}` | Value = 1 |
| `forseti_target_health{target="pihole-alpha"}` | Value = 0 (healthy) |
| `forseti_config_adlists` | Value >= 1 |
| `forseti_config_deny_domains` | Value >= 1 |
| `forseti_config_allow_domains` | Value >= 1 |
| `forseti_collector_fetches_total` | Present with `status="success"` |

### Stats (default enabled)

| Metric | Check |
|--------|-------|
| `forseti_dns_queries{target="pihole-alpha"}` | Value >= 0 |
| `forseti_dns_queries_blocked` | Present |
| `forseti_blocked_percentage` | Value 0-100 |
| `forseti_domains_blocked` | Value >= 0 |
| `forseti_gravity_last_update_timestamp` | Value > 0 (unix timestamp) |
| `forseti_clients_active` | Value >= 0 |
| `forseti_queries_forwarded` | Present |
| `forseti_queries_cached` | Present |

### Blocking (default enabled)

| Metric | Check |
|--------|-------|
| `forseti_blocking_status{target="pihole-alpha"}` | Value = 1 (enabled) |

### DHCP (default enabled)

| Metric | Check |
|--------|-------|
| `forseti_dhcp_leases_active` | Present per target |

### Reconcile (default enabled)

| Metric | Check |
|--------|-------|
| `forseti_reconcile_runs_total` | Present with `status="success"` |
| `forseti_reconcile_duration_seconds` | Present |
| `forseti_reconcile_last_run_timestamp` | Value > 0 |
| `forseti_reconcile_last_success_timestamp` | Value > 0 |

## Reconcile verification checklist

After reconcile, verify resources on Pi-hole alpha via API.

### Setup: Authenticate

```bash
SID=$(curl -s -X POST http://localhost:18080/api/auth \
  -H 'Content-Type: application/json' \
  -d '{"password":"alpha-dev-password"}' | jq -r '.session.sid')
```

### Groups

| Check | Command | Expected |
|-------|---------|----------|
| Groups exist | `curl -s http://localhost:18080/api/groups -H "X-FTL-SID: $SID"` | Groups include "default" and "testing" |
| Marker present | Check comment field | `[forseti]` in comment for managed groups |

### Adlists (block)

| Check | Command | Expected |
|-------|---------|----------|
| Block-lists exist | `curl -s "http://localhost:18080/api/lists?type=block" -H "X-FTL-SID: $SID"` | At least 2 adlists from config |
| Marker present | Check comment field | `[forseti]` in comments |
| StevenBlack list | Check addresses | `https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts` present |

### Allowlists

| Check | Command | Expected |
|-------|---------|----------|
| Allow-lists exist | `curl -s "http://localhost:18080/api/lists?type=allow" -H "X-FTL-SID: $SID"` | At least 1 allowlist from config |
| Marker present | Check comment field | `[forseti]` in comments |
| anudeepND list | Check addresses | Whitelist URL present |
| Group assignment | Check groups field | Assigned to "default" group |

### Deny domains

| Check | Command | Expected |
|-------|---------|----------|
| Deny exact | `curl -s http://localhost:18080/api/domains/deny/exact -H "X-FTL-SID: $SID"` | `ads.example.com` and `tracking.example.com` present |
| Marker present | Check comment field | `[forseti]` as comment |

### Allow domains

| Check | Command | Expected |
|-------|---------|----------|
| Allow exact | `curl -s http://localhost:18080/api/domains/allow/exact -H "X-FTL-SID: $SID"` | `allowed.example.com` and `safe.example.com` present |

### Local DNS

| Check | Command | Expected |
|-------|---------|----------|
| DNS records | `curl -s http://localhost:18080/api/config/dns/hosts -H "X-FTL-SID: $SID"` | Contains `192.168.1.50 nas.lan` and `192.168.1.60 printer.lan` |

### Clients

| Check | Command | Expected |
|-------|---------|----------|
| Clients exist | `curl -s http://localhost:18080/api/clients -H "X-FTL-SID: $SID"` | `192.168.1.0/24` present with `[forseti]` marker |

### Manual entry preservation

| Check | Steps | Expected |
|-------|-------|----------|
| Manual adlist preserved | Add adlist without `[forseti]` marker via Pi-hole UI, run reconcile | Manual adlist untouched |

## New collector toggle scenarios

These tests require specific toggles enabled. If the dev config does not have
them enabled, add to `forseti.dev.yml` under `metrics.collectors`:

```yaml
metrics:
  collectors:
    top_domains: true
    top_clients: true
    system: true
    history: true
  top_stats_count: 10
  history_interval: 1m
```

### Top domains (toggle: top_domains)

| Check | Method | Expected |
|-------|--------|----------|
| Metric registered | `grep forseti_top_queried_domains /metrics` | Present with `target` and `domain` labels |
| Metric registered | `grep forseti_top_blocked_domains /metrics` | Present with `target` and `domain` labels |
| Count matches config | Count domain labels per target | At most `top_stats_count` entries |
| Toggle disabled | Remove toggle, reload, check | Metrics not present |

### Top clients (toggle: top_clients)

| Check | Method | Expected |
|-------|--------|----------|
| Metric registered | `grep forseti_top_clients /metrics` | Present with `target` and `client` labels |
| Count matches config | Count client labels per target | At most `top_stats_count` entries |

### System info (toggle: system)

| Check | Method | Expected |
|-------|--------|----------|
| CPU metric | `grep forseti_pihole_cpu_percent /metrics` | Value 0-100 per target |
| Memory metric | `grep forseti_pihole_memory_percent /metrics` | Value 0-100 per target |
| Load metrics | `grep forseti_pihole_load_1m /metrics` | Value >= 0 per target |
| Load 5m | `grep forseti_pihole_load_5m /metrics` | Value >= 0 per target |
| Load 15m | `grep forseti_pihole_load_15m /metrics` | Value >= 0 per target |
| Temperature | `grep forseti_pihole_temperature_celsius /metrics` | Present if hardware reports temp, absent otherwise |

### Query history (toggle: history)

| Check | Method | Expected |
|-------|--------|----------|
| Total metric | `grep forseti_query_history_total /metrics` | Present per target |
| Blocked metric | `grep forseti_query_history_blocked /metrics` | Present per target |
| Tick-based | Wait > history_interval, check values change | Values update on tick interval |

## Health and status endpoint scenarios

### /healthz

| Check | Method | Expected |
|-------|--------|----------|
| Healthy response | `curl -s http://localhost:9099/healthz` | HTTP 200 |
| After stall | Stop reconcile (pause forseti), wait > 2x interval | HTTP 503 or degraded |

### /status

| Check | Method | Expected |
|-------|--------|----------|
| Status response | `curl -s http://localhost:9099/status` | JSON with per-target status |
| Target info | Check response | Each target has `name`, `health`, `lastRun` |

### /status/history

| Check | Method | Expected |
|-------|--------|----------|
| History response | `curl -s http://localhost:9099/status/history` | JSON array of run results |
| Entries present | Check array length | At least 1 entry after first reconcile |

## Group name validation scenarios

### Config mode

| Check | Method | Expected |
|-------|--------|----------|
| CRLF rejected | Add group with `\n` in name to config YAML | Config validation fails with error |
| Null rejected | Add group with `\x00` in name | Config validation fails |
| Valid name | Normal group names | Config loads successfully |

## CNAME sync scenarios (sync mode)

These require a sync-mode config. Use `forseti.sync.yml` with `allowlists` and
`cnames` in the resources list.

| Check | Method | Expected |
|-------|--------|----------|
| CNAME in valid resources | Config with `cnames` in `sync.resources` | Config loads without error |
| Allowlists in valid resources | Config with `allowlists` in `sync.resources` | Config loads without error |

## Config validation scenarios

### Allowlist validation

| Check | Method | Expected |
|-------|--------|----------|
| Duplicate URL | Two allowlists with same URL in config | Validation error |
| Undefined group | Allowlist referencing nonexistent group | Validation error |
| Valid allowlist | Allowlist with valid URL and groups | Config loads |

### New toggle config

| Check | Method | Expected |
|-------|--------|----------|
| top_stats_count > 100 | Set to 200 | Validation error |
| history_interval < 1m | Set to 30s | Validation error |
| Defaults | No explicit values | top_stats_count=10, history_interval=5m |
