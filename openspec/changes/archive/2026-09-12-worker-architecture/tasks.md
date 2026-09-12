## 1. Define interfaces

- [x] 1.1 Create `internal/worker/interfaces.go` with `SessionManager`, `SettingsReconciler`, `ContentReconciler`, `GravityTrigger`, `Recorder` interfaces
- [x] 1.2 Verify existing types satisfy the interfaces: `session.Pool` → `SessionManager`, `gravity.Scheduler` → `GravityTrigger`, `metrics.Server` → `Recorder`
- [x] 1.3 Add thin adapter methods where needed to make existing types satisfy the interfaces (e.g., if method signatures don't match)

## 2. TargetWorker core

- [x] 2.1 Create `internal/worker/worker.go` with `TargetWorker` struct: fields for config, state (health, consecutiveFails, lastError, lastSuccess, backoffUntil, gravityCorrupted), and interface dependencies
- [x] 2.2 Implement `NewTargetWorker(rt config.ResolvedTarget, deps Dependencies) *TargetWorker`
- [x] 2.3 Implement `Reconcile() error` — the core orchestration: get session → settings → wait ready → refresh session → content → gravity → record metrics
- [x] 2.4 Implement `UpdateConfig(rt config.ResolvedTarget)` — update resolved target config, preserve state
- [x] 2.5 Implement `ShouldSkip(now time.Time) bool` — check backoff timer
- [x] 2.6 Implement `Health() TargetHealth` — return current health state and metadata

## 3. Health and backoff

- [x] 3.1 Define `TargetHealth` type with `healthy`, `degraded`, `down` states
- [x] 3.2 Implement health transitions in Reconcile(): success → healthy (reset failures), failure → degraded (increment + backoff), gravity corruption → down
- [x] 3.3 Implement backoff calculation: `backoffUntil = now + min(consecutiveFails * interval, 30min)`
- [x] 3.4 Write test: healthy → degraded on failure
- [x] 3.5 Write test: degraded → healthy on success, failures reset
- [x] 3.6 Write test: backoff increases with consecutive failures
- [x] 3.7 Write test: backoff capped at 30 minutes
- [x] 3.8 Write test: ShouldSkip returns true during backoff, false after

## 4. Orchestration tests

- [x] 4.1 Write test: normal reconcile cycle calls session → settings → content → gravity → recorder in order
- [x] 4.2 Write test: settings change with FTL restart → session refresh → content with fresh session
- [x] 4.3 Write test: session failure → skip everything, mark degraded
- [x] 4.4 Write test: settings failure → continue to content reconciliation
- [x] 4.5 Write test: content returns NeedsGravity=true → gravity triggered
- [x] 4.6 Write test: content returns NeedsGravity=false → gravity NOT triggered
- [x] 4.7 Write test: gravity corruption detected → health=down, gravity NOT triggered
- [x] 4.8 Write test: UpdateConfig preserves health state

## 5. Watch loop refactor

- [x] 5.1 Create worker map management in `cmd/forseti/main.go`: `workers map[string]*worker.TargetWorker`
- [x] 5.2 Implement worker map sync on hot-reload: add new, update existing, remove stale
- [x] 5.3 Replace `reconcileAll()` with worker-based loop: for each worker, skip if backoff, call Reconcile
- [x] 5.4 Remove old `reconcileAll()` function
- [x] 5.5 Wire existing pool, scheduler, metrics server as interface implementations

## 6. Validation

- [x] 6.1 Run `go test ./...` — all tests pass (including existing tests)
- [x] 6.2 Run `golangci-lint run` — no new lint violations
- [x] 6.3 Run `go vet ./...` — clean
- [x] 6.4 Manual smoke test: verify watch mode starts and reconciles correctly
