## Why

In sync mode, forseti replicates adlists, domains, and clients from a primary Pi-hole to replicas. However, group assignments are silently dropped: `syncAdlists`, `syncDomains`, and `syncClients` all pass `nil` for the `groups` parameter when creating entries on replicas. This means entries that are scoped to specific groups on the primary end up assigned to no groups (or default group only) on replicas, breaking filtering rules.

## What Changes

- **Group assignment replication**: When syncing adlists, domains, and clients, translate the primary's group IDs to replica group IDs by name, then pass them to the Create API calls.
- **Group ID translation**: Build a primary-name→replica-ID mapping after groups are synced, since group IDs differ between Pi-hole instances.
- **Sync order guarantee**: Groups are already synced first, so the required groups exist on the replica before adlists/domains/clients reference them.

## Capabilities

### New Capabilities

- `sync-group-preservation`: Preserve group assignments when syncing resources from primary to replica Pi-hole instances.

### Modified Capabilities

_(none)_

## Impact

- `internal/sync/syncer.go`: All sync methods gain group translation logic; `syncAdlists`, `syncDomains`, `syncClients` pass translated group IDs instead of `nil`
- `internal/sync/syncer_test.go`: New test cases verifying group assignments are preserved
- No config changes required — this is a correctness fix for existing sync behavior
