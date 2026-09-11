## ADDED Requirements

### Requirement: Adlist group assignments are preserved during sync

When syncing adlists from primary to replica, the syncer SHALL translate the primary's group IDs to replica group IDs by matching group names, and pass the translated IDs to `CreateAdlist`.

#### Scenario: Adlist with groups synced to replica

- **WHEN** the primary has adlist "https://example.com/list.txt" assigned to groups [2, 3] where ID 2 is "ads" and ID 3 is "tracking", and the replica has groups "ads" (ID 5) and "tracking" (ID 7)
- **THEN** the syncer SHALL create the adlist on the replica with groups [5, 7]

#### Scenario: Adlist with unresolvable group

- **WHEN** the primary adlist references group ID 99 which has no corresponding group name on the replica
- **THEN** the syncer SHALL create the adlist without that group assignment and log the skip

### Requirement: Domain group assignments are preserved during sync

When syncing deny/allow domains from primary to replica, the syncer SHALL translate group IDs and pass them to `CreateDomain`.

#### Scenario: Domain with groups synced to replica

- **WHEN** the primary has deny domain "ads.example.com" assigned to groups [2] where ID 2 is "ads", and the replica has group "ads" (ID 4)
- **THEN** the syncer SHALL create the domain on the replica with groups [4]

### Requirement: Client group assignments are preserved during sync

When syncing clients from primary to replica, the syncer SHALL translate group IDs and pass them to `CreateClient`.

#### Scenario: Client with groups synced to replica

- **WHEN** the primary has client "192.168.1.100" assigned to groups [2, 3], and both groups exist on the replica with different IDs
- **THEN** the syncer SHALL create the client on the replica with the translated group IDs

### Requirement: Replica groups are re-fetched after group sync

After `syncGroups` completes, the syncer SHALL re-fetch the replica's group list to obtain IDs for any newly created groups.

#### Scenario: Newly synced group is available for translation

- **WHEN** group "tracking" does not exist on the replica before sync, and `syncGroups` creates it with replica ID 8
- **THEN** subsequent adlist/domain/client sync SHALL be able to resolve "tracking" to replica ID 8

### Requirement: Group translation helper

The syncer SHALL use a shared translation function that converts primary group IDs to replica group IDs via group name lookup.

#### Scenario: Translation with full resolution

- **WHEN** primary IDs [1, 2] map to names ["default", "ads"] and replica has "default" (ID 1) and "ads" (ID 3)
- **THEN** the translation SHALL return [1, 3]

#### Scenario: Translation with partial resolution

- **WHEN** primary IDs [1, 99] and ID 99 has no name mapping
- **THEN** the translation SHALL return [1] (only resolvable IDs)
