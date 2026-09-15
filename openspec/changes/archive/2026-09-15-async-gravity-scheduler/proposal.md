## Why

The gravity scheduler's `triggerLocked` method performs synchronous I/O (HTTP session acquire, login, gravity POST) while holding the event-loop mutex. With 18 adlists in production, Pi-hole gravity takes 30-120 seconds. During this time the scheduler cannot process cron ticks, async gravity requests from workers, or respond to shutdown signals. Docker test confirmed the architecture produces `database is locked` errors when gravity and reconcile overlap.

## What Changes

- **Async gravity execution**: `triggerLocked` spawns a goroutine for the I/O work instead of blocking. The existing `inFlight` map already prevents concurrent runs per target.
- **Graceful shutdown**: Add a `sync.WaitGroup` so `Start` waits for in-flight gravity goroutines before returning on context cancellation.
- **Simplify `processAsync`**: Both code paths (with/without schedule entry) use the same async pattern, eliminating the fragile defer-based manual lock/unlock on the no-schedule path.

## Capabilities

### New Capabilities

_(none)_

### Modified Capabilities

- `gravity-loop-prevention`: Gravity execution becomes non-blocking — the "no concurrent gravity per target" requirement is preserved, but the scheduler event-loop is no longer blocked during execution.

## Impact

- `internal/gravity/scheduler.go` — `triggerLocked`, `processAsync`, `Start` (WaitGroup)
- `internal/gravity/scheduler_test.go` — tests that call `triggerLocked` directly need adjustment for async behavior
- No API changes, no config changes, no breaking changes
