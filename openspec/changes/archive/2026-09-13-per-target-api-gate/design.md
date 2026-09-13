## Context

Forseti has three subsystems that independently hit each Pi-hole's REST API: the reconciler (sequential CRUD operations), the collector (4 parallel stats calls on Prometheus scrape), and the gravity scheduler (heavyweight adlist rebuild). All three share a session pool but have no coordination — they fire concurrently against the same FTL instance.

On resource-constrained targets (Pi Zero 2 W), this causes cascading failures: FTL can't serve concurrent requests fast enough, the collector's hardcoded 10s timeout expires, and the target goes permanently dark in metrics. Under heavy reconcile load (adlist deletion taking 30s+), even fast targets like a MikroTik start timing out because FTL is busy processing reconciler requests.

The session pool (`session.Pool`) is the natural coordination point — it already owns per-target client lifecycle and is the single entry point all subsystems use.

## Goals / Non-Goals

**Goals:**
- Prevent concurrent API overload on any single Pi-hole target
- Allow per-target tuning of timeout and concurrency limits via config
- Ensure the collector always returns data (stale over nothing)
- Fix the `TriggerGravity` data race on `httpClient` field swap
- Maintain backward compatibility — existing configs work unchanged with current defaults

**Non-Goals:**
- Priority scheduling between subsystems (reconciler > collector etc.) — simple semaphore is sufficient
- Per-endpoint rate limiting within a single subsystem
- Automatic detection of target performance characteristics
- Changing the collect-on-scrape model to background collection (future work)

## Decisions

### 1. Gate lives in `session.Pool`

**Decision:** Add per-target semaphore (buffered channel) to the session pool with `Acquire`, `TryAcquire`, and `Release` methods.

**Why:** The pool already owns per-target state and is injected into all three subsystems. Adding the gate here avoids a new cross-cutting component and keeps the coordination close to the resource it protects.

**Alternative considered:** Standalone `gate` package — rejected because it would need its own per-target map, lifecycle management, and would be yet another dependency to wire through all subsystems.

**Alternative considered:** Gate inside `pihole.Client` — rejected because the client doesn't know about cross-subsystem coordination; it's a single-target HTTP wrapper.

### 2. Channel-based semaphore, not `sync.Mutex`

**Decision:** Use `chan struct{}` with capacity = `max_concurrent` as the semaphore.

**Why:** Channels support both blocking (`Acquire`) and non-blocking (`TryAcquire` via `select/default`) naturally. A mutex only supports exclusive access (max_concurrent=1) and doesn't support try-acquire without `sync.TryLock` (which exists but using channels is more idiomatic for counted semaphores).

### 3. Subsystem acquire strategies

**Decision:**
- Reconciler: `Acquire` (blocking) — reconciliation is the primary function, must complete
- Collector: `TryAcquire` (non-blocking) — on failure, serve stale cache
- Gravity: `Acquire` (blocking) — rare, important, runs asynchronously

**Why:** The collector runs on every Prometheus scrape (every 30s). Blocking would hold up the scrape response and potentially cause Prometheus to mark the target as down. Stale data is strictly better than no data. The reconciler and gravity are infrequent and must complete their work.

### 4. Per-target config via `api` block

**Decision:** Add an `api` sub-struct to `config.Target` with `timeout` (duration, default 30s) and `max_concurrent` (int, default 4).

**Why:** Different Pi-hole targets have vastly different hardware capabilities. A Pi Zero needs 45s timeout and serialized access; a MikroTik is fine with 10s and 4 concurrent calls. The `api` block groups these knobs logically and leaves room for future additions (e.g., retry config).

**Default rationale:** 30s matches the current hardcoded `httpClient.Timeout`. 4 matches the current collector behavior (4 parallel API calls).

### 5. Fix gravity `httpClient` race via local client

**Decision:** `TriggerGravity` creates a local `*http.Client` with the extended timeout instead of swapping the shared `httpClient` field.

**Why:** The current code does `c.httpClient = &http.Client{Timeout: timeout}` then defers restore — a data race if any other goroutine uses the same `*pihole.Client` concurrently. With `max_concurrent > 1`, this race is live. Creating a local client for the single gravity POST eliminates the race without adding a mutex to the hot path.

### 6. Scrape handler timeout derived from config

**Decision:** Replace the hardcoded `context.WithTimeout(10s)` in `metrics.go` with the maximum `api.timeout` across all configured targets, plus a small buffer (5s).

**Why:** The current 10s is too short for slow targets. Using `max(target.api.timeout) + 5s` ensures the scrape handler gives every target enough time while still having a finite deadline. The buffer accounts for session auth overhead.

### 7. Extend `SessionManager` interface with gate methods

**Decision:** Add `Acquire(name string)`, `TryAcquire(name string) bool`, and `Release(name string)` to the `SessionManager` interface in `worker/interfaces.go`.

**Why:** The worker already depends on `SessionManager`. Adding gate methods to this interface keeps the dependency graph clean and makes the gate testable via the existing mock patterns.

## Risks / Trade-offs

**[Stale metrics during long reconcile]** With `max_concurrent: 1`, a reconcile that takes 60s means 2 scrape cycles serve stale data → Acceptable: stale data is better than the current behavior (no data at all). The `forseti_collector_cache_stale_total` metric makes this visible.

**[Gate not released on panic]** If a subsystem panics while holding the gate, the slot is leaked → Mitigation: all acquire/release pairs use `defer`. Go's defer runs on panic. The gate channel is also per-target, so a leaked slot only affects one target.

**[Gravity still uses pool.Get under gate]** The gravity scheduler currently calls `pool.Get()` inside its trigger — this acquires the pool mutex briefly (for session lookup), then the gate semaphore. If reconciler holds the gate and calls `pool.Invalidate` (which also takes the pool mutex), there's no deadlock because `Invalidate` doesn't wait on the gate → No risk, but worth noting the lock ordering: pool.mu is always acquired before the gate channel.

**[Backward compatibility]** Missing `api` block in config → defaults apply (timeout=30s, max_concurrent=4). Existing behavior is preserved exactly. No breaking changes.
