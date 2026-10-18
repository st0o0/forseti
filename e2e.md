# E2E Test Scenarios

Scenario coverage for Forseti's config mode and sync mode.
Run these after changes to `collector`, `session`, `worker`, `gravity`, `metrics`,
`reconcile`, `sync`, or `pihole` client.

## Quick reference

| Mode | Docker command | Config file |
|------|---------------|-------------|
| Config | `docker compose -f docker-compose.dev.yml --env-file .env.dev up -d --build` | forseti.dev.yml |
| Sync | `docker compose -f docker-compose.dev.yml -f docker-compose.sync.yml --env-file .env.dev up -d --build` | forseti.sync.yml |

Sync mode requires seeding the primary first: `bash e2e-seed-primary.sh`

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

---

## Sync Mode E2E Scenarios

These tests verify primary-to-replica synchronization. Run them as a separate
Docker stack session after completing the config-mode tests.

### Setup

```bash
# 1. Tear down any running stack
docker compose -f docker-compose.dev.yml --env-file .env.dev down -v

# 2. Start Pi-hole containers only (no Forseti yet)
docker compose -f docker-compose.dev.yml --env-file .env.dev up -d pihole-alpha pihole-beta

# 3. Wait for both Pi-holes to be healthy
#    Poll http://localhost:18080/admin/ and http://localhost:18081/admin/

# 4. Seed the primary with test data
bash e2e-seed-primary.sh

# 5. Start Forseti in sync mode
docker compose -f docker-compose.dev.yml -f docker-compose.sync.yml --env-file .env.dev up -d --build forseti
```

### Setup: Authenticate to both Pi-holes

```bash
ALPHA_SID=$(curl -s -X POST http://localhost:18080/api/auth \
  -H 'Content-Type: application/json' \
  -d '{"password":"alpha-dev-password"}' | jq -r '.session.sid')

BETA_SID=$(curl -s -X POST http://localhost:18081/api/auth \
  -H 'Content-Type: application/json' \
  -d '{"password":"beta-dev-password"}' | jq -r '.session.sid')
```

### Scenario: Sync groups

| Check | Command | Expected |
|-------|---------|----------|
| Primary has group | `curl -s http://localhost:18080/api/groups -H "X-FTL-SID: $ALPHA_SID"` | `e2e-sync-group` present |
| Replica synced | `curl -s http://localhost:18081/api/groups -H "X-FTL-SID: $BETA_SID"` | `e2e-sync-group` present with `[forseti-sync]` marker |
| Default group untouched | Check replica groups | `Default` group (id=0) still present, no marker |

### Scenario: Sync adlists (block)

| Check | Command | Expected |
|-------|---------|----------|
| Primary has adlist | `curl -s "http://localhost:18080/api/lists?type=block" -H "X-FTL-SID: $ALPHA_SID"` | StevenBlack hosts URL present |
| Replica synced | `curl -s "http://localhost:18081/api/lists?type=block" -H "X-FTL-SID: $BETA_SID"` | Same URL present with `[forseti-sync]` marker |

### Scenario: Sync allowlists

| Check | Command | Expected |
|-------|---------|----------|
| Primary has allowlist | `curl -s "http://localhost:18080/api/lists?type=allow" -H "X-FTL-SID: $ALPHA_SID"` | anudeepND whitelist URL present |
| Replica synced | `curl -s "http://localhost:18081/api/lists?type=allow" -H "X-FTL-SID: $BETA_SID"` | Same URL present with `[forseti-sync]` marker |

### Scenario: Sync deny domains

| Check | Command | Expected |
|-------|---------|----------|
| Primary deny exact | `curl -s http://localhost:18080/api/domains/deny/exact -H "X-FTL-SID: $ALPHA_SID"` | `sync-deny.example.com` present |
| Replica deny exact | `curl -s http://localhost:18081/api/domains/deny/exact -H "X-FTL-SID: $BETA_SID"` | `sync-deny.example.com` with `[forseti-sync]` |
| Primary deny regex | `curl -s http://localhost:18080/api/domains/deny/regex -H "X-FTL-SID: $ALPHA_SID"` | `.*tracking\.sync\..*` present |
| Replica deny regex | `curl -s http://localhost:18081/api/domains/deny/regex -H "X-FTL-SID: $BETA_SID"` | Same regex with `[forseti-sync]` |

### Scenario: Sync allow domains

| Check | Command | Expected |
|-------|---------|----------|
| Primary allow exact | `curl -s http://localhost:18080/api/domains/allow/exact -H "X-FTL-SID: $ALPHA_SID"` | `sync-allow.example.com` present |
| Replica allow exact | `curl -s http://localhost:18081/api/domains/allow/exact -H "X-FTL-SID: $BETA_SID"` | `sync-allow.example.com` with `[forseti-sync]` |

### Scenario: Sync local DNS

| Check | Command | Expected |
|-------|---------|----------|
| Primary DNS | `curl -s http://localhost:18080/api/config/dns/hosts -H "X-FTL-SID: $ALPHA_SID"` | `192.168.10.1 sync-nas.lan` and `192.168.10.2 sync-printer.lan` |
| Replica DNS | `curl -s http://localhost:18081/api/config/dns/hosts -H "X-FTL-SID: $BETA_SID"` | Same entries present |

