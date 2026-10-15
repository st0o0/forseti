## Context

Forseti currently manages 7 of ~50+ Pi-hole v6 pihole.toml settings and exposes metrics under a split `pihole_*` / `forseti_*` namespace with no way to disable unused metric groups. The settings reconciler (`internal/reconcile/settings.go`) uses a `SettingMapping` pattern that maps Forseti YAML paths to Pi-hole API paths — this pattern scales well to additional fields. The collector (`internal/collector/collector.go`) fetches stats, blocking status, and upstreams in parallel per target but always registers all metrics unconditionally.

## Goals / Non-Goals

**Goals:**
- Full pihole.toml coverage: DNS (advanced), blocking, DHCP, webserver, privacy, misc
- Unified `forseti_*` metric namespace
- Per-area collector toggles controlling both metric registration and API calls
- Settings drift detection as a Prometheus gauge
- DHCP lease count metric

**Non-Goals:**
- Per-target collector overrides (all targets share the same collector config)
- Real-time DHCP lease event streaming
- Managing Pi-hole debug flags via settings (low value, high risk)
- Backwards-compatible `pihole_*` metric aliases

## Decisions

### 1. Extend SettingMapping table, not a generic pass-through

**Decision**: Add ~40 new entries to `buildDesiredSettings()` following the existing `SettingMapping` pattern. Each field is explicitly declared with its Forseti path, Pi-hole API path, and value.

**Alternative considered**: Generic pass-through that forwards arbitrary YAML keys to `/api/config/*` paths. Rejected because it removes validation at config-load time — typos would silently fail at apply time instead of failing fast.

### 2. Nested config structs per area

**Decision**: Add new Go structs for each settings area (`DHCPSettings`, `WebserverSettings`, `MiscSettings`, `RevServerSettings`) following the existing pattern of `DNSSettings`, `BlockingSettings`, `PrivacySettings`. All use pointer fields for optional values so `IsEmpty()` can distinguish "not set" from zero values.

**Alternative considered**: Single flat map. Rejected because it loses type safety and makes validation harder.

### 3. Collector toggles as a config struct with bool pointers

**Decision**: Add `CollectorToggles` struct to `Metrics` config. Each field is a `*bool` — nil means default (true). At metric server construction time, check toggles before registering each metric group. At collection time, skip API calls for disabled groups.

```
metrics:
  collectors:
    stats: true
    upstreams: true
    query_types: true
    blocking: true
    reconcile: true
    gravity: true
    sessions: true
    settings_drift: true
    dhcp: true
```

**Alternative considered**: String list of enabled collectors. Rejected because booleans are simpler for opt-out (user only needs to set `false` on what they don't want).

### 4. Conditional metric registration

**Decision**: When a collector toggle is false, the corresponding `prometheus.Collector` objects are never created or registered. The `Server` struct fields become nil, and all `Record*`/`Update*` methods check for nil before writing. This ensures disabled collectors produce zero overhead — no stale metrics, no API calls.

**Alternative considered**: Register all metrics but skip populating them. Rejected because it leaves zero-value metrics in `/metrics` output which confuse dashboards.

### 5. Settings drift as a reconcile-time gauge

**Decision**: Add `forseti_settings_drift{target, setting}` gauge. Updated during each reconcile cycle by calling `DiffSettings()` and setting 1 for each changed setting, 0 for each matching setting. Only declared settings are tracked.

**Alternative considered**: Collect drift at scrape time (in the collector). Rejected because it would add a `GET /api/config` call to every scrape, and drift detection is logically part of reconciliation.

### 6. DHCP lease metric via new API method

**Decision**: Add `GetDHCPLeases() ([]APIDHCPLease, error)` to the Pi-hole client, calling `GET /api/dhcp/leases`. Expose `forseti_dhcp_leases_active{target}` as a gauge set to the count of active leases. Collected alongside stats in the collector when the `dhcp` toggle is enabled.

### 7. Metric rename — clean break, no aliases

**Decision**: Rename all `pihole_*` metrics to `forseti_*` in a single release. Document the rename in release notes with a mapping table. No dual-emission or alias period.

**Rationale**: Dual-emission doubles cardinality and complicates the code. Users can update Grafana dashboards with a find-replace. This is a pre-1.0 project.

## Risks / Trade-offs

- **Breaking metric names** → Mitigated by documenting the full rename mapping in release notes and providing a sed/jq one-liner for dashboard migration
- **Large settings surface area** → Mitigated by the explicit mapping pattern — each field is individually validated and tested. Missing a field is a feature gap, not a bug
- **DHCP API availability** → Pi-hole only exposes `/api/dhcp/leases` when DHCP is enabled. If disabled, the endpoint returns empty or errors. Mitigated by graceful handling in the collector (log warning, skip metric update)
- **Nil-check overhead in metrics methods** → Minimal runtime cost. The alternative (separate Server implementations per toggle configuration) is over-engineered

## Open Questions

- Should `settings_drift` count as a collector toggle or always be active when settings are declared? Current design: toggle-controlled.
