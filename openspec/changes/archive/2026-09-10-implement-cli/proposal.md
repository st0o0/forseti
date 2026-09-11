## Why

All internal packages (config, pihole, reconcile, metrics) are implemented. The CLI entry point (`cmd/forseti/main.go`) currently has stubs for plan/apply/watch. This change wires everything together to make forseti a functional tool.

## What Changes

- Implement `plan` command: load config, run Plan against all targets, display diff report
- Implement `apply` command: load config, run Apply against all targets, display results
- Implement `watch` command: daemon mode with periodic reconcile + metrics server
- Add `--config` flag parsing for all commands
- Proper error handling and exit codes
- Signal handling for graceful shutdown in watch mode

## Capabilities

### New Capabilities
None. Wires existing implementations to the CLI stubs.

### Modified Capabilities
None.

## Impact
- `cmd/forseti/main.go` — full implementation replacing stubs
- No new dependencies (uses stdlib `flag`, `os/signal`)
