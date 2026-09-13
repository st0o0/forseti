## MODIFIED Requirements

### Requirement: Gravity update trigger
The client SHALL support triggering a gravity update via `POST /api/action/gravity`. This is a heavyweight operation that re-downloads all adlists. The gravity request SHALL use a separate, extended HTTP timeout (default 5 minutes) independent of the client's standard request timeout. The gravity request SHALL use a locally-scoped HTTP client for the extended timeout instead of modifying the shared `httpClient` field, to prevent data races with concurrent API calls.

#### Scenario: Trigger after adlist change
- **WHEN** the reconciler has added or removed adlists
- **THEN** the client SHALL call the gravity action endpoint

#### Scenario: Gravity timeout independent of default
- **WHEN** the default client timeout is 30s and gravity timeout is 5m
- **THEN** the gravity request SHALL use the 5m timeout, not the 30s default

#### Scenario: Concurrent safety during gravity
- **WHEN** a gravity request is in progress and another goroutine uses the same client for a stats call
- **THEN** the stats call SHALL use the client's configured timeout, unaffected by the gravity timeout

## ADDED Requirements

### Requirement: Configurable client timeout
The `pihole.Client` SHALL accept a configurable HTTP timeout at construction time via `NewClient`. The timeout SHALL apply to all standard API requests (auth, CRUD, stats). The default SHALL remain 30s for backward compatibility.

#### Scenario: Client with custom timeout
- **WHEN** a client is created with timeout=45s
- **THEN** all HTTP requests (except gravity) SHALL use a 45s timeout

#### Scenario: Client with default timeout
- **WHEN** a client is created without specifying a timeout (zero value)
- **THEN** all HTTP requests (except gravity) SHALL use the default 30s timeout
