## MODIFIED Requirements

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
