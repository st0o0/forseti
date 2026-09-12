## Context

Forseti's watch mode runs a monolithic `reconcileAll` function that handles session management, settings reconciliation, content reconciliation, gravity triggering, and metrics recording — all inline, sequentially per target, with no cross-cycle state. The function is untestable because it directly depends on concrete types (session pool, metrics server, gravity scheduler) with no interfaces.

Recent fixes (error-path-hardening, gravity-loop-fix) added workarounds within this structure. The architecture needs restructuring to make the orchestration testable and maintainable.

## Goals / Non-Goals

**Goals:**
- Make the reconcile orchestration fully unit-testable with mocked dependencies
- Give each target its own lifecycle state (health, backoff, error tracking)
- Hot-reload that properly handles target add/remove without restart
- Exponential backoff for failing targets
- Reduce `reconcileAll` / watch loop to ~30 lines of worker-map management

**Non-Goals:**
- Parallel target processing (sequential is fine for 2-3 targets)
- Async gravity (keep synchronous behind interface, can be made async later)
- Changing the reconcile algorithm itself (Plan/Apply/diff logic stays)
- Changing the config schema or CLI interface

## Decisions

### 1. Worker per target, not per cycle

**Decision**: Create `internal/worker.TargetWorker` struct that persists across cycles. Each worker owns its target config, health state, and backoff timer. The watch loop maintains a `map[string]*TargetWorker`.

**Why not stateless per-cycle?** Cross-cycle state (consecutive failures, backoff, gravity corruption) is essential for backoff and health tracking. A per-cycle approach would need external state tracking, which is what reconcileAll already fails at.

### 2. Four interfaces for dependencies

**Decision**: Define four interfaces that the worker depends on:

```go
type SessionManager interface {
    Get(target config.Target) (*pihole.Client, error)
    Invalidate(name string)
}

type SettingsReconciler interface {
    Diff(settings *config.Settings, client *pihole.Client) (*SettingsDiffReport, error)
    Apply(name string, settings *config.Settings, client *pihole.Client) (*SettingsApplyReport, error)
}

type ContentReconciler interface {
    Apply(rt *config.ResolvedTarget, client *pihole.Client, opts ReconcileOptions) (*ApplyReport, error)
}

type GravityTrigger interface {
    TriggerNow(target string, reason TriggerReason) error
}
```

**Why four, not fewer?** Each represents a distinct concern with distinct test scenarios. Merging them would create fat interfaces that are harder to mock.

**Why not more?** Metrics recording could be a fifth interface, but the Recorder interface already exists in the gravity package. Reuse it and add a broader `Recorder` that covers reconcile metrics too.

### 3. Health state machine

**Decision**: Three health states with clear transitions:

```
                    ┌────────────────────┐
      success       │                    │   success
    ┌──────────────▶│     healthy        │◀──────────┐
    │               │                    │           │
    │               └────────┬───────────┘           │
    │                        │ failure               │
    │               ┌────────▼───────────┐           │
    │               │                    │           │
    │               │     degraded       │───────────┘
    │               │  (backoff active)  │
    │               └────────┬───────────┘
    │                        │ gravity corrupted
    │               ┌────────▼───────────┐
    │               │                    │
    └───────────────│      down          │
      success       │  (manual/gravity   │
                    │   recovery needed) │
                    └────────────────────┘
```

- **healthy**: Normal reconcile every cycle
- **degraded**: Exponential backoff (skip N cycles), auto-recovers on success
- **down**: Gravity DB corrupted or persistent failure. Still attempts reconcile (at max backoff interval) but logs differently. Auto-recovers if reconcile succeeds.

### 4. Exponential backoff formula

**Decision**: `backoffUntil = now + min(consecutiveFails * interval, 30min)`. Linear scaling capped at 30 minutes.

**Why linear, not exponential?** With a 5min cycle interval, exponential (2^n * interval) reaches 2.5 hours after just 5 failures. Linear scaling (5, 10, 15, 20, 25, 30cap) stays responsive. A truly dead target settles at checking every 30min.

### 5. Hot-reload via worker map sync

**Decision**: On config reload, compute a diff between current worker map and new resolved targets:
- Target exists in both: `worker.UpdateConfig(newResolvedTarget)`
- Target only in new config: `NewTargetWorker(...)` and add to map
- Target only in old config: `worker.Stop()` and remove from map

The gravity scheduler, collector, and session pool are NOT rebuilt. Instead, workers use interfaces that delegate to the shared pool/scheduler. Adding a target means creating a worker that uses the existing pool.

**Why not rebuild everything?** Pool and scheduler are stateful (active sessions, cron timers). Rebuilding would drop all sessions and reset gravity schedules. Worker-level config update preserves running state.

### 6. Recorder interface for metrics

**Decision**: Define a `Recorder` interface that covers all metrics the worker needs to report:

```go
type Recorder interface {
    RecordReconcile(result ReconcileResult)
    RecordChanges(target string, changes map[string]int)
    UpdateDrift(target string, drift map[string]bool)
    UpdateSettingsDrift(target string, drift map[string]bool)
    MarkTargetUnreachable(target string)
    MarkTargetReachable(target string)
}
```

The existing metrics.Server implements this interface. Tests use a mock.

## Risks / Trade-offs

- **More indirection** → Worker adds a layer between main and the reconcile logic. Justified by testability gains. The actual reconcile algorithm (Plan/Apply/diff) is unchanged.
- **State management complexity** → Workers have mutable state (health, backoff). Mitigated by keeping state transitions simple and well-tested.
- **Hot-reload edge cases** → Target rename (same URL, different name) looks like remove+add. Acceptable — the new worker will re-login.

## Open Questions

- Should the collector (metrics scraping) also be part of the worker, or stay separate? Currently it runs independently with its own timer. Making it worker-owned would unify lifecycle management but adds scope.
