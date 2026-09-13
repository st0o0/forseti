## Why

All three subsystems (reconciler, collector, gravity) independently hit each Pi-hole's API without coordination. On resource-constrained targets like a Pi Zero, concurrent requests overwhelm FTL — the slow target goes dark first, then contention spills over to healthy targets. The collector has a hardcoded 10s scrape timeout too short for slow hardware, and the HTTP client timeout (30s) cannot be tuned per target. Result: permanent metric blackouts and cascading reconcile failures.

## What Changes

- Add per-target `api.timeout` and `api.max_concurrent` configuration fields
- Introduce a per-target semaphore ("gate") in the session pool that all subsystems must acquire before hitting the API
- Collector serves stale cached data when the target is busy instead of timing out with no data
- Replace hardcoded scrape timeout with per-target timeout contexts
- Fix data race in `TriggerGravity` where the shared `httpClient` field is swapped without synchronization
- Add `forseti_collector_cache_stale_total` metric for stale-on-busy events

## Capabilities

### New Capabilities

- `api-concurrency-gate`: Per-target semaphore controlling how many concurrent API calls each subsystem can make to a single Pi-hole instance. Enforces configurable `max_concurrent` limit across reconciler, collector, and gravity scheduler.

### Modified Capabilities

- `config`: Add `api` block to target config with `timeout` and `max_concurrent` fields
- `pihole-api`: Accept configurable timeout instead of hardcoded 30s; fix gravity `httpClient` swap race
- `pihole-stats-collection`: Collector uses `TryAcquire` (non-blocking) and serves stale cache when target is busy
- `collector-observability`: Add `forseti_collector_cache_stale_total` metric
- `target-worker`: Worker acquires gate before reconcile, releases after

## Impact

- **Config**: New optional `api` block on each target (backward-compatible, defaults to current behavior)
- **Session pool**: Gets semaphore map and `Acquire`/`TryAcquire`/`Release` methods
- **Worker interfaces**: `SessionManager` extended with gate methods
- **Collector**: Behavioral change — prefers stale data over no data when target is busy
- **Gravity scheduler**: Acquires gate before triggering (replaces internal-only `inFlight` guard)
- **Metrics endpoint**: Scrape timeout derived from target config instead of hardcoded 10s
