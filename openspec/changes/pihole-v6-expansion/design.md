## Context

Forseti manages Pi-hole gravity.db content declaratively. The Pi-hole v6 REST API exposes endpoints for allow-lists, top-N statistics, query history, and host system info that Forseti does not yet consume. The `feat/observability-overhaul` branch provides structured logging, a run-result model, and a reachability tracker. This change layers new data sources and allow-list management on top of that infrastructure.

Current collector pattern: toggle-gated parallel goroutines in `collectTarget()`, each calling a Pi-hole API method and feeding results into `metrics.Server.Update*()`. New collectors follow the same shape.

Current reconcile pattern: `PiholeAPI` interface defines CRUD methods. `diff*` functions produce `ResourceDiff` with Adds/Deletes/Updates. Apply processes diffs in fixed order. Allow-lists mirror block-lists structurally.

## Goals / Non-Goals

**Goals:**
- Expose Pi-hole top-domains, top-clients, query-history, and system-health as Prometheus metrics
- Manage allow-lists (`/api/lists?type=allow`) in config-mode reconcile and sync-mode replication
- Make `UpdateDomain` work for all type/kind combinations
- Add CNAME records to sync mode
- Sanitize group names to prevent injection

**Non-Goals:**
- Raw FTL log forwarding (needs a log-shipping design)
- Teleporter backup/restore integration
- DHCP lease management beyond read-only metrics
- TOTP/app-password authentication

## Decisions

### D1: Rebase onto observability-overhaul

Rebase `feat/pihole-v6-expansion` onto `feat/observability-overhaul` before starting implementation. The branch provides `slog`-based logging, `run.Result`, and `targetstate.Reachability` that new collectors should use. Zero feature overlap means minimal conflicts.

Alternative: merge both into main independently. Rejected because we would duplicate logging/reachability plumbing.

### D2: Allow-lists reuse the Adlist type

`config.Adlist` struct (URL + Comment + Groups) is identical for block and allow lists. Reuse it as `Allowlists []Adlist` in the config. The Pi-hole API uses the same `/api/lists` endpoint with `?type=allow` vs `?type=block`.

Alternative: new dedicated type. Rejected because the fields are identical and the API shape is the same.

### D3: Separate client methods per list type

Add `ListAllowlists`, `CreateAllowlist`, `UpdateAllowlist`, `DeleteAllowlists` as separate methods rather than parameterizing adlist methods with a `listType` argument. Keeps the `PiholeAPI` interface explicit and avoids a breaking change to existing adlist method signatures.

Alternative: generic `ListLists(listType)`. Rejected because it complicates the interface and every caller must pass the type — more error-prone.

### D4: Top-N metrics use configurable limit with GaugeVec

`forseti_top_queried_domains{target, domain}` as GaugeVec. The `count` parameter to `/api/stats/top_domains` defaults to 10, configurable per toggle config. Higher values increase label cardinality — the config should document the trade-off.

Alternative: histogram. Rejected because domain names are categorical, not numeric distributions.

### D5: System metrics as a separate toggle

`system` toggle gates `/api/info/system` calls. These expose host-level CPU/memory/temp which is useful but may conflict with node_exporter if already running. Keeping it opt-in avoids duplicate scraping.

### D6: History collector uses tick-based pattern

Like `NewDomainsCollector`, the history collector runs on its own interval rather than collect-on-scrape. History data is bucketed by Pi-hole (typically 10-minute windows) and polling it on every Prometheus scrape would be wasteful. A configurable poll interval (default: 5 minutes) feeds a ring buffer; the scrape reads the latest snapshot.

Alternative: collect-on-scrape. Rejected because history responses are larger and the data changes slowly.

### D7: UpdateDomain takes type and kind parameters

Change `UpdateDomain(domain, comment string, groups []int) error` to `UpdateDomain(domType, kind, domain, comment string, groups []int) error`. This is a breaking change to the `PiholeAPI` interface but only internal callers exist. All existing call sites pass deny/exact — update them in the same commit.

### D8: Group name sanitization at config parse time

Validate group names in `config.validate()` — reject names containing CR, LF, or null bytes. Fail-fast at config load rather than at API call time. This catches the issue for both config and sync modes.

### D9: CNAME sync follows DNS record pattern

CNAME records have no comment field and no group association — same as local DNS A/AAAA records. Use the same key-based add/delete pattern (no marker, purge-flag controlled). Add `cnames` to `validSyncResources` and `state`.

## Risks / Trade-offs

- **Label cardinality** (top_domains/top_clients): high `count` values create many time series. Mitigation: default to 10, document the risk, cap at 100.
- **Pi-hole API version drift**: new endpoints may change between v6.x releases. Mitigation: graceful 404 handling — if an endpoint returns 404, log a warning and skip the collector for that scrape cycle.
- **Rebase conflicts**: observability-overhaul touches worker, sync, collector. Mitigation: rebase early, resolve once.
- **System metrics overlap with node_exporter**: users running both get duplicate host metrics. Mitigation: opt-in toggle, document in README.