### Scenario: Sync CNAME records

| Check | Command | Expected |
|-------|---------|----------|
| Primary CNAME | `curl -s http://localhost:18080/api/config/dns/cnameRecords -H "X-FTL-SID: $ALPHA_SID"` | `app.sync.lan,server.sync.lan` and `web.sync.lan,lb.sync.lan` |
| Replica CNAME | `curl -s http://localhost:18081/api/config/dns/cnameRecords -H "X-FTL-SID: $BETA_SID"` | Same CNAME entries present |
| FTL restart log | `docker logs forseti` | `CNAME changes will trigger FTL restart` message for beta |

### Scenario: Sync clients

| Check | Command | Expected |
|-------|---------|----------|
| Primary client | `curl -s http://localhost:18080/api/clients -H "X-FTL-SID: $ALPHA_SID"` | `10.0.0.0/8` present |
| Replica client | `curl -s http://localhost:18081/api/clients -H "X-FTL-SID: $BETA_SID"` | `10.0.0.0/8` present with `[forseti-sync]` |

### Scenario: Sync marker isolation

| Check | Steps | Expected |
|-------|-------|----------|
| Manual replica entry preserved | Add adlist directly on beta without marker, wait for sync | Manual adlist still present, not deleted |
| Manual replica group preserved | Add group directly on beta without marker, wait for sync | Manual group still present |

### Scenario: Orphan cleanup

| Check | Steps | Expected |
|-------|-------|----------|
| Remove from primary | Delete `sync-deny.example.com` from alpha | After next sync: domain removed from beta (had `[forseti-sync]` marker) |
| Manual entry kept | Manual entry on beta without marker | Not deleted during orphan cleanup |

### Scenario: Sync metrics

| Check | Method | Expected |
|-------|--------|----------|
| Sync runs | `grep forseti_reconcile_runs_total /metrics` | Present with `status="success"` for both targets |
| Sync duration | `grep forseti_reconcile_duration /metrics` | Present per target |
| Target health | `grep forseti_target_health /metrics` | Both targets health=0 after successful sync |

### Scenario: Sync recovery

| Check | Steps | Expected |
|-------|-------|----------|
| Replica down | `docker pause forseti-pihole-beta` | Sync cycle logs error for beta, alpha snapshot still succeeds |
| Replica recovers | `docker unpause forseti-pihole-beta` | Next sync cycle succeeds, beta back to healthy |
| Primary down | `docker pause forseti-pihole-alpha` | All replicas skipped (primary snapshot failed) |
| Primary recovers | `docker unpause forseti-pihole-alpha` | Next cycle succeeds normally |

### Scenario: local_dns_purge behavior

| Check | Steps | Expected |
|-------|-------|----------|
| Purge enabled (default) | Add DNS record only on beta, sync runs | Record deleted from beta |
| Purge disabled | Set `sync.local_dns_purge: false`, reload, add DNS only on beta | Record kept on beta |

---

## Config Mode E2E: Detailed Reconcile Scenarios

Additional config-mode scenarios beyond the basic reconcile checks.

### Scenario: Adlist update (group change)

| Check | Steps | Expected |
|-------|-------|----------|
| Initial state | Config has adlist in group "default" | Adlist created with default group |
| Change groups | Update config: add "testing" group to adlist, wait for reconcile | Adlist updated in-place (PUT), groups now include both |
| Verify in-place | Check adlist ID | Same ID (not delete+recreate) |

### Scenario: Allowlist lifecycle

| Check | Steps | Expected |
|-------|-------|----------|
| Add allowlist | Start with allowlist in config | Created with `[forseti]` marker |
| Update groups | Change allowlist groups in config | Updated in-place via PUT |
| Remove from config | Remove allowlist from config, wait for reconcile | Deleted from Pi-hole |
| Manual preserved | Add allowlist directly on Pi-hole (no marker) | Not deleted by Forseti |

### Scenario: Gravity trigger

| Check | Steps | Expected |
|-------|-------|----------|
| Adlist change triggers | Add new adlist to config | `gravity started` log after reconcile, `reason=adlist_change` |
| Allowlist change triggers | Add new allowlist to config | `gravity started` log, `reason=adlist_change` |
| Domain change: no gravity | Add new deny domain | No gravity trigger (domains don't trigger gravity) |

### Scenario: Config hot-reload

| Check | Steps | Expected |
|-------|-------|----------|
| Add new deny domain | Edit mounted config file, add deny domain | `config reloaded` in logs, domain appears on next reconcile |
| Add new allowlist | Edit config, add allowlist entry | Allowlist created after reload |
| Change toggle | Edit config, disable a collector toggle | Metric disappears after reload |
| Invalid config | Edit config to invalid YAML | `config reload failed` in logs, old config continues working |

### Scenario: Multiple targets diverge

| Check | Steps | Expected |
|-------|-------|----------|
| Alpha has extra entry | Add entry only on alpha via API | Forseti detects drift, plan shows delete |
| Beta missing entry | Delete forseti-managed entry on beta via API | Forseti detects drift, re-creates on next cycle |
| Both targets reconcile | Wait for full cycle | Both targets converge to desired state |
