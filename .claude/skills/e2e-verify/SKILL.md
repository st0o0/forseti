---
description: "Run the full E2E test plan against the dev stack. Tests both config mode and sync mode. Handles Docker teardown, startup, seeding, unit tests, and Docker E2E verification. Use when the user wants to verify all features, run E2E tests, or do a final verification pass."
---

# E2E Verification

Execute the complete E2E test plan from `e2e.md` against the dev stack.
Tests both **config mode** (YAML-driven reconcile) and **sync mode** (primary→replica sync).

**Every test must be PASS or FAIL. No SKIP allowed.** If a test requires specific state,
create that state before running the test.

## How to run

1. Read `e2e.md` fully before starting
2. Execute Phase 0 (config mode setup)
3. Run Phase 1-5 (config mode tests)
4. Execute Phase 6 (sync mode setup - tear down, seed primary, start sync)
5. Run Phase 7-8 (sync mode tests)
6. Write results to `E2E-TEST-RESULTS-<DATE>.md`

## Architecture

```
Main Agent (Opus):
  === CONFIG MODE ===
  Phase 0: Docker teardown + rebuild (config mode)
  Phase 1: Unit tests (go test with coverage)
  Phase 2: Metrics endpoint verification
  Phase 3: Reconcile behavior verification (groups, adlists, allowlists, domains, DNS, clients)

  --> Spawn parallel subagents for independent checks:
  Agent "api-collectors" (Haiku):   New collector toggles (top_domains, top_clients, system, history)
  Agent "api-config-extra" (Haiku): Config hot-reload, allowlist lifecycle, gravity triggers
  Main agent concurrently:          Health/liveness, recovery test (pause/unpause)

  === SYNC MODE ===
  Phase 6: Docker teardown, seed primary, start sync mode
  Phase 7: Sync verification (all resource types on replica)

  --> Spawn parallel subagents:
  Agent "sync-verify" (Haiku):      Verify all synced resources on replica
  Agent "sync-markers" (Haiku):     Verify markers, manual entry preservation, orphan cleanup
  Main agent concurrently:          Sync recovery (pause/unpause primary + replica)

  Phase 8: Collect all results, write file
```

## Phase 0: Config Mode Setup (Clean Start)

```powershell
docker compose -f docker-compose.dev.yml --env-file .env.dev down -v
docker compose -f docker-compose.dev.yml --env-file .env.dev up -d --build
```

Wait for all services: forseti (9099), pihole-alpha (18080), pihole-beta (18081).
Wait for at least one successful reconcile (grep metrics for `forseti_reconcile_runs_total`).

## Phase 1: Unit Tests

```powershell
go test ./... -timeout 120s
go test ./internal/pihole/ ./internal/reconcile/ ./internal/collector/ ./internal/sync/ ./internal/config/ -cover -timeout 60s
```

## Phase 2: Metrics Verification

Check all metrics from the "Metrics verification checklist" in e2e.md:
- Always present (build_info, target_reachable, config_*, collector_*)
- Stats (dns_queries, blocked, domains_blocked, gravity_last_update, ...)
- Blocking, DHCP, Reconcile
- New: System info (cpu, memory, load) - toggle must be enabled in dev config
- New: History (query_history_total/blocked) - tick-based, wait for first tick
- New: Top stats (top_queried_domains, top_clients) - may be empty if no DNS traffic

## Phase 3: Config Mode Reconcile Verification

Authenticate to Pi-hole alpha (port 18080, password from `.env.dev`).
Check each resource type per the "Reconcile verification checklist" in e2e.md:
- Groups (with [forseti] marker)
- Adlists (block, with marker, StevenBlack present)
- Allowlists (with marker, anudeepND whitelist present)
- Deny/Allow domains (exact entries with marker)
- Local DNS records
- Clients (with marker)

## Phase 4: Config Mode Parallel Verification

Spawn subagents for independent work:

### Agent "api-collectors" (Haiku)
Verify new toggle-gated metrics exist and have plausible values.
Pure GET on `/metrics`. Check system metrics have values 0-100, load >= 0, etc.

### Agent "api-config-extra" (Haiku)
Test config-mode extra scenarios from e2e.md:
- Allowlist lifecycle (present after reconcile)
- Gravity trigger verification (check logs for gravity after adlist/allowlist changes)

