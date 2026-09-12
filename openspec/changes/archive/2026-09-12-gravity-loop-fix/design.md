## Context

Production shows a vicious cycle: every reconcile cycle, the diff finds adlists to delete, batchDelete returns 404, but `NeedsGravity` is still true because it's computed from the diff entries (not actual outcomes). Gravity triggers, times out, and the cycle repeats. On pizero, gravity corruption ("no such table") persists across cycles.

The gravity scheduler has no deduplication — concurrent triggers from the scheduler cron and the reconcile path can overlap, and gravity runs synchronously under the scheduler's mutex, blocking all scheduled operations.

## Goals / Non-Goals

**Goals:**
- Stop gravity triggering on phantom deletes (404 = nothing changed = no gravity needed)
- Prevent concurrent gravity triggers for the same target
- Detect gravity DB corruption and skip reconcile for degraded targets

**Non-Goals:**
- Fixing the Pi-hole batchDelete address mismatch (likely a Pi-hole bug — Forseti sends the exact address Pi-hole returned)
- Async gravity execution (separate change, larger refactor)
- Investigating why pizero's gravity.db is corrupted (infrastructure issue)

## Decisions

### 1. NeedsGravity from actual outcomes, not diff

**Decision**: Track `adlistsChanged bool` in the Apply function. Set it true only when a CreateAdlist or UpdateAdlist call succeeds, or when DeleteAdlists succeeds (not 404). Set `report.Diff.NeedsGravity` from this flag instead of `HasChanges()`.

**Why?** The diff shows intent, not outcomes. A 404 delete means nothing changed — gravity is unnecessary. With our idempotent delete fix, 404s are suppressed, but HasChanges() still counts them.

### 2. Gravity in-flight tracking

**Decision**: Add an `inFlight map[string]bool` (protected by the existing mutex) to the gravity scheduler. Before triggering, check if gravity is already running for that target. If so, skip with a log warning.

**Why?** The scheduler's `trigger()` runs synchronously under the mutex. If the reconcile path calls `TriggerNow()` while scheduled gravity is running (or vice versa), they can overlap. The mutex only protects map access, not the gravity HTTP call itself.

**Alternative considered**: Make gravity async with channels. Rejected as too large a refactor for this fix.

### 3. Gravity corruption detection

**Decision**: In the reconcile path (main.go), check for "no such table" in the error message from list operations (after retry exhaustion). If detected, log at ERROR with a specific message ("gravity database corrupted, skipping target") and mark target as unreachable. Don't trigger gravity for a corrupted target.

**Why?** "no such table: gravity" / "no such table: group" means the gravity.db is fundamentally broken. Retrying won't help. Gravity rebuild might fix it, but only if triggered externally (not by Forseti's reconcile loop which would just add load). The next cycle will try again after gravity presumably completes.

## Risks / Trade-offs

- **False positive on corruption detection** → String matching on error messages is fragile. Mitigated by only matching the specific "no such table" pattern that Pi-hole returns.
- **Missed gravity trigger** → If a real adlist change happens in the same cycle as a phantom delete, we still trigger gravity because CreateAdlist/UpdateAdlist success sets the flag.
- **In-flight tracking without async** → The `inFlight` flag must be set before and cleared after the synchronous call. A panic could leak the flag. Mitigated by using defer.
