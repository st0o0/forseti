## Why

The `forseti healthcheck` subcommand prints `"ok"` without verifying anything. It does not load the config, connect to any Pi-hole target, or check the session pool. In Kubernetes or Docker health probes this gives false confidence — a container reports healthy while the Pi-hole API is unreachable or sessions are expired. A meaningful healthcheck should verify that forseti can reach its targets.

## What Changes

- **Deep healthcheck**: Load the config file and attempt a login to each Pi-hole target. Exit 0 only when all targets are reachable; exit 1 with details on failure.
- **`--config` flag for healthcheck**: The healthcheck subcommand will accept `--config <path>` to know which targets to probe.
- **Timeout**: Each target probe is bounded by a short timeout (default 5s) so the healthcheck itself does not hang.
- **Structured output**: Print per-target status lines (`[target] ok` or `[target] error: ...`) so operators can diagnose which target is down.

## Capabilities

### New Capabilities

- `healthcheck-depth`: Deep healthcheck that verifies Pi-hole target connectivity per target with timeout and structured output.

### Modified Capabilities

_(none)_

## Impact

- `cmd/forseti/main.go`: healthcheck case gains config loading, target probing, and exit code logic
- `internal/pihole/client.go`: no changes needed — `Login()` already serves as a connectivity check
- Dockerfile HEALTHCHECK: may need `--config` path added to the CMD
