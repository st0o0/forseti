## ADDED Requirements

### Requirement: Liveness endpoint
The metrics HTTP server SHALL serve `GET /healthz`. It SHALL return 200 with `{"status":"ok"}` while the watch loop is alive, and 503 with `{"status":"stalled"}` when the loop has not completed a cycle within `max(3 × interval, interval + 2m)`. The result SHALL NOT depend on Pi-hole reachability.

#### Scenario: Pi-hole down, Forseti alive
- **WHEN** every Pi-hole target is unreachable but the watch loop keeps running cycles
- **THEN** `/healthz` SHALL return 200

#### Scenario: Loop stuck
- **WHEN** no cycle has completed for longer than the stall threshold
- **THEN** `/healthz` SHALL return 503

### Requirement: Healthcheck probes the liveness endpoint
The `forseti healthcheck` command SHALL perform an HTTP GET against `http://127.0.0.1:<port>/healthz` with a 3-second timeout and exit 0 on a 200 response and 1 otherwise. The port SHALL be taken from `--addr` if given, otherwise from `metrics.port` in the file passed via `--config`, otherwise 9099. The command SHALL NOT log in to any Pi-hole and SHALL NOT fail because the config file is invalid.

#### Scenario: Healthy daemon
- **WHEN** the daemon is running and `/healthz` returns 200
- **THEN** `forseti healthcheck --config /config/forseti.yml` SHALL exit 0

#### Scenario: Invalid config edit
- **WHEN** the config file was edited into an invalid state while the daemon keeps running with the previous config
- **THEN** the healthcheck SHALL still exit 0

## REMOVED Requirements

### Requirement: Healthcheck verifies target connectivity
**Reason**: Restarting Forseti does not fix an unreachable Pi-hole, and logging in every 30s per target bypasses the session pool.
**Migration**: Monitor Pi-hole reachability with `forseti_target_reachable` and `forseti_target_health`; container health now reflects Forseti's own liveness.

### Requirement: Healthcheck requires config flag
**Reason**: The probe only needs the metrics port, which has a default.
**Migration**: `--config` remains accepted (used only to read `metrics.port`); `--addr` can be used instead.

### Requirement: Healthcheck has a per-target timeout
**Reason**: The healthcheck no longer contacts targets.
**Migration**: None; the probe has a single 3-second timeout.

### Requirement: Healthcheck cleans up sessions
**Reason**: The healthcheck no longer creates sessions.
**Migration**: None.
