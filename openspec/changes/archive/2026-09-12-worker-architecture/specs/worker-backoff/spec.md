# Worker Backoff

Exponential backoff for failing targets with health state transitions.

## ADDED Requirements

### Requirement: Exponential backoff on failure
The worker SHALL implement backoff that increases with consecutive failures. The backoff formula SHALL be `min(consecutiveFails * reconcileInterval, 30 minutes)`. During backoff, `ShouldSkip()` SHALL return true and the watch loop SHALL skip the worker's reconcile cycle.

#### Scenario: First failure no backoff
- **WHEN** a worker fails for the first time
- **THEN** the next cycle SHALL still attempt reconciliation (backoff = 1 * interval, which is the normal interval)

#### Scenario: Repeated failures increase backoff
- **WHEN** a worker has failed 3 consecutive times with a 5-minute interval
- **THEN** the backoff SHALL be 15 minutes (3 * 5min), skipping 2 cycles

#### Scenario: Backoff capped at 30 minutes
- **WHEN** a worker has failed 10 consecutive times with a 5-minute interval
- **THEN** the backoff SHALL be 30 minutes (capped), not 50 minutes

#### Scenario: Success resets backoff
- **WHEN** a worker succeeds after 5 consecutive failures
- **THEN** consecutive failures SHALL reset to 0 and backoff SHALL clear immediately

### Requirement: Backoff does not prevent recovery
The worker SHALL always attempt reconciliation after the backoff period expires. Backoff SHALL never permanently disable a target.

#### Scenario: Target recovers after long outage
- **WHEN** a target has been down for 2 hours and comes back online
- **THEN** the worker SHALL attempt reconciliation on the next cycle after backoff expires and SHALL recover to healthy on success
