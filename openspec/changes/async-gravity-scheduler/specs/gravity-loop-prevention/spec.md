## MODIFIED Requirements

### Requirement: No concurrent gravity per target
The gravity scheduler SHALL prevent concurrent gravity triggers for the same target. If gravity is already running for a target, a new trigger request SHALL be skipped and logged at WARN. Gravity execution SHALL NOT block the scheduler event loop — scheduled triggers and async requests for other targets MUST continue to be processed while gravity runs.

#### Scenario: Scheduled gravity while reconcile gravity runs
- **WHEN** the reconcile path triggers gravity for target "pizero" and the scheduler attempts a scheduled trigger for "pizero"
- **THEN** the scheduler SHALL skip the second trigger and log a warning

#### Scenario: Independent targets
- **WHEN** gravity is running for "mikrotik" and a trigger arrives for "pizero"
- **THEN** the scheduler SHALL run gravity for "pizero" (different target, no conflict)

#### Scenario: Event loop remains responsive during gravity
- **WHEN** gravity is running for "mikrotik" (taking 60+ seconds)
- **THEN** the scheduler SHALL continue processing cron ticks and async trigger requests for other targets without delay

#### Scenario: Graceful shutdown during gravity
- **WHEN** the context is cancelled while gravity is in-flight for a target
- **THEN** `Start` SHALL wait for in-flight gravity operations to complete before returning

## ADDED Requirements

### Requirement: Scheduler graceful shutdown
The scheduler SHALL track all in-flight gravity goroutines via a `sync.WaitGroup`. When the context is cancelled, `Start` SHALL exit the event loop and wait for all in-flight operations to finish before returning.

#### Scenario: Shutdown with in-flight gravity
- **WHEN** context is cancelled while 2 targets have gravity running
- **THEN** `Start` SHALL wait for both to complete and then return
