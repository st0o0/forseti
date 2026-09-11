## Why

In watch mode, the config YAML is loaded once at startup. Any change to adlists, domains, groups, clients, targets, or intervals requires a full process restart. This is painful for GitOps workflows where a config change is pushed and should take effect automatically without container recreation.

## What Changes

- **File-change detection**: Watch the config YAML for modifications during the reconcile/sync loop
- **Validated reload**: Re-run `config.Load()` on change detection; only swap the active config if the new file parses and validates successfully. Log and keep the old config on error.
- **Subsystem refresh**: Update the reconcile/sync ticker interval if it changed, and pass the new config to subsequent reconcile/sync iterations
- **Reload metric**: Expose a `forseti_config_reload_total` counter (success/failure labels) so operators can observe reload events

## Capabilities

### New Capabilities

- `config-hot-reload`: File-change detection and validated config swap in watch mode

### Modified Capabilities

_(none)_

## Impact

- `cmd/forseti/main.go`: watch loop gains config reload logic
- `internal/config/config.go`: no changes needed — `Load()` is already stateless and re-entrant
- `internal/metrics/server.go`: new reload counter metric
- New dependency consideration: `fsnotify` vs poll-based (design decision)
