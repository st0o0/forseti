# Session Observability

Prometheus metrics for session pool health: reauth events and active session count.

### Requirement: Session reauth counter
The metrics server SHALL register `forseti_session_reauth_total` as a Counter with label `{target}`. Each time the Pi-hole client successfully re-authenticates after a 401 response, the counter SHALL increment for that target.

#### Scenario: Reauth on 401
- **WHEN** a Pi-hole API call returns 401 and the client successfully re-authenticates
- **THEN** `forseti_session_reauth_total{target}` SHALL increment by 1

#### Scenario: Failed reauth not counted
- **WHEN** a 401 occurs but re-authentication fails
- **THEN** `forseti_session_reauth_total` SHALL NOT increment (only successful reauths are counted)

### Requirement: Active session gauge
The metrics server SHALL register `forseti_session_active` as a Gauge that reflects the number of active sessions in the session pool.

#### Scenario: Session created
- **WHEN** Pool.Get() creates a new session for a target
- **THEN** `forseti_session_active` SHALL increase by 1

#### Scenario: Pool closed
- **WHEN** Pool.Close() is called
- **THEN** `forseti_session_active` SHALL be set to 0
