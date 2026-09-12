## Why

Production logs show Forseti stuck in an infinite gravity loop: every 5-minute reconcile cycle, the diff detects adlists to delete, the delete returns 404 (already gone), gravity triggers anyway, and gravity times out. This has been running for 1.5+ hours non-stop on both targets. The root cause is threefold: `NeedsGravity` is computed from the diff before operations run (so phantom deletes trigger gravity), the batchDelete address may not match Pi-hole's stored URL (causing the persistent 404), and gravity runs synchronously during reconcile which can corrupt the DB for the next target.

## What Changes

- **NeedsGravity based on actual outcomes**: Track whether creates, updates, or successful deletes actually happened, not just whether the diff has entries. A 404 delete (idempotent, item already gone) does not warrant gravity.
- **Address normalization for batchDelete**: Investigate and fix the mismatch between how Pi-hole stores adlist URLs and how Forseti sends them in batchDelete. Use the ID-based delete endpoint as fallback when address-based delete returns 404.
- **Gravity-corrupted DB detection**: Detect persistent "no such table" errors as a gravity corruption signal. Skip reconcile and gravity for that target, log at ERROR, and mark target as degraded until gravity completes successfully.
- **Prevent gravity trigger during active gravity**: If gravity is already running for a target (from scheduler or previous cycle), skip triggering it again.

## Capabilities

### New Capabilities
- `gravity-loop-prevention`: Gravity trigger conditions, corruption detection, and duplicate gravity prevention.

### Modified Capabilities
- `reconciler`: NeedsGravity now based on actual operation outcomes, not diff entries.
- `pihole-api`: Adlist delete falls back to ID-based endpoint when address-based returns 404.

## Impact

- `internal/reconcile/reconciler.go` — NeedsGravity logic, delete tracking
- `internal/pihole/client.go` — adlist delete fallback by ID
- `internal/gravity/scheduler.go` — duplicate trigger prevention, corruption detection
- No config changes, no breaking changes
