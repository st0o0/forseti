## Why

Forseti covers ~60% of the Pi-hole v6 REST API. Pi-hole v6.7 exposes endpoints for top domains/clients, query history, system health, and allow-lists that Forseti does not use. Users cannot manage allow-lists declaratively, have no visibility into query-level or host-level metrics, and sync mode is missing CNAME support. Extending coverage now aligns with the observability-overhaul branch that provides structured logging and run-model infrastructure.

## What Changes

- Add four new toggle-gated metric collectors: `top_domains`, `top_clients`, `history`, `system`
- Add allow-list CRUD (`/api/lists?type=allow`) with full reconcile and sync support
- Parameterize `UpdateDomain` to support all type/kind combinations (currently hardcoded to deny/exact)
- Add CNAME records to sync mode (already supported in config-mode reconcile)
- Validate and sanitize group names to prevent CRLF injection (Pi-hole v6.7 CVE)

## Capabilities

### New Capabilities

- `allowlist-management`: Declarative allow-list management via YAML config, reconcile diff/apply, and primary-to-replica sync with group-ID translation
- `top-stats-collector`: Toggle-gated Prometheus metrics for top queried/blocked domains and top querying clients from Pi-hole stats endpoints
- `history-collector`: Toggle-gated Prometheus metrics for time-bucketed query activity from Pi-hole history endpoint
- `system-collector`: Toggle-gated Prometheus metrics for Pi-hole host health (CPU, memory, temperature, load averages)
- `group-name-validation`: Input sanitization for group names to prevent CRLF injection before sending to the Pi-hole API

### Modified Capabilities

- `pihole-api`: Add allow-list CRUD methods, parameterize `UpdateDomain` for all type/kind combos, add top-stats/history/system-info read methods
- `cname-reconcile`: Extend CNAME support from config-mode reconcile into sync mode
- `collector-toggles`: Register four new toggle keys (`top_domains`, `top_clients`, `history`, `system`)
- `metrics`: Define new `forseti_*` metric descriptors for top-stats, history, and system collectors

## Impact

- **Pi-hole client** (`internal/pihole/`): new API methods, `UpdateDomain` signature change (breaking for internal callers)
- **Config** (`internal/config/`): new `allowlists` YAML field, new toggle keys
- **Reconcile** (`internal/reconcile/`): new `diffAllowlists` step, updated `PiholeAPI` interface
- **Sync** (`internal/sync/`): new `allowlists` + `cnames` in state and sync loop
- **Collector** (`internal/collector/`): four new collector goroutines
- **Metrics** (`internal/metrics/`): new metric registrations
- **No external dependency changes** - all new endpoints are Pi-hole v6 REST API
