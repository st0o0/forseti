## Context

Settings reconciliation converts YAML config values into Pi-hole v6 API PATCH calls. The current implementation uses `boolToInt()` to coerce all boolean settings to integers before both comparison and apply. This was designed so `normalizeValue()` could compare API responses (which may return bools, ints, or float64s) against desired values using a single canonical form. However, Pi-hole v6's PATCH endpoint rejects integer values for boolean fields ("not of type bool"), so the coercion breaks apply while keeping comparison functional.

Additionally, string comparisons use `fmt.Sprintf("%v")` which is case-sensitive, but Pi-hole returns uppercased enum values (e.g., "LOCAL" vs user-written "local").

The drift metric is set before apply runs and never refreshed afterward, so failed applies leave stale drift=1 readings.

## Goals / Non-Goals

**Goals:**
- Send native Go `bool` values to Pi-hole PATCH API for all boolean settings
- Maintain type-aware comparison that handles bool↔int↔float64 equivalence from API responses
- Add case-insensitive comparison for string enum settings
- Refresh drift metric after successful apply
- Diagnose and fix container healthcheck failure

**Non-Goals:**
- Changing the YAML config format (users still write `true`/`false`)
- Adding new settings fields
- Refactoring the settings reconciliation architecture

## Decisions

### D1: Remove `boolToInt()`, send native bools

Remove `boolToInt()` entirely. `BuildDesiredSettingsList` will store actual `bool` values in `SettingMapping.Value`. The PATCH call (`api.PatchConfig`) will send the native Go `bool`, which `encoding/json` marshals to JSON `true`/`false`.

**Alternative considered**: Keep `boolToInt()` for comparison only and convert back to bool before PATCH. Rejected — adds complexity, the comparison layer should handle mixed types directly.

### D2: Update `normalizeValue()` for symmetric comparison

The comparison must handle these cases from Pi-hole API responses:
- API returns `true` (bool), desired is `true` (bool) → equal
- API returns `1` (float64 from JSON), desired is `true` (bool) → equal
- API returns `0` (float64 from JSON), desired is `false` (bool) → equal

Updated `normalizeValue()` strategy: normalize everything to a comparable string, but with special handling:
- `bool`: leave as-is (don't coerce to int)
- `float64` whole numbers: convert to `int`
- All else: leave as-is

Then update `settingsNeedUpdate()` to handle bool↔int equivalence explicitly:
- If one side is `bool` and the other is `int` (or `float64→int`): compare by truthiness (0=false, non-zero=true)
- If both sides are strings: use `strings.EqualFold`
- Otherwise: use `fmt.Sprintf("%v")` equality

### D3: Case-insensitive string comparison

In `settingsNeedUpdate()`, when both normalized values are strings, use `strings.EqualFold`. This fixes `listening_mode` ("local" vs "LOCAL") and any other enum fields Pi-hole uppercases.

### D4: Post-apply drift metric update

After `ApplySettings` succeeds (no error), re-run `DiffSettings` and update the drift metric with the post-apply state. If apply returns an error, keep the pre-apply drift reading (correct — things are still drifted).

```
DiffSettings()           → pre-apply drift
UpdateSettingsDrift()    → report pre-apply
ApplySettings()          → fix drifts
if apply succeeded:
  DiffSettings()         → post-apply drift
  UpdateSettingsDrift()  → report post-apply (should be all zeros)
```

### D5: Healthcheck — scratch image investigation

The Dockerfile healthcheck uses the forseti binary directly (`CMD ["/forseti", "healthcheck", ...]`), not wget/curl. The exec form JSON array should work on scratch without a shell. Likely runtime causes:
- Pi-hole not ready within start-period (15s may be tight)
- Missing `.env` file → empty passwords → Login() fails
- Network: `depends_on` doesn't wait for Pi-hole healthy, just started

Fix: increase `--start-period` to 30s and add `depends_on` with `condition: service_healthy` if Pi-hole containers have healthchecks. Document `.env` requirement.

## Risks / Trade-offs

- **[Breaking change for Pi-hole versions < v6]** → Mitigation: Forseti only supports Pi-hole v6, this is documented. The current int coercion was never correct for v6.
- **[API response format may vary across Pi-hole versions]** → Mitigation: `settingsNeedUpdate()` handles bool, int, float64, and string comparison generically. New types would need additions.
- **[Double DiffSettings call per cycle]** → Mitigation: Only runs when apply actually made changes. One extra GET /api/config call per cycle with changes — negligible overhead.
