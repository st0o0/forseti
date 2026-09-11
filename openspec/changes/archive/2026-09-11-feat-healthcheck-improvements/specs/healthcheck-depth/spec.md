## ADDED Requirements

### Requirement: Healthcheck verifies target connectivity

The `forseti healthcheck` command SHALL load the config file and attempt a login to each configured Pi-hole target. It SHALL exit 0 only when all targets respond successfully.

#### Scenario: All targets reachable

- **WHEN** the healthcheck runs with a valid config and all Pi-hole targets respond to Login within the timeout
- **THEN** the command SHALL print a status line per target (`[<name>] ok`) and exit with code 0

#### Scenario: One target unreachable

- **WHEN** the healthcheck runs and one target fails Login (connection refused, auth error, or timeout)
- **THEN** the command SHALL print `[<name>] error: <reason>` for the failing target, print `[<name>] ok` for healthy targets, and exit with code 1

#### Scenario: All targets unreachable

- **WHEN** the healthcheck runs and no target responds
- **THEN** the command SHALL print error lines for each target and exit with code 1

### Requirement: Healthcheck requires config flag

The healthcheck subcommand SHALL require the `--config <path>` flag, consistent with `plan`, `apply`, and `watch`.

#### Scenario: Missing config flag

- **WHEN** `forseti healthcheck` is invoked without `--config`
- **THEN** the command SHALL print an error message and exit with code 1

### Requirement: Healthcheck has a per-target timeout

Each target probe SHALL be bounded by a 5-second timeout. If a target does not respond within the timeout, it SHALL be reported as unreachable.

#### Scenario: Target exceeds timeout

- **WHEN** a Pi-hole target does not respond within 5 seconds
- **THEN** the command SHALL report that target as a timeout error and continue probing remaining targets

### Requirement: Healthcheck cleans up sessions

The healthcheck SHALL close each Pi-hole client session immediately after probing, regardless of success or failure.

#### Scenario: Session cleanup after probe

- **WHEN** the healthcheck completes probing a target
- **THEN** the client session SHALL be closed before probing the next target
