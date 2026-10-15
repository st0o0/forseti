## Why

Issue #23: users cannot tell from the container log whether Forseti is working. Sync mode is silent when nothing changes, there is no startup summary, target outages produce either nothing (health transitions) or the same error on every scrape (collector), messages and levels are inconsistent across packages, and lines carry no component. Sync mode also lacks the health/backoff model config mode has, so the two modes cannot log or report the same way. The Docker healthcheck logs into every Pi-hole every 30s and leaks sessions on timeout.

## What Changes

- **Logging foundation**: one logger factory with a `component` attribute on every line; log level changes on reload without rebuilding loggers; config warnings are logged with the configured format; `plan`/`apply` log to stderr; timezone data embedded so `TZ` works in the scratch image.
- **Unified run model**: both modes run each target through the same worker envelope (gate → session → readiness → mode-specific step → classify → health → record). Sync mode gains health, backoff and transition logging for the primary and every replica. If the primary fails, replica runs are `skipped`.
- **Health**: a target becomes `down` after 5 consecutive failures (or gravity corruption).
- **Event catalog**: a fixed set of lifecycle events with stable messages and keys: `forseti starting`, `config loaded`, `http server listening`, `target connected`/`target unreachable`, `run finished`, `cycle complete` (with `next_run`), `target degraded`/`target down`/`target recovered`, `config reloaded` (with what changed), gravity scheduled/started/finished, `shutting down`/`stopped`. **BREAKING** for log-based alerts: `synced`, `reconciled`, `no drift`, `starting` and others are replaced.
- **Noise control**: reachability is logged on transitions only (collector, gravity, new-domains, worker share one tracker); failed collector fetches are retried once per scrape interval; retries and per-request detail move to debug; `new_blocked_domain` is logged once per domain per window instead of every interval.
- **Accurate config-mode change counters**: failed item writes are no longer counted in `forseti_reconcile_changes_total`.
- **Redaction**: URL credentials and query values (e.g. `?token=`) are redacted in logs and errors; Pi-hole API errors are rendered from the parsed key/message and truncated; secret-bearing settings are never logged.
- **Liveness endpoint**: `/healthz` reports whether Forseti's loop is alive; `forseti healthcheck` probes it instead of logging into Pi-holes. **BREAKING**: container health no longer reflects Pi-hole reachability.

## Capabilities

### New Capabilities
- `lifecycle-events`: the stable log event catalog, level policy and transition-based reporting.
- `log-redaction`: rules for keeping credentials, tokens and raw bodies out of logs and errors.

### Modified Capabilities
- `structured-logging`: component attribute, level reload, early/consistent formatting, stderr for one-shot commands, timezone.
- `target-worker`: shared envelope for both modes, down after 5 consecutive failures, logged transitions.
- `sync-run-reporting`: sync runs through workers; primary failure yields `skipped` replica runs; health/backoff for sync targets.
- `healthcheck-depth`: replaced by a liveness probe against `/healthz`.
- `new-blocked-domains`: each newly blocked domain is logged once per window.
- `metrics`: change counters count applied changes only.

## Impact

- New packages: `internal/logging` (+ `logtest`), `internal/run`, `internal/targetstate`, `internal/redact`.
- Reworked: `internal/worker`, `internal/sync`, `cmd/forseti/main.go`, `internal/collector`, `internal/gravity`, `internal/session`, `internal/pihole` (request logging, `APIError`), `internal/reconcile` (item errors), `internal/config` (warnings, reload diff), `internal/metrics` (`/healthz`).
- Dockerfile `HEALTHCHECK` keeps its command line; behaviour changes.
- README: logging section with event catalog; healthcheck semantics.
