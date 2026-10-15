## MODIFIED Requirements

### Requirement: Worker encapsulates target orchestration
The system SHALL provide a TargetWorker that runs one target through a mode-independent envelope: gate acquisition, session acquisition, readiness check, the mode-specific step (config mode: settings reconciliation, content reconciliation and gravity triggering; sync mode: primary snapshot load or replica sync), outcome classification, health update and metrics recording. The same envelope SHALL be used in config and sync mode. The worker SHALL depend on interfaces, not concrete types. The worker SHALL acquire the target's concurrency gate before starting any API work and release it when the run completes (including on error paths).

#### Scenario: Normal reconcile cycle
- **WHEN** a config-mode run is executed on a healthy worker
- **THEN** it SHALL execute: acquire gate → get session → check readiness → reconcile settings → reconcile content → trigger gravity (if needed) → record outcome → release gate

#### Scenario: Sync replica run
- **WHEN** a sync-mode run is executed for a replica
- **THEN** it SHALL execute: acquire gate → get session → check readiness → sync resources from the primary snapshot → record outcome → release gate

#### Scenario: Settings change triggers FTL restart
- **WHEN** settings reconciliation detects changes that cause FTL restart
- **THEN** the worker SHALL wait for FTL ready, refresh the session, and proceed with content reconciliation using the fresh session (all while holding the gate)

#### Scenario: Session acquisition fails
- **WHEN** the session manager returns an error
- **THEN** the worker SHALL release the gate, skip all further steps, increment consecutive failures, and mark target as degraded

#### Scenario: Gate released on panic
- **WHEN** a panic occurs during the run
- **THEN** the gate SHALL still be released via defer, preventing permanent slot exhaustion

### Requirement: Health state machine
The worker SHALL maintain a health state with three levels: healthy, degraded, and down. Transitions: healthy → degraded on the first failure; degraded → down after 5 consecutive failures or on gravity corruption; degraded or down → healthy on success. Each transition SHALL be logged once (`target degraded`, `target down`, `target recovered`). A partial run counts as success.

#### Scenario: Healthy to degraded
- **WHEN** a run fails
- **THEN** health SHALL transition to degraded, backoff SHALL activate and `target degraded` SHALL be logged

#### Scenario: Degraded to down after repeated failures
- **WHEN** a target fails its fifth consecutive run
- **THEN** health SHALL transition to down and `target down` SHALL be logged at error level

#### Scenario: Degraded to healthy
- **WHEN** a run succeeds after previous failures
- **THEN** health SHALL transition to healthy, consecutive failures SHALL reset to 0, backoff SHALL clear and `target recovered` SHALL be logged with the number of failures and the downtime

#### Scenario: Gravity corruption detected
- **WHEN** a list operation fails with "no such table" error
- **THEN** health SHALL transition to down

#### Scenario: No repeated transition logs
- **WHEN** a degraded target fails again without reaching the down threshold
- **THEN** no additional `target degraded` event SHALL be logged
