## Context

The gravity scheduler runs a single-goroutine event loop (`Start`) that multiplexes three concerns: cron checks (30s ticker), async gravity requests from workers (channel), and shutdown. `triggerLocked` performs the actual gravity API call synchronously under the mutex, blocking all three.

The `inFlight` map already exists to prevent concurrent gravity runs per target. It's currently redundant because everything runs serially, but it becomes load-bearing once we make execution async.

## Goals / Non-Goals

**Goals:**
- Gravity execution does not block the scheduler event loop
- Graceful shutdown waits for in-flight gravity operations
- `processAsync` has a single, clear lock discipline

**Non-Goals:**
- Changing the cron evaluation logic or tick interval
- Adding gravity-reconcile coordination (database-lock avoidance) — separate concern
- Changing the `TriggerNow` (synchronous) API — it's used by `forseti plan` and must remain blocking

## Decisions

### Goroutine per trigger, guarded by `inFlight`

**Decision**: `triggerLocked` sets `inFlight[target] = true` under the mutex, then spawns a goroutine that does all I/O (Acquire, Get, TriggerGravity) and clears `inFlight` when done. The caller returns immediately.

**Rationale**: The `inFlight` guard already has the right semantics — it prevents double runs per target while allowing independent targets to run concurrently. Making the I/O async is the minimal change to unblock the event loop.

**Alternative considered**: Dedicated worker goroutine per target with a trigger channel. Rejected — adds complexity without benefit since `inFlight` already handles deduplication.

### WaitGroup for shutdown

**Decision**: Add `sync.WaitGroup` to `Scheduler`. Each spawned goroutine increments it; `Start` calls `wg.Wait()` after the event loop exits on `ctx.Done()`.

**Rationale**: Without this, `Start` returns immediately on shutdown and in-flight gravity operations become orphaned goroutines with no cleanup.

### Consolidate `processAsync`

**Decision**: `processAsync` delegates to the same async `triggerLocked` for targets with a schedule entry. For targets without an entry, it creates a temporary entry-like call to the same async mechanism. This eliminates the manual `mu.Unlock()` / deferred `mu.Lock()` pattern.

**Rationale**: The current code has two paths with different lock disciplines — one calls `triggerLocked` (sync under lock), the other manually unlocks and re-locks with defers. Both should use the same async pattern.

### `TriggerNow` stays synchronous

**Decision**: `TriggerNow` keeps its current blocking behavior. It's the public API used by `forseti plan` where the caller needs to know gravity completed before proceeding.

**Rationale**: Making `TriggerNow` async would change its contract. The worker uses `TriggerAsync` which already routes through the event loop.

## Risks / Trade-offs

- [Low] Goroutine per trigger adds concurrency — but the `inFlight` guard and per-target pool semaphore prevent races. The existing tests for concurrent/in-flight behavior validate this.
- [Low] Multiple targets can run gravity concurrently now — previously they were serialized. This is desired behavior but increases momentary Pi-hole load.
