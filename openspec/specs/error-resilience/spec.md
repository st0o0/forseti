# Error Resilience

Transient error detection, retry policy, and idempotent operation semantics for the Pi-hole API client layer.

### Requirement: Idempotent delete operations
The reconciler SHALL treat a 404 response on any delete operation as success. An item that is already absent does not need to be deleted.

#### Scenario: Adlist already deleted
- **WHEN** the reconciler attempts to delete an adlist that was already removed (Pi-hole returns 404)
- **THEN** the reconciler SHALL treat this as a successful deletion and not log an error

#### Scenario: Domain already deleted
- **WHEN** the reconciler attempts to delete a deny domain that returns 404
- **THEN** the reconciler SHALL treat this as a successful deletion

#### Scenario: Real delete error still reported
- **WHEN** a delete operation returns 500 (internal server error)
- **THEN** the reconciler SHALL report this as an error in the reconcile report

### Requirement: Transient error detection
The pihole client package SHALL expose an `IsTransient(error) bool` function that identifies errors caused by temporary conditions. Transient errors are: connection refused, connection reset, `APIError` with status 400 and message containing "database_error" or "Database not available", and `APIError` with status 503.

#### Scenario: Database unavailable during FTL restart
- **WHEN** Pi-hole returns `400 {"error":{"key":"database_error","message":"Could not read domains from database table"}}`
- **THEN** `IsTransient` SHALL return true

#### Scenario: Connection refused after FTL restart
- **WHEN** the TCP connection to Pi-hole is refused
- **THEN** `IsTransient` SHALL return true

#### Scenario: Permanent API error
- **WHEN** Pi-hole returns `400 {"error":{"key":"bad_request","message":"Invalid domain format"}}`
- **THEN** `IsTransient` SHALL return false

### Requirement: Retry on transient list errors
The reconciler SHALL retry list operations (ListAdlists, ListDomains, ListGroups, ListClients, ListLocalDNS, ListCNAME) up to 3 times with exponential backoff (1s, 2s, 4s) when the error is transient. Each retry SHALL be logged at WARN level.

#### Scenario: List succeeds on retry
- **WHEN** `ListDomains("deny","exact")` fails with "Database not available" on first attempt but succeeds on second
- **THEN** the reconciler SHALL use the second attempt's result and log one WARN for the retry

#### Scenario: List fails after all retries
- **WHEN** `ListAdlists` fails with a transient error on all 3 attempts
- **THEN** the reconciler SHALL return the final error as a hard failure for that target

#### Scenario: Non-transient list error not retried
- **WHEN** `ListDomains` fails with a 401 Unauthorized
- **THEN** the reconciler SHALL NOT retry and SHALL fail immediately

### Requirement: No retry on write operations
The reconciler SHALL NOT retry create, update, or delete operations. Only read (list) operations are safe to retry.

#### Scenario: Create fails with transient error
- **WHEN** creating an adlist fails with connection refused
- **THEN** the reconciler SHALL NOT retry the create and SHALL report the error

### Requirement: Extended gravity timeout
The Pi-hole client SHALL use a separate, extended timeout for gravity update operations. The default gravity timeout SHALL be 5 minutes. The timeout SHALL be configurable.

#### Scenario: Slow gravity update
- **WHEN** a gravity update takes 3 minutes to complete on a slow device
- **THEN** the client SHALL wait for the full response without timing out

#### Scenario: Gravity truly stuck
- **WHEN** a gravity update does not complete within the configured timeout
- **THEN** the client SHALL return a timeout error

### Requirement: IsNotFound helper
The pihole client package SHALL expose an `IsNotFound(error) bool` function that returns true when the error is an `APIError` with StatusCode 404.

#### Scenario: 404 detection
- **WHEN** a Pi-hole API call returns HTTP 404
- **THEN** `IsNotFound` SHALL return true for the resulting error

#### Scenario: Non-404 error
- **WHEN** a Pi-hole API call returns HTTP 500
- **THEN** `IsNotFound` SHALL return false
