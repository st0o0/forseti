## 1. Fix session_active counter

- [x] 1.1 Add `OnInvalidate func(target string)` to `PoolCallbacks` in `internal/session/pool.go`
- [x] 1.2 Call `OnInvalidate` in `Invalidate()` when a session is actually removed (client exists in map)
- [x] 1.3 Wire `OnInvalidate` to `session_active.Dec()` in `cmd/forseti/main.go` (alongside existing `OnNewSession`)
- [x] 1.4 Add test in `internal/session/pool_test.go`: verify callback fires on Invalidate, does not fire on Invalidate of unknown target

## 2. Fix comment/marker sync in diff functions

- [x] 2.1 In `diffAdlists`: when URL matches and groups match, also check `strings.Contains(actual.Comment, marker)` — if marker is missing, emit Update instead of Unchanged
- [x] 2.2 In `diffDomains` (deny): when domain matches, check marker in comment — if missing, emit Update
- [x] 2.3 In `diffAllowDomains`: same marker check as deny
- [x] 2.4 In `diffClients`: when client IP matches and groups match, check marker — if missing, emit Update
- [x] 2.5 Add tests in `internal/reconcile/reconciler_test.go` for each diff function: entry exists with correct groups but wrong comment → should be Update, not Unchanged

## 3. Verify

- [x] 3.1 Run `go test ./...` and `golangci-lint run`
- [x] 3.2 Docker E2E: verify session_active stays at 2, StevenBlack gets `[forseti]` marker after first reconcile
