## Context

The reconciler bridges config (desired state) and pihole client (actual state). It needs to handle marker-based ownership, a fixed processing order, and produce actionable diffs for both plan and apply modes.

## Goals / Non-Goals

**Goals:**
- Interface-based pihole client dependency for testability
- `Plan(cfg, client) -> DiffReport` for dry-run
- `Apply(cfg, client) -> ApplyReport` for execution
- Per-resource-type diff logic with marker filtering
- Multi-target orchestration with independent error handling

**Non-Goals:**
- Rollback on partial failure (report errors, continue with other targets)
- Conflict resolution between targets
- Watch loop (that's the CLI's responsibility)

## Decisions

### Interface for Pi-hole operations
Define a `PiholeAPI` interface with the methods the reconciler needs. The real `pihole.Client` satisfies it; tests use a mock. This avoids httptest overhead in reconciler tests.

### DiffReport as the core data structure
```
DiffReport
├── Target     string
├── Groups     ResourceDiff  (adds, deletes, unchanged count)
├── Adlists    ResourceDiff
├── Deny       ResourceDiff
├── Allow      ResourceDiff
├── LocalDNS   ResourceDiff
├── Clients    ResourceDiff
├── NeedsGravity  bool
```

### Marker matching
An entry is "managed" if its comment field contains the marker string (e.g., `[forseti]`). Matching uses `strings.Contains` — the marker can appear anywhere in the comment.

### Reconcile order enforced structurally
The `reconcile` function processes resource types in a hardcoded slice order. No dynamic ordering or dependency resolution needed since the order is fixed by spec.

### Delete order is reversed
For deletions, clients are deleted before groups to avoid referential integrity errors. The apply phase processes additions in forward order and deletions in reverse order.

## Risks / Trade-offs

- **Group ID resolution** → Config references groups by name, API uses numeric IDs. The reconciler must map names to IDs after fetching actual groups. New groups get IDs after creation.
- **Partial failures** → If creating one adlist fails, the reconciler logs the error and continues. The apply report captures all errors.
