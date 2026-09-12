## 1. Error Classification Helpers (pihole package)

- [x] 1.1 Add `IsNotFound(error) bool` helper to `internal/pihole/client.go` — unwraps to `*APIError`, checks `StatusCode == 404`
- [x] 1.2 Add `IsTransient(error) bool` helper — matches connection refused, connection reset, 400 with "database_error"/"Database not available", and 503
- [x] 1.3 Write tests for `IsNotFound` and `IsTransient` with table-driven cases covering: APIError 404, APIError 400 database_error, APIError 400 bad_request, APIError 503, connection refused (net.OpError), wrapped errors, nil error

## 2. Extended Gravity Timeout (pihole package)

- [x] 2.1 Add `GravityTimeout time.Duration` field to `Client` struct with 5-minute default
- [x] 2.2 Modify `TriggerGravity` to create a derived `http.Client` with the gravity-specific timeout
- [x] 2.3 Write test: gravity call with slow server (responds after 2s) succeeds with extended timeout but would fail with default 30s-style timeout
- [x] 2.4 Write test: gravity call with server that never responds times out at configured gravity timeout

## 3. Idempotent Deletes (reconciler)

- [x] 3.1 Modify all delete error handling in `reconciler.go` (adlists, deny, allow, groups, clients, DNS, CNAME) to skip `IsNotFound` errors instead of appending to `report.Errors`
- [x] 3.2 Write tests: reconciler delete paths return 404 from mock — verify no error in report
- [x] 3.3 Write tests: reconciler delete paths return 500 from mock — verify error IS in report

## 4. Retry on Transient List Errors (reconciler)

- [x] 4.1 Add a `retryList` helper function in the reconciler that wraps a list call with retry (3 attempts, 1s/2s/4s backoff, only on `IsTransient`)
- [x] 4.2 Wrap all list calls in the reconciler (`ListAdlists`, `ListDomains` for deny/exact, deny/regex, allow/exact, allow/regex, `ListGroups`, `ListClients`, `ListLocalDNS`, `ListCNAME`) with `retryList`
- [x] 4.3 Write test: mock returns transient error on first call, success on second — reconcile succeeds
- [x] 4.4 Write test: mock returns transient error on all 3 attempts — reconcile fails with the final error
- [x] 4.5 Write test: mock returns non-transient error (401) — no retry, immediate failure

## 5. Session Invalidation After Settings (watch loop)

- [x] 5.1 In `cmd/forseti/main.go` watch loop, add `pool.Invalidate(target)` call after successful `WaitForReady` following a settings change
- [x] 5.2 Write test or verify manually: after settings change + WaitForReady, subsequent reconcile gets a fresh session

## 6. Validation

- [x] 6.1 Run `go test -race ./...` — all tests pass
- [x] 6.2 Run `golangci-lint run` — no new lint violations
- [x] 6.3 Run `go vet ./...` — clean
