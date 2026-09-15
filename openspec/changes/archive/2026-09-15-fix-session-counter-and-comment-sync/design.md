## Context

### Session counter

The session pool tracks active sessions via callbacks: `OnNewSession` increments the `forseti_session_active` gauge, `OnClose` resets it to 0. `Invalidate()` deletes a session (closes client, removes from map) but has no callback — the gauge is never decremented. Sessions are invalidated on: auth failure, reconcile error, settings change requiring FTL restart.

### Comment/marker sync

The reconciler's diff functions match entries by key (URL for adlists, domain for deny/allow, client IP for clients). When a match is found, only group membership is compared — the comment field is ignored. The marker (`[forseti]`) is embedded in the comment by `buildComment()` during create/update. Pre-existing entries that happen to share the same URL never receive the marker because no update is triggered.

## Goals / Non-Goals

**Goals:**
- `session_active` accurately reflects the number of live sessions at all times
- Entries matching a desired URL/domain but missing the managed marker receive it via an update

**Non-Goals:**
- Changing the marker format or position within the comment
- Adding a dedicated `OnInvalidate` callback — the existing `OnNewSession` pattern is sufficient; we just need a matching decrement

## Decisions

### Session counter: add OnInvalidate callback

**Decision**: Add `OnInvalidate func(target string)` to `PoolCallbacks`. Call it in `Invalidate()` when a session is actually removed. Wire it to `session_active.Dec()` in main.go.

**Rationale**: Mirrors the `OnNewSession` pattern. A simple `Dec()` rather than tracking exact counts — the gauge is a simple counter, not per-target.

**Alternative considered**: Decrement directly inside `Invalidate()` without a callback. Rejected because the pool doesn't know about metrics — the callback pattern keeps that separation.

### Comment sync: treat missing marker as drift

**Decision**: In each diff function, when an entry exists by key, also check if `strings.Contains(actual.Comment, marker)`. If the marker is missing, emit an Update. When both groups AND comment are correct, count as Unchanged.

**Rationale**: The marker is load-bearing — it controls whether forseti can later delete the entry. An entry without the marker is in a broken state from forseti's perspective, regardless of group assignment.

## Risks / Trade-offs

- [Low] First reconcile on existing setups will trigger updates for all pre-existing entries to add the marker. This is a one-time migration effect and is the correct behavior.
- [Low] The comment comparison uses `strings.Contains(comment, marker)` — same check as the delete path. No risk of false positives.
