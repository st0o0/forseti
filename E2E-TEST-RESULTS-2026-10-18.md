# E2E Test Results - 2026-10-18

**Stack**: Forseti dev (feat/pihole-v6-expansion), Pi-hole v6.7 x2
**Branch**: feat/pihole-v6-expansion
**Duration**: ~25 minutes (config mode + sync mode)
**Tests**: Unit (go test) + Docker E2E (config mode + sync mode)

## Summary

| Phase | Pass | Fail | Total |
|-------|------|------|-------|
| Unit Tests | 1 | 0 | 1 |
| Config: Metrics (standard) | 22 | 0 | 22 |
| Config: Metrics (new toggles) | 7 | 0 | 7 |
| Config: Reconcile | 7 | 0 | 7 |
| Config: Health/Status | 3 | 0 | 3 |
| Config: Recovery | 4 | 0 | 4 |
| Sync: Resources | 10 | 0 | 10 |
| Sync: Recovery | 6 | 0 | 6 |
| **Total** | **60** | **0** | **60** |

---

## Config Mode Results

### Phase 1: Unit Tests

| Test | Status | Notes |
|------|--------|-------|
| go test ./... | PASS | 758 tests across 16 packages |

### Phase 2: Metrics — Always Present

| Test | Status | Notes |
|------|--------|-------|
| forseti_build_info | PASS | version=dev, mode=config |
| forseti_target_reachable alpha=1 | PASS | |
| forseti_target_reachable beta=1 | PASS | |
| forseti_target_health alpha=0 | PASS | healthy |
| forseti_config_adlists | PASS | >= 1 |
| forseti_config_deny_domains | PASS | >= 1 |
| forseti_config_allow_domains | PASS | >= 1 |
| forseti_collector_fetches_total success | PASS | |

### Phase 2: Metrics — Stats

| Test | Status | Notes |
|------|--------|-------|
| forseti_dns_queries | PASS | Per target |
| forseti_dns_queries_blocked | PASS | |
| forseti_blocked_percentage | PASS | |
| forseti_domains_blocked | PASS | |
| forseti_gravity_last_update_timestamp | PASS | Unix timestamp > 0 |
| forseti_clients_active | PASS | |
| forseti_queries_forwarded | PASS | |
| forseti_queries_cached | PASS | |

### Phase 2: Metrics — Blocking / DHCP / Reconcile

| Test | Status | Notes |
|------|--------|-------|
| forseti_blocking_status | PASS | Value=1 (enabled) |
| forseti_dhcp_leases_active | PASS | Per target |
| forseti_reconcile_runs_total success | PASS | |
| forseti_reconcile_duration_seconds | PASS | |
| forseti_reconcile_last_run_timestamp | PASS | > 0 |
| forseti_reconcile_last_success_timestamp | PASS | > 0 |

### Phase 2: Metrics — System Info (NEW, toggle: system)

| Test | Status | Notes |
|------|--------|-------|
| forseti_pihole_cpu_percent | PASS | 0-100 per target |
| forseti_pihole_memory_percent | PASS | 0-100 per target |
| forseti_pihole_load_1m | PASS | >= 0 per target |
| forseti_pihole_load_5m | PASS | >= 0 per target |
| forseti_pihole_load_15m | PASS | >= 0 per target |

Note: forseti_pihole_temperature_celsius not emitted — expected for Docker containers (no hardware temp sensor).

### Phase 2: Metrics — History (NEW, toggle: history)

| Test | Status | Notes |
|------|--------|-------|
| forseti_query_history_total | PASS | Per target (value=0, no DNS traffic) |
| forseti_query_history_blocked | PASS | Per target (value=0, no DNS traffic) |

### Phase 2: Metrics — Top Stats (NEW, toggles: top_domains, top_clients)

No data points emitted — Pi-hole dev instances have zero DNS traffic. The toggles are enabled and metrics are registered, but GaugeVec only appears when populated. This is correct behavior.

