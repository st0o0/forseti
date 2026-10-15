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

Log messages SHALL use structured key-value attributes instead of embedding context in format strings. Every log line SHALL carry a `component` attribute naming the subsystem that emitted it (e.g. `main`, `worker`, `sync`, `session`, `pihole`, `collector`, `gravity`, `reconcile`, `config`, `http`). Target-scoped operations SHALL include a `target` attribute, including per-request debug logs of the Pi-hole client. Duration-bearing messages SHALL include a `duration` attribute.

#### Scenario: Reconcile completion log

- **WHEN** a reconciliation completes for target "pihole1" in 250ms
- **THEN** the log entry SHALL contain `target=pihole1` and `duration=250ms` as structured attributes rather than inline `[pihole1] reconciled in 250ms`

#### Scenario: Error log with target context

- **WHEN** a session error occurs for target "pihole2"
- **THEN** the log entry SHALL be at error level with `target=pihole2` and `error=<message>` as structured attributes

#### Scenario: Component on every line

- **WHEN** the gravity scheduler logs a finished gravity run
- **THEN** the entry SHALL contain `component=gravity`

#### Scenario: API request debug log

- **WHEN** debug logging is enabled and the client performs a request against target "pihole1"
- **THEN** the entry SHALL contain `target=pihole1`, the method, the path, the response status and the duration

### Requirement: No residual stdlib log usage

After migration, the codebase SHALL NOT import `log` for logging purposes. All logging SHALL use `log/slog`.

#### Scenario: No log package imports remain

- **WHEN** the migration is complete
- **THEN** grepping for `"log"` imports (excluding `"log/slog"`) SHALL return zero results in application code

### Requirement: Log level reload without restart

A config reload that changes `log_level` SHALL apply the new level to all subsequent log output, including loggers created before the reload. A change to `log_format` SHALL require a restart and SHALL be reported with a warning on reload.

#### Scenario: Level raised to debug on reload

- **WHEN** `log_level` changes from `info` to `debug` and the reload succeeds
- **THEN** debug messages from all components SHALL appear without restarting

#### Scenario: Format change on reload

- **WHEN** `log_format` changes from `text` to `json` on reload
- **THEN** the format SHALL remain `text` and a warning SHALL state that a restart is required

### Requirement: Consistent output from the first line

Log output SHALL use the configured format and level from the first line after the config is loaded, including warnings produced while loading and resolving the config. Errors that occur before the config can be loaded SHALL be logged in text format.

#### Scenario: Config warning in JSON format

- **WHEN** `log_format` is `json` and an exclude entry is not found during config resolution
- **THEN** the resulting warning SHALL be a JSON log line

### Requirement: One-shot command output separation

The `plan` and `apply` commands SHALL write log output to stderr so that their stdout contains only the command's result.

#### Scenario: Piping plan output

- **WHEN** `forseti plan` is run with stdout redirected to a file
- **THEN** the file SHALL contain only the plan and no log lines

### Requirement: Timezone support

Timestamps in logs and gravity cron schedules SHALL honour the `TZ` environment variable in the published container image.

#### Scenario: TZ set in container

- **WHEN** the container runs with `TZ=Europe/Berlin`
- **THEN** log timestamps SHALL carry the Europe/Berlin offset and gravity schedules SHALL be evaluated in that timezone
