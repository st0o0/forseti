## ADDED Requirements

### Requirement: Config file change detection via mtime polling

Before each reconcile or sync tick, the watch loop SHALL stat the config file and compare its modification time against the last successful load time. If the mtime is newer, a reload SHALL be attempted.

#### Scenario: Config file modified between ticks

- **WHEN** the config file's mtime is newer than the last load time at the start of a tick
- **THEN** the system SHALL call `config.Load()` on the file path and attempt to swap the active config

#### Scenario: Config file unchanged between ticks

- **WHEN** the config file's mtime is equal to or older than the last load time
- **THEN** the system SHALL skip the reload and proceed with the current config

### Requirement: Validated reload with fallback

A config reload SHALL only replace the active config if the new file parses and validates successfully. On failure, the previous config SHALL remain active.

#### Scenario: Valid config reload

- **WHEN** the config file has changed and `config.Load()` returns a valid config
- **THEN** the active config pointer SHALL be replaced with the new config and a success log message SHALL be emitted

#### Scenario: Invalid config reload

- **WHEN** the config file has changed and `config.Load()` returns an error
- **THEN** the active config pointer SHALL NOT change, an error log message SHALL be emitted, and reconciliation SHALL continue with the previous config

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
