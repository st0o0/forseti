## Context

Forseti currently treats all Pi-hole API errors uniformly — every non-2xx response becomes an `APIError` that propagates up as a failure. In production, FTL restarts (triggered by settings changes or CNAME records) create a window where the API returns 404s, 400 "Database not available", or connection refused. These are all transient conditions that resolve within seconds, but Forseti logs them as errors and moves on without retry.

The `APIError` type already carries `StatusCode` and `Message`, but no caller inspects them for recovery decisions.

## Goals / Non-Goals

**Goals:**
- Make delete operations idempotent (404 = already gone = success)
- Add targeted retry for transient errors on list/read operations
- Invalidate stale sessions after FTL restart detection
- Give gravity operations an appropriate timeout
- Test every error path with realistic Pi-hole error responses

**Non-Goals:**
- General retry middleware for all HTTP calls (too broad, hides real failures)
- Circuit breaker pattern (overkill for 2-4 targets)
- Retry on create/update operations (not idempotent, could cause duplicates)
- Integration test harness with simulated FTL restarts (that's Problem B)

## Decisions

### 1. Idempotent deletes via APIError status check

**Decision**: Add a helper `IsNotFound(error) bool` that unwraps to `*APIError` and checks `StatusCode == 404`. Reconciler delete paths use this to treat 404 as success.

**Why not ignore all delete errors?** A 500 on delete is a real problem (server crash, disk full). Only 404 is semantically "already gone."

### 2. Retry only on list operations, not writes

**Decision**: Wrap list calls (`ListAdlists`, `ListDomains`, etc.) in a retry loop with 3 attempts, 1s/2s/4s backoff. Retry on connection refused and on `APIError` with 400 + "database_error" key.

**Why only lists?** Lists are read-only and safe to retry. Creates and deletes are not idempotent (batch delete could partially succeed). The production failures show list operations hitting "Database not available" — that's the transient window after FTL restart.

**Alternative considered**: Retry at HTTP client level. Rejected because not all 400s are transient — only the specific "database_error" key indicates FTL restart.

### 3. Transient error detection

**Decision**: Add `IsTransient(error) bool` that matches:
- Connection refused / connection reset (net errors)
- `APIError` with StatusCode 400 and message containing "database_error" or "Database not available"
- `APIError` with StatusCode 503

**Why not 500?** Pi-hole 500s are typically real bugs, not transient conditions. The "Database not available" pattern is specifically a 400 with a structured error body.

### 4. Session invalidation after WaitForReady

**Decision**: In the watch loop, after `WaitForReady` succeeds following a settings change, call `pool.Invalidate(target)` before proceeding to reconcile.

**Why?** `WaitForReady` polls the unauthenticated `/api/info` endpoint. FTL being "ready" doesn't mean old authenticated sessions survived the restart. Without invalidation, the reconciler uses a dead session and gets connection refused or 401.

### 5. Extended gravity timeout

**Decision**: `TriggerGravity` creates a derived `http.Client` with a 5-minute timeout (configurable via `GravityTimeout` field on `Client`). This only affects the gravity call, not other API operations.

**Why a separate client?** The default client timeout (30s) is correct for all other API calls. Gravity downloads all adlists and rebuilds the database — on a Pi Zero with large lists, this can take minutes.

## Risks / Trade-offs

- **Retry masks real failures** → Mitigated by limiting retry to list operations only and capping at 3 attempts with logs at WARN level for each retry.
- **Gravity timeout too long** → 5min default is generous but gravity on a Pi Zero with 20+ lists can genuinely take that long. Made configurable so users can tune.
- **Session invalidation is eager** → We invalidate even if the old session might still work. Cost is one extra login (< 100ms). Benefit is eliminating an entire class of stale-session errors.
