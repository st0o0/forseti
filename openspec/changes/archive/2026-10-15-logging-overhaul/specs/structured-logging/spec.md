## MODIFIED Requirements

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

## ADDED Requirements

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
