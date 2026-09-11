## Context

The `internal/pihole` package is an empty stub. The pihole-api spec defines the full REST API contract for Pi-hole v6. The client needs to handle session authentication, CRUD for six resource types, stats retrieval, and gravity triggers.

## Goals / Non-Goals

**Goals:**
- Type-safe client with `NewClient(baseURL, password) *Client`
- Session lifecycle via `Login()` / `Close()` with `Close()` safe to call in defer
- CRUD methods for each resource type matching the API contract
- Response types for API payloads
- Testable via `httptest.Server`

**Non-Goals:**
- Retries or circuit breakers (add later if needed)
- Connection pooling beyond Go's default `http.Client`
- Teleporter backup (optional feature, defer to later)
- Endpoint self-discovery (defer until multi-version support needed)

## Decisions

### Client struct with session state
```
Client
├── baseURL    string
├── password   string
├── httpClient *http.Client
├── sid        string (session ID from auth)
```
All methods check `sid != ""` and return error if not logged in. `Close()` is idempotent.

### Resource type naming
API response types mirror the Pi-hole v6 API JSON structure:
- `APIList` (adlists), `APIDomain`, `APIGroup`, `APIClient`, `APIDNSRecord`
- Each has JSON tags matching the API response fields

### Error handling
Wrap all non-2xx responses in a structured `APIError{StatusCode, Message}` that implements `error`. This gives callers enough context to distinguish auth failures from not-found from server errors.

### No batch create
Per the spec, Pi-hole v6 has no batch create — only batch delete. Create methods handle single items. The reconciler will loop over creates.

### Stats as a flat struct
`Stats` struct captures the summary fields needed for Prometheus metrics. No need for the full Pi-hole stats hierarchy.

## Risks / Trade-offs

- **Session leak on crash** → `Close()` in defer handles normal flows; abnormal termination leaves a session. Pi-hole's 16-session limit means leaked sessions are noticeable but self-heal on Pi-hole restart.
- **API version drift** → Pi-hole v6 API may change between releases. Tests use httptest mocks, so real API changes won't be caught until integration testing against the dev environment.
