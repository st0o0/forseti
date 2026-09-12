## Context

Settings reconciliation in `internal/reconcile/settings.go` maps Forseti YAML config values to Pi-hole v6 API paths and applies diffs. Two bugs cause permanent settings drift in production:

1. Boolean fields (Go `*bool`) are sent as JSON `true`/`false`, but Pi-hole v6 expects integers `0`/`1` for these fields. The API returns HTTP 400.
2. `ApplySettings` returns on the first `PatchConfig` error, skipping all remaining settings — unlike every other Apply function which collects errors and continues.
3. `settingsNeedUpdate()` uses `fmt.Sprintf("%v")` to compare values. After `json.Unmarshal`, Pi-hole returns integers as `float64`, so `"0" != "false"` even when logically equivalent.

## Goals / Non-Goals

**Goals:**
- Boolean config values are correctly sent as integers to the Pi-hole API
- A single failing setting does not block apply of remaining settings
- Drift comparison handles type differences between Go model and JSON-unmarshalled API responses
- Test coverage for error paths, type coercion, and partial failures

**Non-Goals:**
- Changing the YAML config format (users keep writing `true`/`false`)
- Validating every field against the Pi-hole API schema at startup
- Retry logic for failed settings (the watch loop already retries on next cycle)

## Decisions

### Decision 1: Coerce bools in `BuildDesiredSettingsList`, not in `PatchConfig`

The coercion (`true→1`, `false→0`) happens at the point where `SettingMapping.Value` is set, not downstream in the API client.

**Why over PatchConfig coercion:** `PatchConfig` is a generic API method used by other subsystems too. Adding bool→int logic there would be a leaky abstraction — not all Pi-hole API fields treat bools as ints. The settings mapping layer knows which fields are boolean and is the right place to own the type contract.

**Why over changing `*bool` to `*int` in config:** Users write `true`/`false` in YAML. Changing to `*int` would break the config format and require users to write `0`/`1`, which is less readable. The Go struct keeps `*bool` for ergonomic YAML; the mapping layer translates.

**Implementation:** In `BuildDesiredSettingsList`, wrap each `*bool` dereference with a helper `boolToInt(b bool) int` that returns `0` or `1`.

### Decision 2: Collect errors in `ApplySettings`, matching other Apply functions

Replace the immediate `return` on `PatchConfig` error with `errors = append(errors, ...)` and `continue`. Return `errors.Join(errs...)` after the loop.

**Why:** This matches the established pattern in `Apply()` for groups, adlists, domains, and clients (all use `report.Errors = append`). Consistency matters, and the watch loop already handles the error by logging and retrying next cycle.

### Decision 3: Type-aware comparison via `normalizeValue` helper

Replace `fmt.Sprintf("%v")` with a `normalizeValue(v any) any` function that canonicalizes values before comparison:
- `bool` → `int` (`true→1`, `false→0`)
- `float64` → `int` (when the float has no fractional part, e.g. `float64(0)` → `0`)
- Everything else passes through unchanged

Then compare with `fmt.Sprintf("%v")` on the normalized values, or use `reflect.DeepEqual`.

**Why not just `reflect.DeepEqual`:** Without normalization, `reflect.DeepEqual(false, float64(0))` is `false`. The normalization step is required regardless of the comparison method. Using `Sprintf` on normalized values keeps the existing pattern simple.

## Risks / Trade-offs

**[Risk] Other Pi-hole fields may expect booleans, not integers** → The Pi-hole v6 API is inconsistent about bool vs int. We coerce all `*bool` fields to int. If a future Pi-hole version changes a field to expect actual booleans, the `1`/`0` values would still work because JSON `1` is accepted where Pi-hole expects a boolean. Integers are the safer default.

**[Risk] `float64` normalization loses precision for large numbers** → Pi-hole config values are small integers (port numbers, cache sizes, thresholds). No Pi-hole setting uses floats. The truncation from `float64` to `int` is safe for this domain.

**[Risk] `errors.Join` changes the error format** → The watch loop logs the error from `ApplySettings`. With multiple errors joined, the log line becomes longer. This is acceptable — more information is better than silently skipping settings.
