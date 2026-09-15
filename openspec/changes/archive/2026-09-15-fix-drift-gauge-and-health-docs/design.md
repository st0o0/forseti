## Context

`forseti_reconcile_drift{target, type}` is a Prometheus GaugeVec tracking how many items differ from desired state per resource type per target. The current flow:

1. `buildDriftMap` (worker.go) iterates all resource types and only inserts entries where `total > 0`
2. `RecordReconcile` (metrics.go) iterates the returned map and calls `Set(count)` for each entry
3. When a resource type returns to sync (total = 0), it is absent from the map → `Set(0)` is never called → the gauge retains its last non-zero value

Separately, `forseti_target_health` uses an enum encoding (0=healthy, 1=degraded, 2=down) that is documented in the metric Help text but not in the README metrics table.

## Goals / Non-Goals

**Goals:**
- Drift gauges reflect current state after every reconcile cycle (stale values are cleared)
- README documents the `target_health` enum encoding

**Non-Goals:**
- Changing the `target_health` encoding (0=healthy is valid Prometheus style for enum gauges)
- Adding a `--target` filter to `forseti plan` (separate concern)

## Decisions

### Reset drift gauges in `buildDriftMap`

**Decision**: Always emit all 7 resource type keys in `buildDriftMap`, with count 0 when there is no drift.

**Rationale**: The fix belongs in `buildDriftMap` rather than in `RecordReconcile` because the map is the contract between worker and metrics — it should represent the complete current state, not a sparse delta. This keeps `RecordReconcile` a simple pass-through without needing to know the set of resource type names.

**Alternative considered**: Reset all drift labels to 0 inside `RecordReconcile` before iterating the map. Rejected because it couples the metrics layer to knowledge of resource type names and adds an extra loop.

### README documentation only for health encoding

**Decision**: Add a footnote to the README metrics table clarifying `target_health` values. No code change to the encoding.

**Rationale**: The 0=healthy enum pattern is standard for Prometheus state gauges (cf. `kube_node_status_condition`). Changing it would break existing dashboards and alerts.

## Risks / Trade-offs

- [Minimal risk] Setting 7 gauge values per cycle per target adds negligible overhead (7 `Set` calls vs current 0-7).
- [None identified] The README change is purely documentary.
