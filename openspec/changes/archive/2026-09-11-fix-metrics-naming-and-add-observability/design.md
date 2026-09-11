## Context

Forseti exposes ~30 Prometheus metrics across two namespaces: `forseti_*` (controller telemetry) and `pihole_*` (Pi-hole stats). All metrics are defined and registered in `internal/metrics/metrics.go`, with stats collection in `internal/collector/collector.go` and session management in `internal/session/pool.go`.

Currently, 8 Gauge metrics use the `_total` suffix (a Prometheus Counter convention), the Collector subsystem has no self-instrumentation, and session reauths happen silently in the Pi-hole client.

## Goals / Non-Goals

**Goals:**
- Fix all Prometheus naming convention violations
- Make Collector performance and caching observable
- Make session stability (reauth frequency) observable
- Add standard `_build_info` exporter metric

**Non-Goals:**
- Changing metric types (all current types are correct)
- Adding Pi-hole version info metrics (requires extra API endpoint, high cardinality)
- Separate sync-mode metrics (sync already uses RecordReconcile)
- Grafana dashboard updates (external, documented as breaking)

## Decisions

### 1. Rename in place, no compatibility shim

Rename all violating metrics directly. No dual-emission period, no deprecated aliases.

**Why**: Forseti is pre-1.0, the user base is small, and maintaining two metric names adds complexity for no real benefit. A clean break with documented migration is simpler.

**Alternative considered**: Emitting both old and new names for one release cycle. Rejected because it doubles the time series count and the `_total` Gauge names are actively harmful (PromQL linters, accidental `rate()` usage).

### 2. OnReauth callback pattern for session metrics

Add an `OnReauth func()` field to `pihole.Client`. The session pool sets this callback when creating clients, wiring it to the metrics counter. The client calls it in `doJSON` after a successful 401 re-authentication.

**Why**: The reauth logic lives in `Client.doJSON` (line ~503), not in the Pool. The Pool doesn't know when reauths happen. A callback keeps the client decoupled from the metrics package — the client doesn't import metrics, it just calls a function.

**Alternative considered**: Having the Pool wrap every client call to detect reauths. Rejected because it would require proxying the entire Client interface. Also considered: returning reauth count from API calls. Rejected because it changes every call signature.

### 3. Collector metrics on the metrics.Server, instrumented in collector.go

The three new Collector metrics (`duration`, `fetches`, `cache_hits`) are registered on `metrics.Server` like all other metrics. The Collector calls recording methods on Server after each Collect() cycle.

**Why**: Consistent with the existing pattern — all metrics live on Server, subsystems call Server methods to record. Keeps the Collector free of direct prometheus imports.

**Alternative considered**: Self-contained prometheus.Collector interface on the Collector struct. Rejected because it breaks the established pattern and splits metric registration across packages.

### 4. Build info set at startup via Server method

`Server.SetBuildInfo(version, mode string)` sets `forseti_build_info` once. Called from `cmd/forseti` after config is loaded.

**Why**: Version comes from linker flags (`-ldflags`), mode from config. Both are known at startup. The metric never changes during runtime.

## Risks / Trade-offs

**[Breaking metric names]** → Documented in proposal. Users MUST update Grafana dashboards and alert rules. Migration table provided in tasks.

**[OnReauth callback adds a field to Client]** → Minimal risk. The field is optional (nil check before call). No change to Client's public API surface beyond the new field.

**[Collector duration includes cache-hit fast path]** → The histogram will show bimodal distribution: fast (all cached) vs slow (API fetches). This is actually useful — it reveals whether scrape_interval and TTL are well-tuned.

## Migration Plan

1. Ship all renames + new metrics in a single release
2. Document the rename mapping in CHANGELOG (release-please)
3. Provide a sed/awk one-liner for updating Grafana dashboard JSON exports

## Open Questions

None — all decisions are resolved.
