## Why

Two bugs found during E2E testing:

1. **session_active counter leak**: `forseti_session_active` only increments on new sessions (`OnNewSession`) but never decrements on `Invalidate()`. Each session invalidation (settings change → FTL restart → re-login) permanently inflates the counter. In E2E testing, session_active reached 5 instead of the expected 2.

2. **Comment/marker not synced on existing entries**: `diffAdlists`, `diffDomains`, `diffAllowDomains`, and `diffClients` only compare groups (or domain) when an entry already exists by URL/key. The comment field — which carries the `[forseti]` managed marker — is never compared. Pre-existing entries (e.g. Pi-hole's default StevenBlack adlist) never get the marker, making them immune to deletion when later removed from config.

## What Changes

- **Fix session counter**: Decrement `session_active` gauge in session pool's `Invalidate()` callback path.
- **Sync comment/marker on existing entries**: When the diff finds a URL match but the actual comment doesn't contain the marker, emit an Update so the reconciler sets the correct `[forseti] <user comment>`. This applies to adlists, deny/allow domains, and clients.

## Capabilities

### New Capabilities

_(none)_

### Modified Capabilities

- `metrics`: session_active gauge must decrement when a session is invalidated, not only reset on pool close.
- `reconciler`: Diff functions must detect comment/marker mismatch as drift requiring an update.

## Impact

- `internal/session/pool.go` — add `OnInvalidate` callback or decrement in `Invalidate()`
- `internal/reconcile/reconciler.go` — `diffAdlists`, `diffDomains`, `diffAllowDomains`, `diffClients` add comment comparison
- `internal/reconcile/reconciler_test.go` — new test cases for comment-mismatch drift
- `internal/session/pool_test.go` — test for counter decrement on invalidate
- No API or config changes
