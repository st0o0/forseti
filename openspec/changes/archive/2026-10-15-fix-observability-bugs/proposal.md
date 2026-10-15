## Why

Issue #23 asked for better visibility into sync mode. Investigating it showed that several of Forseti's existing observability signals are wrong, not just missing: sync mode reports every run as successful even when writes fail, target health and reachability gauges go stale or flap, and hot-reload silently skips parts of the runtime. These bugs must be fixed before the logging overhaul builds on top of them.

## What Changes

- **Sync run reporting**: per-item write failures in sync mode are counted; a run with item failures is recorded as `partial`, a run that cannot load replica state is recorded as `error` (and the session is invalidated). Sync mode passes per-resource change counts to `forseti_reconcile_changes_total`.
- **New run status `partial`**: `forseti_reconcile_runs_total{status}` gains the value `partial` (both modes) for runs that reached the target but had item-level failures. **BREAKING** for alerts/dashboards that assume only `success`/`error`.
- **Sync honours the API concurrency gate**: replica and primary access acquire the per-target gate like every other subsystem.
- **Health gauge accuracy**: the config-mode worker sets `forseti_target_health` after every cycle, on every failure path (readiness, settings, content), not only on session errors and success.
- **Reachability semantics**: `forseti_target_reachable` reflects only whether the target answered (session + readiness), never the run outcome. A partial or failed content run no longer sets it to 0.
- **Hot-reload completeness**:
  - sync mode reload updates pool API config, stats/new-domains collector targets and scrape timeout (as config mode already does);
  - the gravity scheduler supports `UpdateTargets`, so added/removed targets and changed schedules take effect on reload in both modes.
- **Reload failure noise**: an invalid config file is reported once per file modification instead of on every tick (metric still increments once per failed attempt-per-mtime).
- **Configurable sync DNS purge**: new `sync.local_dns_purge` (default `true`, current mirror behaviour). When `false`, sync only adds local DNS records on replicas and never deletes them. Pi-hole local DNS entries carry no comment field, so the managed-marker invariant cannot apply; this option makes the exception explicit.

## Capabilities

### New Capabilities
- `sync-run-reporting`: how a sync run against a replica is classified (ok/partial/error), recorded in metrics, gated, and how local DNS is mirrored.

### Modified Capabilities
- `metrics`: `forseti_reconcile_runs_total` status values gain `partial`; `forseti_target_reachable` semantics clarified; `forseti_target_health` updated after every cycle.
- `target-worker`: health gauge recorded on every cycle outcome; item-level errors yield a partial result.
- `config-hot-reload`: reload applies to all runtime components in both modes (pool, collectors, gravity scheduler); failed reloads are reported once per modification.

## Impact

- `internal/sync/syncer.go` (+ tests): error accounting, change maps, gate, DNS purge flag.
- `internal/config/config.go`: `SyncConfig.LocalDNSPurge *bool`.
- `internal/metrics/metrics.go`: `ReconcileResult` status, reachable handling.
- `internal/worker/worker.go`: health recording on all failure paths, partial status.
- `internal/gravity/scheduler.go`: `UpdateTargets`.
- `cmd/forseti/main.go`: reload wiring in sync mode, gravity reload, once-per-mtime failure reporting.
- README: `sync.local_dns_purge`, `partial` status.
