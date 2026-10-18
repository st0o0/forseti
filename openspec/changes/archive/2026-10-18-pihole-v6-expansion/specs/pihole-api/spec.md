## ADDED Requirements

### Requirement: Allow-list API methods
The Pi-hole client SHALL provide CRUD methods for allow-lists: `ListAllowlists`, `CreateAllowlist`, `UpdateAllowlist`, `DeleteAllowlists`. These SHALL mirror the adlist methods using `?type=allow` instead of `?type=block`.

#### Scenario: List allowlists
- **WHEN** `ListAllowlists()` is called
- **THEN** the client SHALL send `GET /api/lists?type=allow` and return the parsed list

#### Scenario: Create allowlist
- **WHEN** `CreateAllowlist(address, comment, enabled, groups)` is called
- **THEN** the client SHALL send `POST /api/lists?type=allow` with the given body

#### Scenario: Update allowlist
- **WHEN** `UpdateAllowlist(address, comment, groups)` is called
- **THEN** the client SHALL send `PUT /api/lists/{address}?type=allow` with the given body

#### Scenario: Delete allowlists
- **WHEN** `DeleteAllowlists(addresses)` is called
- **THEN** the client SHALL send `POST /api/lists:batchDelete?type=allow` and fall back to individual DELETE on 404

### Requirement: Top stats API methods
The Pi-hole client SHALL provide methods to fetch top queried domains, top blocked domains, and top clients.

#### Scenario: Fetch top domains
- **WHEN** `GetTopDomains(count)` is called
- **THEN** the client SHALL send `GET /api/stats/top_domains?count={count}` and return parsed results

#### Scenario: Fetch top clients
- **WHEN** `GetTopClients(count)` is called
- **THEN** the client SHALL send `GET /api/stats/top_clients?count={count}` and return parsed results

### Requirement: History API method
The Pi-hole client SHALL provide a method to fetch time-bucketed query history.

#### Scenario: Fetch history
- **WHEN** `GetHistory()` is called
- **THEN** the client SHALL send `GET /api/history` and return parsed time-bucketed data

### Requirement: System info API method
The Pi-hole client SHALL provide a method to fetch host system information.

#### Scenario: Fetch system info
- **WHEN** `GetSystemInfo()` is called
- **THEN** the client SHALL send `GET /api/info/system` and return parsed CPU, memory, temperature, and load data

## MODIFIED Requirements

### Requirement: UpdateDomain supports all type/kind combinations
The `UpdateDomain` method SHALL accept `domType` and `kind` parameters to construct the correct API path (`/api/domains/{type}/{kind}/{domain}`), replacing the current hardcoded `deny/exact` path.

#### Scenario: Update deny exact domain
- **WHEN** `UpdateDomain("deny", "exact", domain, comment, groups)` is called
- **THEN** the client SHALL send `PUT /api/domains/deny/exact/{domain}`

#### Scenario: Update allow regex domain
- **WHEN** `UpdateDomain("allow", "regex", domain, comment, groups)` is called
- **THEN** the client SHALL send `PUT /api/domains/allow/regex/{domain}`
