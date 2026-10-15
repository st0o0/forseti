# Config Hot Reload

## Purpose

Detecting config file changes in watch mode and applying a validated new config to the running process without a restart, with fallback on invalid files.

## Requirements

### Requirement: Config file change detection via mtime polling

Before each reconcile or sync tick, the watch loop SHALL stat the config file and compare its modification time against the last successful load time. If the mtime is newer, a reload SHALL be attempted.

#### Scenario: Config file modified between ticks

- **WHEN** the config file's mtime is newer than the last load time at the start of a tick
- **THEN** the system SHALL call `config.Load()` on the file path and attempt to swap the active config

#### Scenario: Config file unchanged between ticks

- **WHEN** the config file's mtime is equal to or older than the last load time
- **THEN** the system SHALL skip the reload and proceed with the current config

### Requirement: Validated reload with fallback

A config reload SHALL only replace the active config if the new file parses and validates successfully. On failure, the previous config SHALL remain active. A given file modification that fails to load SHALL be reported (error log and failure metric) once; the system SHALL NOT retry or re-report the same modification on every tick, and SHALL attempt a reload again only after the file's mtime changes.

#### Scenario: Valid config reload

- **WHEN** the config file has changed and `config.Load()` returns a valid config
- **THEN** the active config pointer SHALL be replaced with the new config and a success log message SHALL be emitted

#### Scenario: Invalid config reload

- **WHEN** the config file has changed and `config.Load()` returns an error
- **THEN** the active config pointer SHALL NOT change, an error log message SHALL be emitted, and reconciliation SHALL continue with the previous config

#### Scenario: Invalid config not re-reported

- **WHEN** a reload of a given file modification failed and the file is not modified again
- **THEN** subsequent ticks SHALL NOT attempt to reload it, SHALL NOT emit another error log, and SHALL NOT increment `forseti_config_reload_total{result="failure"}` again

#### Scenario: Fixed config after failure

- **WHEN** a reload failed and the file is then modified into a valid config
- **THEN** the next tick SHALL reload it successfully

### Requirement: Ticker interval update on reload

When the active config is swapped successfully and the reconcile (or sync) interval has changed, the ticker SHALL be reset to the new interval.

#### Scenario: Interval changed after reload

- **WHEN** a successful reload produces a config with a different reconcile interval
- **THEN** the ticker SHALL be reset to the new interval value

#### Scenario: Interval unchanged after reload

- **WHEN** a successful reload produces a config with the same reconcile interval
- **THEN** the ticker SHALL NOT be reset

### Requirement: Reload metrics

The metrics server SHALL expose a `forseti_config_reload_total` counter with a `result` label having values `success` or `failure`.

#### Scenario: Successful reload increments counter

- **WHEN** a config reload succeeds
- **THEN** `forseti_config_reload_total{result="success"}` SHALL be incremented by 1

#### Scenario: Failed reload increments counter

- **WHEN** a config reload fails validation
- **THEN** `forseti_config_reload_total{result="failure"}` SHALL be incremented by 1

### Requirement: Reload propagates to all runtime components

A successful reload SHALL apply the new target set and target settings to every runtime component in both config and sync mode: session pool API configuration, stats collector and new-domains collector targets, scrape timeout, and the gravity scheduler. Gravity schedules SHALL be recomputed for added or changed targets and removed for deleted targets.

#### Scenario: Target added in sync mode

- **WHEN** a new replica is added to the config in sync mode and the reload succeeds
- **THEN** the stats collector SHALL scrape the new replica and its API settings (timeout, max_concurrent) SHALL be applied by the pool

#### Scenario: Gravity schedule changed

- **WHEN** a target's `gravity.schedule` changes from `0 3 * * 0` to `0 4 * * *` and the reload succeeds
- **THEN** the scheduler SHALL use the new schedule to compute the next run for that target

#### Scenario: Target added with gravity on change

- **WHEN** a target is added on reload and a later adlist change triggers gravity for it
- **THEN** the scheduler SHALL run gravity for that target instead of reporting an unknown target
