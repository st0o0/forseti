# Pi-hole API

Pi-hole v6 REST API client contract: session authentication, CRUD operations, batch constraints, stats retrieval.

### Requirement: Session-based authentication
The system SHALL authenticate against Pi-hole v6 using `POST /api/auth` which returns a session ID. All subsequent requests SHALL include the `X-FTL-SID` header. After operations complete, the system SHALL call `DELETE /api/auth` to release the session.

#### Scenario: Auth lifecycle
- **WHEN** the client starts a reconcile operation
- **THEN** it SHALL authenticate, perform all API calls with the session header, and logout when done — even if operations fail

#### Scenario: Session cleanup on error
- **WHEN** an API call fails mid-reconcile
- **THEN** the client SHALL still call `DELETE /api/auth` before returning the error

### Requirement: Session limit awareness
Pi-hole v6 allows a maximum of 16 concurrent sessions. The client SHALL never leak sessions. If authentication fails due to session exhaustion, the client SHALL report this clearly.

#### Scenario: Session limit reached
- **WHEN** `POST /api/auth` fails because 16 sessions are active
- **THEN** the client SHALL return an error indicating session limit exhaustion

### Requirement: Adlist CRUD operations
The client SHALL support: listing all adlists (`GET /api/lists`), creating an adlist (`POST /api/lists` with address, comment, enabled, groups), and deleting adlists (`POST /api/lists:batchDelete`). Individual GET/PUT/DELETE by list ID SHALL also be supported.

#### Scenario: Create adlist with marker
- **WHEN** creating an adlist for URL `https://example.com/list.txt` with marker `[forseti]`
- **THEN** the client SHALL POST to `/api/lists` with the comment field containing `[forseti]`

### Requirement: Domain CRUD operations
The client SHALL support allow and deny domains through the `/api/domains/{type}/{kind}` endpoints. Type is `allow` or `deny`, kind is `exact` or `regex`. Batch delete is available via `POST /api/domains:batchDelete`.

#### Scenario: Add deny domain
- **WHEN** adding `ads.example.com` to the deny list
- **THEN** the client SHALL POST to `/api/domains/deny/exact` with the domain and marker comment

### Requirement: Group CRUD operations
The client SHALL support group management through `/api/groups` endpoints. Groups are first-class entities — adlists, domains, and clients reference groups by ID.

#### Scenario: Create group before dependents
- **WHEN** the reconciler creates a new group
- **THEN** the group SHALL be available for reference by adlists, domains, and clients created afterward

### Requirement: Client CRUD operations
The client SHALL support client management through `/api/clients` endpoints. Clients are matched by IP or CIDR and assigned to groups.

#### Scenario: Assign client to group
- **WHEN** creating a client with match `192.168.1.0/24` and group `default`
- **THEN** the client SHALL POST to `/api/clients` with the group reference

### Requirement: Local DNS management
The client SHALL manage local DNS A records through `/api/config/dns/hosts`. Entries are URL-encoded `"IP domain"` strings. CNAME records are managed through `/api/config/dns/cnameRecords`.

#### Scenario: Add A record
- **WHEN** adding local DNS for `internal.lan` -> `192.168.1.100`
- **THEN** the client SHALL PUT to `/api/config/dns/hosts/{entry}` with entry being URL-encoded `192.168.1.100 internal.lan`

#### Scenario: CNAME change triggers restart
- **WHEN** a CNAME record is added or removed
- **THEN** Pi-hole FTL restarts automatically, causing a brief DNS outage — the client SHALL log a warning

### Requirement: Stats retrieval for metrics
The client SHALL retrieve Pi-hole statistics from `/api/stats/summary`, `/api/stats/upstreams`, and `/api/dns/blocking` for exposure as Prometheus metrics. Extended parsing requirements are defined in the `pihole-stats-collection` spec.

#### Scenario: Stats scrape
- **WHEN** the metrics server needs fresh Pi-hole stats
- **THEN** the client SHALL call the stats endpoints and return structured data for metric population

### Requirement: Gravity update trigger
The client SHALL support triggering a gravity update via `POST /api/action/gravity`. This is a heavyweight operation that re-downloads all adlists.

#### Scenario: Trigger after adlist change
- **WHEN** the reconciler has added or removed adlists
- **THEN** the client SHALL call the gravity action endpoint

### Requirement: Teleporter backup before apply
The client SHALL support downloading a full config backup via `GET /api/teleporter` (ZIP file) as a safety net before applying changes. This is optional but recommended for destructive reconcile operations.

#### Scenario: Backup before first apply
- **WHEN** the reconciler is about to apply changes to a target for the first time
- **THEN** the client SHOULD offer to download a teleporter backup before proceeding

### Requirement: Endpoint self-discovery
The client SHALL support querying `GET /api/endpoints` to discover available API endpoints. This enables version-compatibility checking against different Pi-hole v6 releases.

#### Scenario: Version compatibility check
- **WHEN** the client connects to a Pi-hole instance
- **THEN** it SHALL verify that required endpoints are available before attempting reconciliation

### Requirement: Domain search for debugging
The client SHALL support querying `GET /api/search/{domain}` to search a domain across all lists. This is useful for validation and debugging.

#### Scenario: Debug blocked domain
- **WHEN** a user wants to check why a domain is blocked
- **THEN** the client SHALL search the domain across all adlists, deny lists, and allow lists

### Requirement: No batch create
Pi-hole v6 API provides batch delete (`:batchDelete`) but no batch create. The client SHALL create entries one at a time via individual POST requests.

#### Scenario: Adding multiple adlists
- **WHEN** the reconciler needs to add 10 new adlists
- **THEN** the client SHALL make 10 individual POST requests (not a single batch call)
