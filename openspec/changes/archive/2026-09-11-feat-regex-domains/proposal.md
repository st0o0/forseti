## Why

Pi-hole v6 supports four domain list kinds: `exact` and `regex` for both `deny` and `allow`. Forseti currently hardcodes `"exact"` throughout the reconciler and only exposes a bare `domain` field in config. Users who need regex-based blocking or allowing (e.g., `(^|\.)ads\.example\.com$`) must configure those entries manually through the Pi-hole UI, where they risk being deleted by forseti during reconciliation.

## What Changes

- **Config model**: Add an optional `kind` field to `DenyEntry` and `AllowEntry` structs, defaulting to `"exact"`. Accepted values: `exact`, `regex`.
- **Validation**: Skip `validateDomain` (hostname pattern check) when `kind` is `regex`. Add regex compilation check instead to catch invalid patterns early.
- **Reconciler**: Pass each entry's `kind` through to `ListDomains` and `CreateDomain` API calls. Diff deny and allow domains per-kind so exact and regex entries are reconciled independently.
- **Sync engine**: Propagate `kind` from primary to replicas during domain sync.

## Capabilities

### New Capabilities

- `regex-domain-support`: Support for regex and exact domain kinds in deny/allow lists with per-kind reconciliation

### Modified Capabilities

_(none — no existing specs)_

## Impact

- `internal/config/config.go`: `DenyEntry`, `AllowEntry` structs gain `Kind` field; `validate()` updated
- `internal/reconcile/reconciler.go`: `diffDomains`, `diffAllowDomains`, `Plan()`, `Apply()` updated for per-kind handling
- `internal/sync/syncer.go`: domain sync passes `kind` through
- `internal/config/config_test.go`, `internal/reconcile/reconciler_test.go`: new test cases
