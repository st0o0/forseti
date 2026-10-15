## ADDED Requirements

### Requirement: Primary snapshot per cycle
Each sync cycle SHALL load the primary's state once, as a run of the primary target with its own health and backoff, and SHALL use that snapshot for every replica in the cycle. If the primary run fails or is in backoff, every replica run in that cycle SHALL be classified as `skipped`, SHALL NOT change replica health or backoff, and SHALL NOT increment `forseti_reconcile_runs_total`.

#### Scenario: Primary unreachable
- **WHEN** the primary cannot be reached during a sync cycle with two replicas
- **THEN** the primary run SHALL be recorded as `error`, both replica runs SHALL be `skipped`, and `cycle complete` SHALL report `failed=1 skipped=2`

#### Scenario: Primary state loaded once
- **WHEN** a sync cycle runs with three replicas
- **THEN** the primary's state SHALL be read once for that cycle

### Requirement: Replica health and backoff
Replicas in sync mode SHALL use the same health state machine and backoff as config-mode targets. A replica in backoff SHALL be skipped until its backoff expires.

#### Scenario: Replica outage
- **WHEN** a replica fails three consecutive sync runs with a 5-minute interval
- **THEN** it SHALL be degraded and skipped until its 15-minute backoff expires