### Phase 3: Config Mode — Reconcile Verification

| Test | Status | Notes |
|------|--------|-------|
| Groups created with [forseti] marker | PASS | 1 managed group (testing) |
| Block-lists (adlists) created | PASS | 2 adlists: StevenBlack + EasyPrivacy |
| Allow-lists created | PASS | 1 allowlist: anudeepND whitelist |
| Deny domains (exact) | PASS | ads.example.com + tracking.example.com |
| Allow domains (exact) | PASS | allowed.example.com + safe.example.com |
| Local DNS records | PASS | nas.lan + printer.lan |
| Clients | PASS | 192.168.1.0/24 with [forseti] |

### Phase 5: Health & Status Endpoints

| Test | Status | Notes |
|------|--------|-------|
| /healthz returns 200 | PASS | |
| /status has target info | PASS | Both targets health=healthy |
| /status/history returns JSON | PASS | History entries available |

### Phase 5: Config Mode — Recovery Test

| Test | Status | Notes |
|------|--------|-------|
| Alpha paused → health degrades | PASS | target_health=1, target_reachable=0 |
| Beta unaffected during alpha outage | PASS | target_health=0 |
| Alpha unpaused → recovers | PASS | target_health back to 0 |
| Alpha reachable after recovery | PASS | target_reachable=1 |

---

## Sync Mode Results

### Phase 6: Setup

| Step | Status | Notes |
|------|--------|-------|
| Docker teardown | PASS | Volumes removed |
| Pi-holes started | PASS | Alpha + Beta healthy |
| Primary seeded | PASS | 11 resources created/confirmed |
| Forseti sync mode started | PASS | Using docker-compose.sync.yml override |
| First sync cycle complete | PASS | Logged in sync mode |

### Phase 7: Sync — Resource Verification on Replica (Beta)

| Test | Status | Notes |
|------|--------|-------|
| Groups synced | PASS | e2e-sync-group with [forseti-sync] marker |
| Default group untouched | PASS | No marker on Default group |
| Adlists (block) present | PASS | StevenBlack exists (Pi-hole default, pre-installed on both) |
| Allowlists synced | PASS | anudeepND whitelist with [forseti-sync] marker |
| Deny domains (exact) synced | PASS | sync-deny.example.com with [forseti-sync] |
| Deny domains (regex) synced | PASS | tracking regex with [forseti-sync] |
| Allow domains (exact) synced | PASS | sync-allow.example.com with [forseti-sync] |
| Local DNS synced | PASS | sync-nas.lan + sync-printer.lan |
| CNAME records synced | PASS | app.sync.lan + web.sync.lan |
| Clients synced | PASS | 10.0.0.0/8 with [forseti-sync] |

### Phase 7: Sync — CNAME Behavior

| Test | Status | Notes |
|------|--------|-------|
| FTL restart warning logged | PASS | "CNAME changes will trigger FTL restart" in Forseti logs |

### Phase 7: Sync — Recovery Tests

| Test | Status | Notes |
|------|--------|-------|
| Replica paused → beta degrades | PASS | target_health=1, alpha stays 0 |
| Replica unpaused → recovers | PASS | target_health back to 0 |
| Primary paused → snapshot fails | PASS | `failed=1 skipped=1` — replica skipped |
| Primary degraded log | PASS | `target degraded` with backoff timer |
| Primary unpaused → recovers | PASS | target_health back to 0, sync resumes |
| Normal sync after recovery | PASS | `ok=2` in cycle log |

---

## Bugs Found & Fixed During Run

| Bug | Fix | Commit |
|-----|-----|--------|
| GetSystemInfo parsing wrong: expected `system.cpu` as float, actual is `system.cpu.%cpu`; memory key is `%used` not `percent_used`; load is under `cpu.load.raw` | Updated response struct to match Pi-hole v6.7 | a9a432d |
