## Purpose

Gives operators a read-only, at-a-glance view of what Forseti did recently: per-target state, the last runs and cycle summaries, served as JSON from the metrics port and kept in memory only.

## ADDED Requirements

### Requirement: In-memory run history
Forseti SHALL keep the most recent 1440 run results per target and the most recent 1440 cycle summaries in memory. Skipped runs SHALL be kept. History SHALL be dropped when a target is removed from the config and SHALL NOT survive a restart.

#### Scenario: Bounded history
- **WHEN** 2000 runs of target `pihole-2` have completed
- **THEN** the history for `pihole-2` SHALL contain the 1440 most recent runs

#### Scenario: Target removed on reload
- **WHEN** target `pihole-3` is removed from the config and the reload succeeds
- **THEN** `pihole-3` SHALL no longer appear in `/status` or `/status/history`

### Requirement: Status endpoint
`GET /status` SHALL return JSON with: version, mode, start time, uptime, the time and outcome of the last config reload, the loop interval, the last cycle summary (ok, partial, failed, skipped, changes, duration) and the next run time, and for every configured target: name, role, health, consecutive failures, last success time, backoff end, last error (time and sanitized message) and the last run (time, status, stage, duration, per-resource counts). Targets SHALL be ordered by name.

#### Scenario: Replica down
- **WHEN** replica `pihole-2` has failed its last 3 runs
- **THEN** `/status` SHALL report `pihole-2` with health `degraded`, `consecutive_failures` 3, its last error and the time of its last success

#### Scenario: Before the first cycle
- **WHEN** `/status` is requested before any run completed
- **THEN** it SHALL return 200 with every configured target and empty last-run information

### Requirement: History endpoint
`GET /status/history` SHALL return recent entries oldest first. With `target=<name>` it SHALL return that target's runs (time, status, stage, duration, per-resource counts, sanitized errors); without it, the cycle summaries. `since` (a Go duration, default `24h`) SHALL limit entries to that window and `limit` (default 500, maximum 1440) SHALL keep only the most recent entries. An unknown target SHALL yield 404; invalid parameters SHALL yield 400.

#### Scenario: Last 24 hours of a replica
- **WHEN** `GET /status/history?target=pihole-2&since=24h` is requested
- **THEN** the response SHALL list the runs of `pihole-2` from the last 24 hours, oldest first

#### Scenario: Unknown target
- **WHEN** `GET /status/history?target=nope` is requested
- **THEN** the response SHALL be 404

### Requirement: Status responses contain no secrets
Status and history responses SHALL NOT contain passwords, session IDs or raw API response bodies, and SHALL NOT expose target URLs as fields. Error messages MAY name the host they failed to reach but SHALL be redacted as defined by `log-redaction` (no userinfo, no query values) and bounded in length.

#### Scenario: Error with tokenized adlist URL
- **WHEN** the last error of a target mentions `https://lists.example.org/x.txt?token=abc`
- **THEN** `/status` SHALL show `token=REDACTED` and SHALL NOT contain `abc`
