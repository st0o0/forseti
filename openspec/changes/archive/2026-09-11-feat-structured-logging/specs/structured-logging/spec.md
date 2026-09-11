## ADDED Requirements

### Requirement: Configurable log level

The application SHALL support a `log_level` config field accepting `debug`, `info`, `warn`, or `error`. The default SHALL be `info`. Messages below the configured level SHALL be suppressed.

#### Scenario: Debug messages suppressed at info level

- **WHEN** `log_level` is `info` and a debug-level message is emitted
- **THEN** the message SHALL NOT appear in output

#### Scenario: Error messages always shown

- **WHEN** `log_level` is `error` and an error-level message is emitted
- **THEN** the message SHALL appear in output

#### Scenario: Invalid log level rejected

- **WHEN** `log_level` is set to an invalid value like `verbose`
- **THEN** config loading SHALL return a validation error

### Requirement: Configurable log format

The application SHALL support a `log_format` config field accepting `text` or `json`. The default SHALL be `text`.

#### Scenario: JSON format output

- **WHEN** `log_format` is `json`
- **THEN** each log line SHALL be a valid JSON object containing at minimum `time`, `level`, and `msg` fields

#### Scenario: Text format output

- **WHEN** `log_format` is `text`
- **THEN** log output SHALL use slog's default text format with key=value pairs

#### Scenario: Invalid log format rejected

- **WHEN** `log_format` is set to an invalid value like `xml`
- **THEN** config loading SHALL return a validation error

### Requirement: Structured contextual attributes

Log messages SHALL use structured key-value attributes instead of embedding context in format strings. Target-scoped operations SHALL include a `target` attribute. Duration-bearing messages SHALL include a `duration` attribute.

#### Scenario: Reconcile completion log

- **WHEN** a reconciliation completes for target "pihole1" in 250ms
- **THEN** the log entry SHALL contain `target=pihole1` and `duration=250ms` as structured attributes rather than inline `[pihole1] reconciled in 250ms`

#### Scenario: Error log with target context

- **WHEN** a session error occurs for target "pihole2"
- **THEN** the log entry SHALL be at error level with `target=pihole2` and `error=<message>` as structured attributes

### Requirement: No residual stdlib log usage

After migration, the codebase SHALL NOT import `log` for logging purposes. All logging SHALL use `log/slog`.

#### Scenario: No log package imports remain

- **WHEN** the migration is complete
- **THEN** grepping for `"log"` imports (excluding `"log/slog"`) SHALL return zero results in application code
