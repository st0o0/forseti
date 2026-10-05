## ADDED Requirements

### Requirement: Query history retrieval
The client SHALL support retrieving query history via `GET /api/queries`, accepting a time range (`from`/`until` as Unix timestamps) and a status filter restricted to blocked statuses. The call SHALL use the same session-authenticated request path (including auto-reauth on `401`) as other read operations such as `GetStats`.

#### Scenario: Fetch blocked queries for a rolling window
- **WHEN** the collector requests blocked query history for a target with `from=now-24h` and `until=now`
- **THEN** the client SHALL call `GET /api/queries` with those time bounds and a blocked-status filter, authenticated via the target's existing session

#### Scenario: Session expiry during query history fetch
- **WHEN** `GET /api/queries` returns `401` due to an expired session
- **THEN** the client SHALL re-authenticate and retry the request, consistent with the existing auto-reauth behavior
