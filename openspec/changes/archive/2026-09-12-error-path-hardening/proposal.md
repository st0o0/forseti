## Why

Forseti's error handling treats all API failures as equal — a 404 on delete (item already gone) is logged the same as a 500 (server down), and transient errors like Pi-hole's "Database not available" during FTL restart immediately fail the entire reconcile with no retry. Production logs show recurring patterns: 404 on adlist delete, connection refused after settings changes, gravity timeouts, and database unavailability during FTL restarts. These are all recoverable situations that Forseti currently treats as hard failures.

## What Changes

- **Idempotent deletes**: Reconciler treats 404 on delete as success (item already gone). Uses `errors.As(*pihole.APIError)` to check `StatusCode`.
- **Transient error retry in reconciler**: List operations that fail with transient errors (400 "Database not available", connection refused) are retried with short backoff before failing the target.
- **Session invalidation after FTL ready**: Watch loop invalidates the session pool for a target after successful `WaitForReady`, since FTL restart kills existing sessions even though the unauthenticated `/api/info` endpoint responds.
- **Gravity timeout extension**: `TriggerGravity` uses an extended HTTP timeout (configurable, default 5min) since gravity updates are known long-running operations.
- **Tests for all error paths**: Each fix gets corresponding unit tests using `httptest.Server` mocks that return specific HTTP status codes and error bodies matching real Pi-hole responses.

## Capabilities

### New Capabilities
- `error-resilience`: Transient error detection, retry policy, and idempotent operation semantics across pihole client, reconciler, and gravity.

### Modified Capabilities
- `pihole-api`: APIError now actively used for status-code-specific handling (404 idempotent, transient detection).
- `reconciler`: Delete operations become idempotent; list operations gain retry on transient errors.

## Impact

- `internal/pihole/client.go` — gravity timeout, helper to check APIError status codes
- `internal/reconcile/reconciler.go` — 404 handling on deletes, retry on transient list errors
- `cmd/forseti/main.go` — session invalidation after WaitForReady
- New and extended tests in `client_test.go`, `reconciler_test.go`, `scheduler_test.go`
- No API changes, no config schema changes, no breaking changes