### Main agent concurrent
- Health endpoints: `/healthz` (200), `/status` (JSON with targets), `/status/history`
- Recovery: `docker pause forseti-pihole-alpha`, verify degradation, unpause, verify recovery

## Phase 5: Collect Config Mode Results

Wait for subagents, validate FAIL results, record all config-mode results.

## Phase 6: Sync Mode Setup

```powershell
# Tear down everything
docker compose -f docker-compose.dev.yml --env-file .env.dev down -v

# Start Pi-holes only (no Forseti)
docker compose -f docker-compose.dev.yml --env-file .env.dev up -d pihole-alpha pihole-beta

# Wait for healthy
# Poll http://localhost:18080/admin/ and http://localhost:18081/admin/

# Seed primary with test data
bash e2e-seed-primary.sh

# Start Forseti in sync mode (uses docker-compose.sync.yml override)
docker compose -f docker-compose.dev.yml -f docker-compose.sync.yml --env-file .env.dev up -d --build forseti

# Wait for first successful sync cycle
# Poll metrics for forseti_reconcile_runs_total with status="success"
```

## Phase 7: Sync Mode Verification

Authenticate to both Pi-holes. Check every resource type was synced:

### Verify on replica (beta, port 18081)

For each resource type, verify:
1. **Present**: the resource exists on the replica
2. **Marker**: `[forseti-sync]` in the comment field (where applicable)
3. **Content match**: same data as on the primary

Resource checklist from e2e.md "Sync Mode E2E Scenarios":
- Groups (e2e-sync-group with marker)
- Adlists/block (StevenBlack with marker)
- Allowlists (anudeepND with marker)
- Deny domains exact + regex (with marker)
- Allow domains exact (with marker)
- Local DNS (no marker - key-based)
- CNAME records (no marker - key-based, check FTL restart log)
- Clients (with marker)

### Spawn parallel subagents

#### Agent "sync-verify" (Haiku)
Verify all resource types on replica via API. Pure GET requests. Report PASS/FAIL per resource.

#### Agent "sync-markers" (Haiku)
Test marker isolation and orphan cleanup:
- Add manual entry on beta (no marker) → verify it survives sync
- Check that Default group (id=0) is untouched

#### Main agent concurrent: Sync recovery
- Pause replica → verify sync logs error for beta, alpha still succeeds
- Unpause replica → verify recovery
- Pause primary → verify all replicas skipped
- Unpause primary → verify normal sync resumes

## Phase 8: Write Results

Collect all results from both modes. Write to `E2E-TEST-RESULTS-<DATE>.md`:

```markdown
# E2E Test Results - <DATE>

**Stack**: Forseti v<version>, Pi-hole v<version> x2
**Branch**: <branch>

## Summary

| Phase | Pass | Fail | Total |
|-------|------|------|-------|
| Unit Tests | X | Y | N |
| Config: Metrics | ... | ... | ... |
| Config: Reconcile | ... | ... | ... |
| Config: Collectors | ... | ... | ... |
| Config: Health/Recovery | ... | ... | ... |
| Sync: Resources | ... | ... | ... |
| Sync: Markers | ... | ... | ... |
| Sync: Recovery | ... | ... | ... |
| **Total** | **X** | **Y** | **N** |
```

## Test Methods

### Metrics checks
```powershell
$metrics = Invoke-RestMethod "http://localhost:9099/metrics"
if ($metrics -match 'metric_name') { "PASS" } else { "FAIL" }
```

### Pi-hole API checks
Always authenticate first with POST to `/api/auth`, then use `X-FTL-SID` header.

### Docker operations
```powershell
docker pause forseti-pihole-alpha     # simulate outage
docker unpause forseti-pihole-alpha   # restore
docker logs forseti --tail 50         # check logs
```

### Seeding primary
```bash
bash e2e-seed-primary.sh              # populates alpha with test data
```

## Important

- Don't modify `e2e.md` - it's the reusable spec
- Write results to a separate file
- Follow the execution order exactly - config mode first, then sync mode
- Config mode and sync mode use different Docker Compose overrides
- Sync mode requires seeding the primary BEFORE starting Forseti
- If a setup step fails, mark dependent tests as FAIL too
- Launch subagents in ONE message for parallelism
- Use `model: "haiku"` for subagents
- Tear down the stack at the end
