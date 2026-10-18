## 1. Branch Setup

- [x] 1.1 Rebase `feat/pihole-v6-expansion` onto `origin/feat/observability-overhaul`
- [x] 1.2 Verify build and tests pass on the rebased branch (`go test -race ./...`, `go vet ./...`)

## 2. Pi-hole Client: Allow-List API

- [x] 2.1 Add API response/request types for allow-lists (reuse `APIList` with `type=allow`)
- [x] 2.2 Implement `ListAllowlists()` - `GET /api/lists?type=allow`
- [x] 2.3 Implement `CreateAllowlist()` - `POST /api/lists?type=allow`
- [x] 2.4 Implement `UpdateAllowlist()` - `PUT /api/lists/{address}?type=allow`
- [x] 2.5 Implement `DeleteAllowlists()` - `POST /api/lists:batchDelete?type=allow` with DELETE fallback
- [x] 2.6 Add unit tests for allow-list client methods

## 3. Pi-hole Client: New Stats & System Endpoints

- [x] 3.1 Add response types for top domains, top clients, history, and system info
- [x] 3.2 Implement `GetTopDomains(count)` - `GET /api/stats/top_domains?count={n}`
- [x] 3.3 Implement `GetTopClients(count)` - `GET /api/stats/top_clients?count={n}`
- [x] 3.4 Implement `GetHistory()` - `GET /api/history`
- [x] 3.5 Implement `GetSystemInfo()` - `GET /api/info/system`
- [x] 3.6 Add unit tests for new stats/system client methods

## 4. Pi-hole Client: UpdateDomain Parameterization

- [x] 4.1 Change `UpdateDomain` signature to accept `domType` and `kind` parameters
- [x] 4.2 Update `PiholeAPI` interface in `internal/reconcile/reconciler.go`
- [x] 4.3 Update all existing call sites to pass `"deny"`, `"exact"` explicitly
- [x] 4.4 Add unit tests for UpdateDomain with different type/kind combinations

## 5. Config: Allowlists & New Toggles

- [x] 5.1 Add `Allowlists []Adlist` field to `Config` struct with `yaml:"allowlists"`
- [x] 5.2 Add validation for allowlist duplicate URLs
- [x] 5.3 Add `top_domains`, `top_clients`, `history`, `system` to `CollectorToggles` (default disabled)
- [x] 5.4 Add configurable `count` for top_domains and top_clients (default 10, max 100)
- [x] 5.5 Add configurable `history_interval` for history collector (default 5m, min 1m)
- [x] 5.6 Add group name validation: reject CR, LF, null bytes in `validate()`
- [x] 5.7 Add unit tests for new config fields and validation rules

## 6. Reconcile: Allow-Lists

- [x] 6.1 Add `ListAllowlists`, `CreateAllowlist`, `UpdateAllowlist`, `DeleteAllowlists` to `PiholeAPI` interface
- [x] 6.2 Implement `diffAllowlists` function mirroring `diffAdlists` with `[forseti]` marker
- [x] 6.3 Add `Allowlists ResourceDiff` field to `DiffReport`
- [x] 6.4 Wire allowlist reconcile into `Apply()` after adlists, before deny domains
- [x] 6.5 Set `NeedsGravity` on allowlist changes
- [x] 6.6 Include allowlists in `Plan()` output
- [x] 6.7 Add unit tests for allowlist reconciliation (add, update, delete, manual preserved)

## 7. Sync: Allow-Lists & CNAME

- [x] 7.1 Add `allowlists` field to sync `state` struct
- [x] 7.2 Add `allowlists` to `validSyncResources` list
- [x] 7.3 Implement `syncAllowlists` with group-ID translation and `[forseti-sync]` marker
- [x] 7.4 Add `cnames` field to sync `state` struct
- [x] 7.5 Add `cnames` to `validSyncResources` list
- [x] 7.6 Implement `syncCNAMEs` following local DNS add/delete pattern with purge flag
- [x] 7.7 Wire CNAME restart trigger for sync-mode CNAME changes
- [x] 7.8 Add group name validation for names received from primary (warn and skip invalid)
- [x] 7.9 Add unit tests for allowlist sync and CNAME sync

## 8. Metrics: New Metric Registrations

- [x] 8.1 Add `forseti_top_queried_domains` and `forseti_top_blocked_domains` GaugeVec (target, domain)
- [x] 8.2 Add `forseti_top_clients` GaugeVec (target, client)
- [x] 8.3 Add `forseti_query_history_total` and `forseti_query_history_blocked` GaugeVec (target, bucket)
- [x] 8.4 Add system gauges: `forseti_pihole_cpu_percent`, `_memory_percent`, `_temperature_celsius`, `_load_1m/5m/15m`
- [x] 8.5 Add `Update*` methods with nil-guards for each new metric group
- [x] 8.6 Wire toggle-gated registration in `NewServer()` for all four new toggle groups
- [x] 8.7 Handle stale label cleanup via Reset before re-population for top-N metrics

## 9. Collector: New Collection Goroutines

- [x] 9.1 Add `topDomains` goroutine in `collectTarget()` gated by `top_domains` toggle
- [x] 9.2 Add `topClients` goroutine in `collectTarget()` gated by `top_clients` toggle
- [x] 9.3 Add `systemInfo` goroutine in `collectTarget()` gated by `system` toggle
- [x] 9.4 Implement `HistoryCollector` as tick-based collector (like `NewDomainsCollector`)
- [x] 9.5 Add graceful 404 handling: log warning, skip collector, no error propagation
- [x] 9.6 Add history collector to watcher startup/shutdown lifecycle
- [x] 9.7 Add unit tests for new collector goroutines

## 10. Integration & Docs

- [x] 10.1 Run full test suite with race detector (`go test -race ./...`)
- [x] 10.2 Run linter (`golangci-lint run`)
- [x] 10.3 Update example config YAML with new `allowlists` and toggle fields
- [x] 10.4 Update README with new metrics table and allowlist documentation
- [x] 10.5 Docker build verification (`docker build -t forseti:ci .`)
