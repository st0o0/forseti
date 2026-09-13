## ADDED Requirements

### Requirement: Per-target API concurrency gate
The session pool SHALL provide a per-target semaphore that limits the number of concurrent API calls to each Pi-hole instance. The semaphore capacity SHALL equal the target's configured `max_concurrent` value. All subsystems (reconciler, collector, gravity) MUST acquire a slot before making API calls to a target and release it when done.

#### Scenario: Concurrent calls within limit
- **WHEN** `max_concurrent` is 4 for target "mikrotik" and 3 goroutines each acquire a slot
- **THEN** all 3 SHALL proceed without blocking

#### Scenario: Concurrent calls at limit
- **WHEN** `max_concurrent` is 1 for target "pizero" and the reconciler holds the slot
- **THEN** any other subsystem calling `Acquire` SHALL block until the reconciler releases

#### Scenario: TryAcquire succeeds when slot available
- **WHEN** `max_concurrent` is 2 for a target and 1 slot is in use
- **THEN** `TryAcquire` SHALL return true and the caller proceeds

#### Scenario: TryAcquire fails when full
- **WHEN** `max_concurrent` is 1 for target "pizero" and the slot is held
- **THEN** `TryAcquire` SHALL return false without blocking

### Requirement: Gate lifecycle follows pool lifecycle
The gate semaphores SHALL be created when a target is first accessed via the pool. When the pool is closed, all gate state SHALL be released. When a target's config changes `max_concurrent` on hot-reload, the gate SHALL be recreated with the new capacity.

#### Scenario: Gate created on first access
- **WHEN** a target has no existing gate and `Acquire` is called
- **THEN** the pool SHALL create a semaphore with capacity from the target's `max_concurrent` config

#### Scenario: Gate capacity updated on hot-reload
- **WHEN** a target's `max_concurrent` changes from 4 to 1 during hot-reload
- **THEN** the pool SHALL replace the gate with a new semaphore of capacity 1 after all current slots are released

### Requirement: Release is idempotent for unknown targets
Calling `Release` for a target with no active gate or for an unknown target name SHALL be a no-op and SHALL NOT panic.

#### Scenario: Release unknown target
- **WHEN** `Release("nonexistent")` is called
- **THEN** the call SHALL return without error or panic
