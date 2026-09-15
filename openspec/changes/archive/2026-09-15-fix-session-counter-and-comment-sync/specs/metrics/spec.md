## MODIFIED Requirements

### Requirement: Forseti reconciliation metrics
The system SHALL expose reconciliation telemetry:

- `forseti_reconcile_runs_total{target, status}` — counter of reconcile cycles (status: success/error)
- `forseti_reconcile_duration_seconds{target}` — histogram of reconcile cycle duration
- `forseti_reconcile_changes_total{target, type, action}` — counter of changes applied (type: adlist/deny/allow/dns/group/client, action: add/delete)
- `forseti_reconcile_drift{target, type}` — gauge of items that differ from desired state at each cycle
- `forseti_target_reachable{target}` — gauge (1 = reachable, 0 = unreachable)
- `forseti_session_active` — gauge of active sessions in pool

`forseti_session_active` SHALL increment when a new session is created and decrement when a session is invalidated. It SHALL reset to 0 when the pool is closed. The gauge MUST accurately reflect the number of live sessions at all times.

After each reconcile cycle, `forseti_reconcile_drift` SHALL be set for all resource types (groups, adlists, deny, allow, local_dns, cname, clients). Resource types with no drift SHALL be set to 0. The gauge MUST NOT retain stale values from previous cycles.

#### Scenario: Successful reconcile updates metrics
- **WHEN** a reconcile cycle completes successfully against target `pihole-router` with 2 adlists added
- **THEN** `forseti_reconcile_runs_total{target="pihole-router", status="success"}` SHALL increment by 1 and `forseti_reconcile_changes_total{target="pihole-router", type="adlist", action="add"}` SHALL increment by 2

#### Scenario: Failed reconcile
- **WHEN** a reconcile cycle fails against target `pihole-pi`
- **THEN** `forseti_reconcile_runs_total{target="pihole-pi", status="error"}` SHALL increment by 1

#### Scenario: Drift resolves after apply
- **WHEN** a previous cycle reported `forseti_reconcile_drift{target="pizero", type="adlists"} = 1` and the next cycle finds no adlist drift
- **THEN** `forseti_reconcile_drift{target="pizero", type="adlists"}` SHALL be 0

#### Scenario: Session invalidation decrements gauge
- **WHEN** a session for target "alpha" is invalidated (auth failure, settings change restart)
- **THEN** `forseti_session_active` SHALL decrement by 1

#### Scenario: Session re-creation after invalidation
- **WHEN** a session is invalidated and then re-created on next reconcile
- **THEN** `forseti_session_active` SHALL first decrement then increment, ending at the same value
