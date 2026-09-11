## Why

Forseti currently manages only 7 of ~50+ Pi-hole v6 pihole.toml settings (DNS upstream, cache, rate limit, blocking mode, privacy level). Users who want full declarative control over their Pi-hole instances still need to manage DHCP, webserver, advanced DNS, and misc settings outside of Forseti. Additionally, all Prometheus metrics are always registered and collected — there is no way to disable unused metric groups — and the metric prefix is split between `pihole_*` and `forseti_*`, making the namespace inconsistent.

## What Changes

- **BREAKING**: Rename all `pihole_*` Prometheus metrics to `forseti_*` prefix for a unified namespace
- Expand the `settings` config section to cover all Pi-hole v6 pihole.toml areas: advanced DNS (DNSSEC, conditional forwarding, listening mode, etc.), blocking control (active, timer), DHCP, webserver, and misc
- Add `metrics.collectors` config section with per-area boolean toggles (default: all enabled) controlling both metric registration and API calls
- Add `forseti_settings_drift{target, setting}` gauge metric to detect manual Pi-hole config changes
- Add DHCP lease metrics (`forseti_dhcp_leases_active{target}`)
- Add `GetDHCPLeases()` to the Pi-hole API client

## Capabilities

### New Capabilities
- `collector-toggles`: Per-area boolean switches in `metrics.collectors` that control which metric groups are registered and which Pi-hole API calls are made during scrape
- `settings-drift-metrics`: Settings drift detection as a Prometheus gauge, comparing YAML desired state against actual Pi-hole config each reconcile cycle
- `dhcp-metrics`: DHCP lease count metric and Pi-hole DHCP API integration

### Modified Capabilities
- `pihole-settings`: Expand supported settings from 7 fields to full pihole.toml coverage (DNS, blocking, DHCP, webserver, privacy, misc)
- `pihole-prometheus-metrics`: Rename all `pihole_*` metrics to `forseti_*` prefix
- `metrics`: Add collector toggles config, settings drift metric, DHCP metrics
- `config`: Add `metrics.collectors` section with validation and defaults

## Impact

- **Breaking**: All `pihole_*` Prometheus metric names change to `forseti_*` — existing Grafana dashboards and alerts need updating
- **Config schema**: New fields in `settings` and `metrics.collectors` sections (additive, existing configs remain valid)
- **API client**: New methods for DHCP lease fetching
- **Collector**: Conditional metric registration and API call skipping based on collector toggles
- **Settings reconciler**: ~40 new field mappings in `buildDesiredSettings()`
- **Validation**: New validation rules for DHCP, webserver, misc settings
