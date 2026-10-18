## ADDED Requirements

### Requirement: CNAME sync support
The sync engine SHALL include CNAME records in primary-to-replica synchronization. CNAME sync SHALL follow the same key-based add/delete pattern as local DNS A/AAAA records (no marker, purge-flag controlled).

#### Scenario: Sync CNAME from primary to replica
- **WHEN** the primary has a CNAME record (domain, target) that the replica does not
- **THEN** the sync SHALL create the CNAME record on the replica

#### Scenario: Delete orphaned CNAME on replica
- **WHEN** the replica has a CNAME record not present on the primary and purge is enabled
- **THEN** the sync SHALL delete the CNAME record from the replica

#### Scenario: CNAME in valid sync resources
- **WHEN** the sync config specifies `resources` to sync
- **THEN** `cnames` SHALL be a valid resource name in the list

#### Scenario: CNAME restart trigger
- **WHEN** CNAME records are added or removed during sync
- **THEN** the sync SHALL trigger an FTL restart on the affected replica (existing CNAME restart behavior)
