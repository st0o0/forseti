## Why

Settings reconciliation has 3 confirmed bugs causing persistent drift on every reconcile cycle. Boolean values are sent as integers (per current spec), but Pi-hole v6's PATCH API rejects non-bool types with "not of type bool". String enum comparisons are case-sensitive while Pi-hole returns uppercased values. Together these cause perpetual apply failures and false drift reports. A fourth issue — unhealthy container healthcheck — needs investigation.

## What Changes

- **BREAKING**: Boolean settings sent as native `bool` instead of `int` (0/1) to match Pi-hole v6 API expectations
- Remove `boolToInt()` coercion from `BuildDesiredSettingsList` — all 13 boolean fields affected
- Update `normalizeValue()` to handle bool↔int comparison for API responses that return either form
- Add case-insensitive comparison for string enum settings (`listening_mode`, `blocking.mode`) in `settingsNeedUpdate`
- Update drift metric after successful apply to reflect true post-apply state
- Investigate and fix container healthcheck failure

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `pihole-settings`: Boolean settings must be sent as native bools, not integers. The current spec (line 69) mandates int coercion, which Pi-hole v6 rejects. Type-aware comparison must also handle case-insensitive string matching for enum fields.
- `settings-drift-metrics`: Drift metric should reflect post-apply state, not just pre-apply diff. Current spec says "cleared on next reconcile cycle" but if apply fails silently due to type errors, drift is never cleared.

## Impact

- `internal/reconcile/settings.go` — `boolToInt()`, `BuildDesiredSettingsList`, `normalizeValue`, `settingsNeedUpdate`
- `cmd/forseti/main.go` — drift metric update after apply
- `Dockerfile` / `docker-compose*.yml` — healthcheck investigation
- Existing settings reconciliation tests need updating for bool values
