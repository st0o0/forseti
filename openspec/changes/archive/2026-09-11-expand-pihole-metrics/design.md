## Context

Forseti's metrics server registers 11 pihole_* gauges but only populates 5 from `/api/stats/summary`. The Stats struct parses only `queries.total`, `queries.blocked`, `queries.percent_blocked`, `gravity.domains_being_blocked`, and `gravity.last_update`. The Pi-hole v6 summary endpoint returns significantly more data (forwarded, cached, unique_domains, frequency, clients, query types, query status, reply types) that is currently ignored.

Additionally, `/api/stats/upstreams` provides per-upstream resolver query counts and response time statistics that no part of Forseti consumes today.

The existing metrics spec already references `pihole_reply_type_total` and `pihole_upstream_queries_total` but they were never implemented.

## Goals / Non-Goals

**Goals:**
- Parse all available fields from `/api/stats/summary` into the Stats struct
- Add a new API call to `/api/stats/upstreams` with its own response struct
- Register and populate all new Prometheus metrics
- Remove or repurpose dead metrics whose data source doesn't exist in the v6 API
- Achieve metric parity with eko/pihole-exporter and bazmonk/pihole6_exporter

**Non-Goals:**
- Top-N metrics (top queries, top blocked, top clients) — high cardinality, Prometheus anti-pattern
- Per-minute windowed metrics (bazmonk's `_1m` series) — adds complexity, limited value with Prometheus rate()
- Grafana dashboard creation — separate concern
- Changes to reconcile or config metrics

## Decisions

### 1. Expand Stats struct vs. separate types

**Decision**: Expand the existing `Stats` struct with new fields and extend `UnmarshalJSON`.

**Why**: The data comes from a single API call (`/api/stats/summary`). A single struct keeps the client interface simple — one call, one return value. The maps for types/status/replies use `map[string]int` which is flexible enough for any keys Pi-hole returns.

**Alternative**: Separate structs per section (QueryStats, ClientStats, etc.) — rejected because it fragments a single API response unnecessarily.

### 2. New GetUpstreams method + UpstreamStats struct

**Decision**: Add a dedicated `GetUpstreams() ([]UpstreamStats, error)` method calling `/api/stats/upstreams`.

**Why**: This is a separate API endpoint with a different shape. Mixing it into Stats would conflate two API calls. The caller decides whether to make the extra call (it's optional — summary metrics work without it).

### 3. Dead metrics: cache_size, cache_inserted, cache_evicted, ftl_memory_bytes

**Decision**: Remove `pihole_cache_size`, `pihole_cache_inserted`, `pihole_cache_evicted`, and `pihole_ftl_memory_bytes`. These fields are not available in the Pi-hole v6 `/api/stats/summary` response.

**Why**: Keeping registered-but-never-populated metrics is misleading. If a future Pi-hole version exposes this data, we can add them back with the correct source.

**Alternative**: Call `/api/info/ftl` or `/api/info/system` for FTL memory — rejected for now because those endpoints aren't documented for this purpose and would add API calls for marginal value.

### 4. Blocking status via /api/dns/blocking

**Decision**: Populate `pihole_status` by calling `GET /api/dns/blocking` which returns `{"blocking": true/false}`. Add a `GetBlockingStatus() (bool, error)` method.

**Why**: The enabled/disabled status is operationally important (alerting on accidentally disabled Pi-hole). The endpoint is lightweight and stable in v6.

### 5. GaugeVec label design for typed metrics

**Decision**: Use consistent label naming:
- `pihole_query_types{target, query_type}` — DNS record type (A, AAAA, PTR, etc.)
- `pihole_query_status{target, status}` — query resolution status (GRAVITY, FORWARDED, CACHE, etc.)
- `pihole_reply_types{target, reply_type}` — reply type (CNAME, IP, NXDOMAIN, etc.)
- `pihole_upstream_queries_total{target, upstream, name, port}` — per-upstream

**Why**: Explicit label names avoid collision (e.g., `type` is ambiguous). The `target` label is consistent with all existing pihole_* metrics for multi-instance support.

### 6. Stale label cleanup for dynamic metrics

**Decision**: Reset GaugeVec metrics before each update cycle using `Reset()` on the vectors that have dynamic label values (query_types, query_status, reply_types, upstreams).

**Why**: If a query type or upstream disappears between scrapes, the old label combination would persist with a stale value. Reset + re-set ensures only current values are exposed.

### 7. gravity_size vs. domains_blocked

**Decision**: Remove `pihole_gravity_size` (never populated) and keep `pihole_domains_blocked` (already populated from `gravity.domains_being_blocked`). These were redundant — both intended to represent "domains on blocklist".

**Why**: One metric for this concept is enough. `pihole_domains_blocked` is already live and named consistently with other exporters' `pihole_domains_being_blocked`.

## Risks / Trade-offs

- **[Cardinality from typed metrics]** → Query types, status, and reply types add ~40 label combinations per target. This is well within Prometheus norms and far less than top-N metrics would add.
- **[Extra API call for upstreams]** → One additional HTTP request per scrape interval. Mitigated by the configurable scrape_interval. The call is lightweight.
- **[Breaking metric removal]** → Removing `pihole_gravity_size`, `pihole_cache_*`, `pihole_ftl_memory_bytes` could break dashboards referencing them. Mitigated by the fact that they were never populated (always 0), so no dashboard should depend on them.
- **[Pi-hole API stability]** → The v6 summary response shape could change. Mitigated by defensive parsing with `json:"..."` tags and zero-value defaults for missing fields.
