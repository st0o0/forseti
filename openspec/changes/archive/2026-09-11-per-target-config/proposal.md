## Why

Forseti currently applies identical gravity.db content (groups, adlists, deny/allow lists, clients, local DNS) to every target. There is no way to vary configuration per Pi-hole instance. Real-world setups need per-target differences — a low-RAM Pi-hole needs `force_on_disk`, a kids Pi-hole needs extra deny lists and parental adlists, an office Pi-hole may need different upstream DNS. Additionally, Pi-hole FTL settings (`pihole.toml`) are entirely outside Forseti's scope today, forcing operators to manage them separately via Docker env vars or manual edits.

## What Changes

- **New `settings:` section** in forseti.yaml — a curated, Forseti-native format for Pi-hole FTL settings (dns, blocking, privacy). Reconciled via Pi-hole v6's `/api/config` endpoint. Not a 1:1 mirror of pihole.toml; a deliberate subset of operationally relevant settings.
- **New `file:` field on targets** — optional path to a per-target YAML override file. The override file uses the same structure as the global config (settings, groups, adlists, deny, allow, local_dns, cname, clients) plus an `exclude:` block.
- **Merge engine** — computes effective config per target: global defaults + target appends − target excludes. Settings use deep merge (scalar overwrites scalar). Content lists use append. Exclude removes by key (domain, url, match).
- **Settings reconciliation** — new reconcile step that diffs desired settings against actual Pi-hole config and applies changes via the config API.
- **Backwards compatible** — existing forseti.yaml without `settings:` or `file:` works unchanged.

## Capabilities

### New Capabilities
- `per-target-overrides`: Target-level config file loading, merge engine (append + exclude), and effective config computation
- `pihole-settings`: Forseti-native settings format, mapping to Pi-hole v6 config API, settings reconciliation (read/diff/apply)

### Modified Capabilities
- `config`: Adds `settings:` top-level field and `file:` field on targets; extends validation for both
- `reconciler`: Adds settings reconciliation step before content reconciliation

## Impact

- `internal/config` — new types (Settings, TargetOverride), file loading, merge logic, extended validation
- `internal/pihole` — new methods: `GetConfig()`, `PatchConfig()` for reading/writing pihole.toml via API
- `internal/reconcile` — new `reconcileSettings()` step in Plan/Apply
- `cmd/forseti` — plan/apply output includes settings diff
- Config validation must handle cross-file references and detect conflicts (e.g., exclude referencing non-existent global entry)
