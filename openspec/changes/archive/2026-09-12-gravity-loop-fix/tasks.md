## 1. NeedsGravity from actual outcomes (reconciler)

- [x] 1.1 In `Apply()`, track `adlistsChanged bool` — set true on successful CreateAdlist, UpdateAdlist, or non-404 DeleteAdlists
- [x] 1.2 Replace `report.Diff.NeedsGravity = report.Diff.Adlists.HasChanges()` with `report.Diff.NeedsGravity = adlistsChanged`
- [x] 1.3 Write test: all adlist deletes return 404 → NeedsGravity is false
- [x] 1.4 Write test: adlist create succeeds + delete returns 404 → NeedsGravity is true
- [x] 1.5 Write test: adlist delete succeeds (non-404) → NeedsGravity is true

## 2. Gravity in-flight tracking (scheduler)

- [x] 2.1 Add `inFlight map[string]bool` field to gravity Scheduler, protected by existing mutex
- [x] 2.2 In `trigger()` and `TriggerNow()`, check and set inFlight before the gravity call, clear with defer after
- [x] 2.3 If target is already in-flight, skip and log at WARN
- [x] 2.4 Write test: concurrent trigger for same target is skipped
- [x] 2.5 Write test: triggers for different targets run independently

## 3. Gravity corruption detection (reconcile path)

- [x] 3.1 Add `IsGravityCorrupted(error) bool` helper to pihole package — matches "no such table" in error message
- [x] 3.2 In reconciler, when a list operation fails after retries, check IsGravityCorrupted and return a specific error type
- [x] 3.3 In watch loop (main.go), detect gravity corruption error and mark target unreachable with specific log message
- [x] 3.4 Write test: IsGravityCorrupted matches "no such table: group" and "no such table: gravity"
- [x] 3.5 Write test: IsGravityCorrupted does not match "Database not available"

## 4. Validation

- [x] 4.1 Run `go test ./...` — all tests pass
- [x] 4.2 Run `golangci-lint run` — no new lint violations
- [x] 4.3 Run `go vet ./...` — clean
