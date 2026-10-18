# Lifecycle Events

## Purpose

Defines the stable set of log events Forseti emits over its lifecycle, the level policy behind them, and the rule that state-like conditions are logged on transitions rather than on every observation.

## Requirements

### Requirement: Level policy
Log levels SHALL be used as follows:
- `error`: something Forseti owns failed and the intended outcome was not reached (failed run, target down, failed gravity, failed config reload, server cannot bind).
- `warn`: the outcome was reached in a degraded way or a problem is expected to self-heal (partial run, target degraded, target unreachable, config warnings).
- `info`: lifecycle events and state changes (startup, config summary, connections, runs with changes, cycle summaries, recoveries, reloads, gravity, shutdown).
- `debug`: per-request, per-item, retry and no-op detail.

#### Scenario: Retry attempts are debug
- **WHEN** a transient API error is retried
- **THEN** the retry SHALL be logged at debug level and only the final outcome SHALL appear at a higher level

### Requirement: Stable event messages
Lifecycle events SHALL use fixed, lowercase message strings without interpolated values; all variable data SHALL be in attributes. The following events SHALL exist with at least the listed attributes:

| Message | Level | Attributes |
|---|---|---|
| `forseti starting` | info | version, mode, config |
| `config loaded` | info | mode, targets, interval; sync: primary, replicas, resources; config: resource counts |
| `config warning` | warn | target, reason |
| `http server listening` | info | addr, endpoints |
| `target connected` | info | target |
| `target unreachable` | warn | target, source, error |
| `run finished` | see run outcome requirement | target, status, duration, per-resource counts |
| `cycle complete` | info | mode, targets, ok, partial, failed, skipped, changes, duration, next_run |
| `target degraded` | warn | target, consecutive_failures, retry_at, error |
| `target down` | error | target, consecutive_failures, error |
| `target recovered` | info | target, failures, downtime |
| `config reloaded` | info | changed, targets_added, targets_removed |
| `config reload failed` | error | error |
| `gravity scheduled` | info | target, schedule, next_run |
| `gravity started` / `gravity finished` | info (finished with error: error) | target, reason, duration |
| `shutting down` | info |  - |
| `stopped` | info | uptime |

The `new_blocked_domain` event keeps its existing name for compatibility.

#### Scenario: Startup summary in sync mode
- **WHEN** Forseti starts in sync mode with one primary and two replicas
- **THEN** the log SHALL contain `forseti starting` with the version and `config loaded` naming the primary, both replicas, the interval and the synced resources

#### Scenario: Heartbeat on idle cycles
- **WHEN** a sync or reconcile cycle completes with no changes
- **THEN** a `cycle complete` event SHALL still be logged at info level with `changes=0` and the `next_run` time

### Requirement: Run outcome logging
Every run of a target SHALL produce exactly one `run finished` event. Its level SHALL be: info when the run succeeded with changes; debug when it succeeded without changes or was skipped; warn when partial; error when failed. Failed and partial runs SHALL include the error(s) and the stage at which the run failed.

#### Scenario: Partial sync run
- **WHEN** one adlist write fails on replica `pihole-2`
- **THEN** a warn-level `run finished` event SHALL be logged with `target=pihole-2`, `status=partial`, the failed count and the sanitized error

### Requirement: Transition-based state logging
Reachability and health SHALL be logged only when they change. A target that stays unreachable SHALL NOT produce a new `target unreachable` event per observation, regardless of which subsystem (worker, collector, gravity, new-domains) observes it. Health transitions SHALL produce exactly one `target degraded`, `target down` or `target recovered` event.

#### Scenario: Collector on a down target
- **WHEN** a target is unreachable for one hour while Prometheus scrapes every 15 seconds
- **THEN** exactly one `target unreachable` event SHALL be logged for that target, and one `target connected` event when it answers again

#### Scenario: Recovery reported once
- **WHEN** a degraded target succeeds after 3 failed runs
- **THEN** one `target recovered` event SHALL be logged with `failures=3`

### Requirement: Config reload reports what changed
A successful reload SHALL log `config reloaded` listing the changed top-level sections and the targets added and removed. A failed reload SHALL log `config reload failed` once per file modification.

#### Scenario: Interval and target change
- **WHEN** the reloaded config changes `sync.interval` and adds target `pihole-3`
- **THEN** the event SHALL include `sync` in `changed` and `pihole-3` in `targets_added`
