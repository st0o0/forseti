# Pi-hole API (delta)

## MODIFIED Requirements

### Requirement: Gravity update trigger
The client SHALL support triggering a gravity update via `POST /api/action/gravity`. This is a heavyweight operation that re-downloads all adlists. The gravity request SHALL use a separate, extended HTTP timeout (default 5 minutes) independent of the client's standard request timeout.

#### Scenario: Trigger after adlist change
- **WHEN** the reconciler has added or removed adlists
- **THEN** the client SHALL call the gravity action endpoint

#### Scenario: Gravity timeout independent of default
- **WHEN** the default client timeout is 30s and gravity timeout is 5m
- **THEN** the gravity request SHALL use the 5m timeout, not the 30s default
