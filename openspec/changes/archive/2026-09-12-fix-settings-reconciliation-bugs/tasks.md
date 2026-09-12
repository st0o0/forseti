## 1. Remove boolToInt coercion

- [x] 1.1 Delete `boolToInt()` function from `internal/reconcile/settings.go`
- [x] 1.2 Update all 13 boolean fields in `BuildDesiredSettingsList` to pass native `bool` values instead of `boolToInt()` calls
- [x] 1.3 Update `normalizeValue()` to stop converting `bool` → `int`; leave bools as-is

## 2. Type-aware comparison

- [x] 2.1 Rewrite `settingsNeedUpdate()` to handle bool↔int equivalence (true==1, false==0) and bool↔float64 equivalence
- [x] 2.2 Add case-insensitive comparison for string values using `strings.EqualFold`

## 3. Post-apply drift metric

- [x] 3.1 In `cmd/forseti/main.go` watch loop, after successful `ApplySettings()`, re-run `DiffSettings()` and call `UpdateSettingsDrift()` with post-apply state
- [x] 3.2 Keep pre-apply drift unchanged when `ApplySettings()` returns an error

## 4. Healthcheck

- [x] 4.1 Increase `--start-period` in Dockerfile HEALTHCHECK from 15s to 30s
- [x] 4.2 Add `depends_on` with `condition: service_healthy` for pihole containers in docker-compose.dev.yml (requires Pi-hole healthcheck)

## 5. Tests

- [x] 5.1 Update existing settings reconciliation tests to use `bool` values instead of `int` in expected mappings
- [x] 5.2 Add test cases for `settingsNeedUpdate`: bool↔int, bool↔float64, case-insensitive strings
- [x] 5.3 Run `go test ./...` and `go vet ./...` to verify all changes (golangci-lint not installed locally)
