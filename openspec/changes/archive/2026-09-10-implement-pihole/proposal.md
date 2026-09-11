## Why

The Pi-hole API client (`internal/pihole`) is needed by both the reconciler and the metrics server. It wraps the Pi-hole v6 REST API with session-based authentication, CRUD operations for all managed resource types, and stats retrieval. With the config package done, this is the next dependency to unblock.

## What Changes

- Implement the Pi-hole v6 REST API client in `internal/pihole/client.go`
- Session-based auth lifecycle: login (`POST /api/auth`), session header on all requests, logout (`DELETE /api/auth`) with guaranteed cleanup
- CRUD operations: adlists, domains (deny/allow), groups, clients, local DNS (A records + CNAME)
- Batch delete support via `:batchDelete` endpoints
- Stats retrieval from `/api/stats/summary` and related endpoints
- Gravity trigger via `POST /api/action/gravity`
- Unit tests using `net/http/httptest` to mock the Pi-hole API

## Capabilities

### New Capabilities

None. Implements the existing `pihole-api` spec.

### Modified Capabilities

None.

## Impact

- `internal/pihole/client.go` — full implementation replacing the empty stub
- `internal/pihole/client_test.go` — new test file
- `go.mod` — no new dependencies (uses stdlib `net/http` and `encoding/json`)
