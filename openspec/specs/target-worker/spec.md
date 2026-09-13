# Target Worker

Per-target worker with lifecycle management, health tracking, and interface-driven orchestration.

### Requirement: Worker encapsulates target orchestration
The system SHALL provide a TargetWorker that encapsulates the full reconciliation sequence for a single target: gate acquisition, session acquisition, settings reconciliation, content reconciliation, gravity triggering, and metrics recording. The worker SHALL depend on interfaces, not concrete types. The worker SHALL acquire the target's concurrency gate before starting any API work and release it when the reconcile cycle completes (including on error paths).

#### Scenario: Normal reconcile cycle
- **WHEN** `Reconcile()` is called on a healthy worker
- **THEN** it SHALL execute: acquire gate → get session → reconcile settings → reconcile content → trigger gravity (if needed) → record metrics → release gate

#### Scenario: Settings change triggers FTL restart
- **WHEN** settings reconciliation detects changes that cause FTL restart
- **THEN** the worker SHALL wait for FTL ready, refresh the session, and proceed with content reconciliation using the fresh session (all while holding the gate)

#### Scenario: Session acquisition fails
- **WHEN** the session manager returns an error
- **THEN** the worker SHALL release the gate, skip all reconciliation steps, increment consecutive failures, and mark target as degraded

#### Scenario: Gate released on panic
- **WHEN** a panic occurs during reconciliation
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
The worker SHALL maintain a health state with three levels: healthy, degraded, and down. Transitions: healthy → degraded on failure, degraded → healthy on success, degraded → down on gravity corruption, down → healthy on success.

#### Scenario: Healthy to degraded
- **WHEN** a reconcile cycle fails
- **THEN** health SHALL transition to degraded and backoff SHALL activate

#### Scenario: Degraded to healthy
- **WHEN** a reconcile cycle succeeds after previous failures
- **THEN** health SHALL transition to healthy, consecutive failures SHALL reset to 0, and backoff SHALL clear

#### Scenario: Gravity corruption detected
- **WHEN** a list operation fails with "no such table" error
- **THEN** health SHALL transition to down

### Requirement: Config update without restart
The worker SHALL support updating its target configuration via `UpdateConfig()` without losing state. This enables hot-reload to add, update, or remove targets without restarting the process.

#### Scenario: Target config changes on reload
- **WHEN** the YAML config changes and hot-reload fires
- **THEN** existing workers SHALL receive updated config, new targets SHALL get new workers, removed targets SHALL have their workers stopped
