## Why

Settings reconciliation has two bugs that reinforce each other, causing permanent drift on production Pi-hole instances. Boolean settings like `dns.cache.force_on_disk` are sent as JSON `true`/`false` but the Pi-hole v6 API expects integers `0`/`1`, producing HTTP 400 errors. Because `ApplySettings` bails on the first error, all subsequent settings in the same cycle are never attempted. Additionally, the drift comparison uses `fmt.Sprintf("%v")` which mismatches Go types (`bool` vs `float64` from JSON unmarshal), causing phantom drift even when logical values agree.

## What Changes

- Add bool→int coercion for settings values sent to the Pi-hole API, converting `true`/`false` to `1`/`0` before JSON marshaling
- Change `ApplySettings` from bail-on-first-error to collect-and-continue, matching the pattern used by all other Apply functions
- Replace `fmt.Sprintf("%v")` comparison in `settingsNeedUpdate` with type-aware comparison that treats `bool`/`int`/`float64` equivalents as equal
- Add tests for partial-failure scenarios, realistic API response types, and bool coercion

## Capabilities

### New Capabilities

_(none)_

### Modified Capabilities

- `pihole-settings`: Settings field mapping scenarios must specify that boolean config values are sent as integers (`0`/`1`) to the Pi-hole API, not JSON booleans
- `reconciler`: Settings reconciliation must continue applying remaining settings when one fails, collecting errors rather than bailing

## Impact

- `internal/reconcile/settings.go` — `BuildDesiredSettingsList`, `settingsNeedUpdate`, `ApplySettings`
- `internal/reconcile/settings_test.go` — new tests for error handling, type coercion, type-aware comparison
- `internal/pihole/client.go` — possible coercion location in `PatchConfig`
- Existing specs `pihole-settings` and `reconciler` need delta specs reflecting the corrected behavior
