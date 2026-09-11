## Context

The reconciler uses `ResourceDiff` with `Adds`, `Deletes`, and `Unchanged` fields. Diff functions compare keys (URL for adlists, IP for clients, domain for domains) and check marker ownership for deletions. When a key exists on both sides, it's counted as `Unchanged` — even if attributes like group assignments differ. The Pi-hole v6 REST API supports PUT for updating existing resources, but the client only implements POST (create) and batch DELETE.

## Goals / Non-Goals

**Goals:**
- Detect group assignment drift between desired config and actual Pi-hole state
- Apply updates via PUT without delete-and-recreate (preserving Pi-hole counters/metadata)
- Extend `ResourceDiff` with an `Updates` slice so plan output shows updates separately from adds

**Non-Goals:**
- Updating the `enabled` field (forseti always creates enabled entries)
- Updating comments (the marker is the only managed comment content)
- Update detection for groups themselves (groups have no sub-attributes beyond name)
- Update detection for DNS records (no group assignments, no metadata)

## Decisions

### 1. New `ActionUpdate` diff action and `Updates` slice

Add `ActionUpdate DiffAction = "update"` and `Updates []DiffEntry` to `ResourceDiff`. `HasChanges()` includes updates. `DiffEntry` already carries `ID` which is needed for PUT calls.

Why: Clean separation in plan output and apply logic. Updates are semantically distinct from add+delete pairs.

### 2. Group comparison uses sorted int slices

When a key matches, resolve desired group names to IDs using `buildGroupNameToID`, then compare sorted `[]int` slices. If they differ, emit an update entry.

Why: The Pi-hole API stores groups as `[]int` IDs. Comparing at the ID level avoids name-vs-ID mismatches. Sorting ensures order-independent comparison.

### 3. Diff functions receive `groupNameToID` map

`diffAdlists` and `diffClients` (and domain variants) need the group name-to-ID map to resolve desired group names for comparison. Pass it as an additional parameter.

Why: The map is already built in `Plan()` and `Apply()` from the groups list. Threading it through avoids redundant API calls.

### 4. Pi-hole client update methods use PUT with resource ID

New methods: `UpdateAdlist(id int, groups []int)`, `UpdateClient(id int, groups []int)`, `UpdateDomain(id int, groups []int)`. Each does `PUT /api/<resource>/<id>` with a `{"groups": [...]}` body.

Why: Pi-hole v6 API convention — PUT to the resource path with ID for partial updates.

### 5. Apply processes updates after adds, before deletes

Order: adds → updates → deletes. This ensures newly created groups are available for update references.

Why: Consistent with the existing add-before-delete pattern for dependency ordering.

## Risks / Trade-offs

- **Interface change breaks mock implementations in tests** → Tests use a `mockAPI` struct that must add the new methods. Internal-only impact.
- **Group resolution depends on accurate group list** → If groups were just created in the same apply cycle, `groupNameToID` is updated with their IDs (already handled for adds). Updates use the same map.
- **PUT semantics may vary across Pi-hole versions** → Mitigated by targeting Pi-hole v6 only, which is the stated API target.
