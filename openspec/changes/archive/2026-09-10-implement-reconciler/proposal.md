## Why

The reconciler is the core of Forseti — it computes the diff between desired state (YAML config) and actual state (Pi-hole API), then applies changes. With config and pihole client packages done, this is the critical piece that makes `plan` and `apply` commands functional.

## What Changes

- Implement the reconciler in `internal/reconcile/reconciler.go`
- Marker-based ownership: only touch entries tagged with the configured marker
- Three-way diff: desired vs. actual, filtered by marker presence
- Fixed reconcile ordering: groups -> adlists -> deny -> allow -> local DNS -> clients
- Gravity trigger only after adlist changes
- Plan mode (dry-run) returning a diff report
- Apply mode executing changes against a Pi-hole target
- Multi-target support: reconcile each target independently

## Capabilities

### New Capabilities

None. Implements the existing `reconciler` spec.

### Modified Capabilities

None.

## Impact

- `internal/reconcile/reconciler.go` — full implementation
- `internal/reconcile/reconciler_test.go` — unit tests
- Depends on `internal/config` and `internal/pihole`
