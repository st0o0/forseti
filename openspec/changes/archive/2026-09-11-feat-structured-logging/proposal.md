## Why

Forseti uses `log.Printf` throughout (30+ call sites across 5 packages) with no log levels, no structured output, and no way to control verbosity. In production containers this means noisy text logs that are hard to filter and cannot be ingested by structured log aggregators (Loki, CloudWatch, etc.). Operators have no way to suppress debug-level output or switch to JSON for machine parsing.

## What Changes

- **Replace `log.Printf` with `log/slog`**: Migrate all logging to Go's stdlib structured logger, using `slog.Info`, `slog.Warn`, `slog.Error`, and `slog.Debug` with key-value attributes.
- **Configurable log level**: Add a `log_level` config field (debug, info, warn, error) defaulting to `info`.
- **Configurable log format**: Add a `log_format` config field (text, json) defaulting to `text`. JSON mode emits one JSON object per log line.
- **Contextual attributes**: Attach `target`, `component`, and `duration` as structured fields instead of embedding them in format strings like `[%s]`.

## Capabilities

### New Capabilities

- `structured-logging`: Structured logging with configurable level and format using `log/slog`

### Modified Capabilities

_(none)_

## Impact

- `internal/config/config.go`: new `log_level` and `log_format` fields
- `cmd/forseti/main.go`: slog handler initialization, all log calls migrated
- `internal/sync/syncer.go`: 16 log calls migrated
- `internal/gravity/scheduler.go`: 5 log calls migrated
- `internal/collector/collector.go`: 5 log calls migrated
- Zero new dependencies (stdlib `log/slog`)
