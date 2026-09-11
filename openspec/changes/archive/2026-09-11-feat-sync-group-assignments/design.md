## Context

The sync engine replicates resources from a primary Pi-hole to one or more replicas. Groups are synced first (by name), then adlists, domains, and clients. Each resource type has a `Groups []int` field with group IDs, but these IDs are instance-specific — group "ads" might be ID 2 on the primary and ID 5 on the replica. Currently all sync methods pass `nil` for groups, losing assignments.

## Goals / Non-Goals

**Goals:**
- Preserve group assignments during sync by translating primary group IDs to replica group IDs via group name matching
- Handle missing groups gracefully (skip assignment if a group doesn't exist on the replica)

**Non-Goals:**
- Updating group assignments on already-synced entries (update-in-place is a separate change)
- Syncing group metadata beyond name/enabled state

## Decisions

### 1. Build group name→ID maps from both sides

After `syncGroups` runs, re-fetch the replica's groups to get the authoritative name→ID mapping (including any groups just created). Build a parallel primary-ID→name map from the primary's group list. Translation: primary group ID → primary group name → replica group ID.

**Why**: Group IDs are auto-incremented per instance. Name is the stable identifier. Re-fetching the replica's groups after sync ensures newly created groups have their IDs available.

### 2. Helper function `translateGroupIDs`

A shared `translateGroupIDs(primaryIDs []int, primaryIDToName map[int]string, replicaNameToID map[string]int) []int` function used by all three sync methods.

**Why**: Avoids duplicating the translation logic. Returns only IDs that resolved successfully — unresolvable groups are silently skipped (the group either doesn't exist or was a default group).

### 3. Pass translated groups to Create calls

Change `nil` → `translateGroupIDs(...)` in `syncAdlists`, `syncDomains`, and `syncClients`.

### 4. Re-fetch replica groups after syncGroups

The `syncOneReplica` method already fetches replica state before syncing. After `syncGroups` completes (potentially adding new groups), re-fetch `replica.ListGroups()` to get updated IDs.

**Why**: A group created during `syncGroups` won't be in the original replica state snapshot. Without re-fetch, newly synced groups can't be referenced.

## Risks / Trade-offs

- **Extra API call per replica** (one `ListGroups` re-fetch) → Acceptable; one lightweight GET per sync cycle.
- **Unresolvable group IDs are silently dropped** → By design. If a primary entry references a group that wasn't synced (e.g., group sync is disabled in resources config), the entry is created without that group. Logging at debug level.
