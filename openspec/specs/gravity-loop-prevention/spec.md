# Gravity Loop Prevention

Gravity trigger conditions, corruption detection, and duplicate gravity prevention.

### Requirement: Gravity trigger based on actual outcomes
The reconciler SHALL only set `NeedsGravity` to true when at least one adlist create, update, or successful delete (non-404) operation completed. Phantom deletes (404) SHALL NOT trigger gravity.

#### Scenario: All deletes return 404
- **WHEN** the reconciler's diff includes 3 adlist deletes but all return 404
- **THEN** `NeedsGravity` SHALL be false and gravity SHALL NOT be triggered

#### Scenario: Mix of creates and phantom deletes
- **WHEN** the reconciler creates 1 adlist and 2 deletes return 404
- **THEN** `NeedsGravity` SHALL be true because a create succeeded

#### Scenario: Successful delete
- **WHEN** the reconciler deletes an adlist and the delete returns 200
- **THEN** `NeedsGravity` SHALL be true

### Requirement: No concurrent gravity per target
The gravity scheduler SHALL prevent concurrent gravity triggers for the same target. If gravity is already running for a target, a new trigger request SHALL be skipped and logged at WARN.

#### Scenario: Scheduled gravity while reconcile gravity runs
- **WHEN** the reconcile path triggers gravity for target "pizero" and the scheduler attempts a scheduled trigger for "pizero"
- **THEN** the scheduler SHALL skip the second trigger and log a warning

#### Scenario: Independent targets
- **WHEN** gravity is running for "mikrotik" and a trigger arrives for "pizero"
- **THEN** the scheduler SHALL run gravity for "pizero" (different target, no conflict)

### Requirement: Gravity corruption detection
The reconciler SHALL detect gravity database corruption errors ("no such table") and skip reconciliation for the affected target. The target SHALL be marked as unreachable/degraded.

#### Scenario: Corrupted gravity database
- **WHEN** a list operation fails with "no such table: group" after retry exhaustion
- **THEN** the reconciler SHALL log "gravity database corrupted" at ERROR and skip the target

#### Scenario: Transient database error resolves
- **WHEN** a list operation fails with "Database not available" but succeeds on retry
- **THEN** the reconciler SHALL proceed normally (this is transient, not corruption)
