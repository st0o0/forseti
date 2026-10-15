# Target Worker

Per-target worker with lifecycle management, health tracking, and interface-driven orchestration.

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

### Requirement: Worker maintains cross-cycle state
The worker SHALL persist state between reconcile cycles: health status, consecutive failure count, last error, last success time, and backoff timer.

#### Scenario: Consecutive failures tracked
- **WHEN** a worker fails 3 cycles in a row then succeeds
- **THEN** consecutive failures SHALL be 3 before the success and 0 after

#### Scenario: State survives config update
- **WHEN** a hot-reload updates the worker's config
- **THEN** health state, backoff timer, and failure count SHALL be preserved

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

### Requirement: Config update without restart
The worker SHALL support updating its target configuration via `UpdateConfig()` without losing state. This enables hot-reload to add, update, or remove targets without restarting the process.

#### Scenario: Target config changes on reload
- **WHEN** the YAML config changes and hot-reload fires
- **THEN** existing workers SHALL receive updated config, new targets SHALL get new workers, removed targets SHALL have their workers stopped

### Requirement: Cycle outcome is always recorded
After every reconcile cycle the worker SHALL record the run outcome (`success`, `partial` or `error`) and the resulting health state in metrics, on every exit path: session failure, readiness failure, settings failure, content failure, and success. A cycle whose content apply returned item-level errors SHALL be recorded as `partial` and SHALL count as a success for health and backoff purposes, because the target was reachable.

#### Scenario: Readiness failure recorded
- **WHEN** the readiness check fails for target `pizero`
- **THEN** the worker SHALL record run status `error`, mark the target unreachable, transition health to degraded, and publish the degraded health state

#### Scenario: Content apply error recorded
- **WHEN** content reconciliation returns an error for target `pizero`
- **THEN** the worker SHALL record run status `error` and publish the degraded health state

#### Scenario: Item-level errors are partial
- **WHEN** content reconciliation succeeds overall but reports one failed item write
- **THEN** the worker SHALL record run status `partial`, keep health at healthy, and not activate backoff
