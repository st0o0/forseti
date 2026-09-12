## Why

The 108-line `reconcileAll` function is a God Function with 9 responsibilities, no cross-cycle state, and no testable interfaces. Every production bug so far (stale sessions, gravity loops, phantom deletes, DB corruption) was only caught in production because the orchestration logic cannot be unit-tested. Hot-reload doesn't rebuild the gravity scheduler or collector, so adding/removing targets requires a restart. Failing targets get the full reconcile attempt every cycle with no backoff, wasting time and generating noise.

## What Changes

- **TargetWorker struct**: Each target gets its own worker with lifecycle state (health, consecutive failures, backoff, gravity corruption flag). Workers own their session, reconciliation, and gravity trigger via interfaces.
- **Worker-based watch loop**: `reconcileAll` replaced by a worker map. On each tick: sync workers with config, skip workers in backoff, call `worker.Reconcile()`. Watch loop drops from ~120 to ~30 lines.
- **Exponential backoff**: Workers track consecutive failures and skip cycles with exponential backoff (capped at max interval). Success resets the counter.
- **Hot-reload with worker sync**: Config reload syncs the worker map — existing workers get `UpdateConfig()`, new targets get new workers, removed targets get `Stop()`. Gravity scheduler and collector rebuild automatically.
- **Interface-driven dependencies**: SessionManager, Reconciler, GravityTrigger, Recorder — all interfaces, all mockable, all testable.
- **Orchestration tests**: Full test coverage for the settings → restart → invalidate → reconcile → gravity sequence using mocked interfaces.

## Capabilities

### New Capabilities
- `target-worker`: Per-target worker with lifecycle management, health tracking, backoff, and interface-driven orchestration.
- `worker-backoff`: Exponential backoff for failing targets with health state transitions.

### Modified Capabilities
- `reconciler`: Content reconciliation exposed through a clean interface consumed by the worker.
- `config`: Hot-reload syncs worker map — targets added/removed without restart.

## Impact

- `internal/worker/` — new package with TargetWorker, interfaces, orchestration logic
- `cmd/forseti/main.go` — watch loop simplified to worker map management
- `internal/gravity/scheduler.go` — gravity trigger exposed as interface
- `internal/reconcile/` — reconciler methods exposed through interface for worker consumption
- `internal/session/` — session pool operations exposed through interface
- Existing tests unchanged, new orchestration tests added
- No config schema changes, no breaking user-facing changes
