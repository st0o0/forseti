## 1. Bool-to-int coercion

- [x] 1.1 Add `boolToInt(b bool) int` helper in `internal/reconcile/settings.go`
- [x] 1.2 Wrap all `*bool` dereferences in `BuildDesiredSettingsList` with `boolToInt` (14 fields: ForceOnDisk, DomainNeeded, BogusPriv, DNSSEC, QueryLogging, CNAMEDeepInspect, ResolveIPv4, ResolveIPv6, RevServer.Enabled, Blocking.Active, DHCP.Active, DHCP.IPv6, DHCP.RapidCommit, Misc.Check.Load)
- [x] 1.3 Add unit tests verifying `BuildDesiredSettingsList` produces `int(0)`/`int(1)` for all boolean fields, not `bool`

## 2. Type-aware comparison

- [x] 2.1 Add `normalizeValue(v any) any` helper that converts `bool→int` and whole `float64→int`
- [x] 2.2 Replace `settingsNeedUpdate` to normalize both sides before comparing
- [x] 2.3 Add unit tests: `bool(false)` vs `float64(0)` → equal, `int(10000)` vs `float64(10000)` → equal, `int(500)` vs `float64(1500)` → not equal

## 3. Collect-and-continue in ApplySettings

- [x] 3.1 Change `ApplySettings` loop to collect errors with `append` and `continue` instead of returning on first error
- [x] 3.2 Return `errors.Join(errs...)` after the loop completes
- [x] 3.3 Add test: 3 settings where the 2nd PatchConfig fails → verify 1st and 3rd are still applied, error returned contains the failing setting name

## 4. Improve existing test realism

- [x] 4.1 Update `TestDiffSettings_OneChange` mock to return `float64` values (matching real `json.Unmarshal` behavior) instead of Go bools
- [x] 4.2 Add test: mock returns `float64(0)` for optimizer, desired is `0` (coerced from `false`) → no diff reported
- [x] 4.3 Add test: mock `PatchConfig` returns error → verify error propagation and diff still returned

## 5. Verify and lint

- [x] 5.1 Run `go test -race ./internal/reconcile/...` — all tests pass
- [x] 5.2 Run `go vet ./...` — no issues
- [x] 5.3 Run `golangci-lint run` — no new warnings
