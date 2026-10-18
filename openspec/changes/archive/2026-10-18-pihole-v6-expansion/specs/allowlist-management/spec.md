## ADDED Requirements

### Requirement: Allowlist YAML configuration
The system SHALL accept an `allowlists` field in the YAML config with the same structure as `adlists` (URL, comment, groups). Validation SHALL reject duplicate URLs within the allowlists array. Allowlists SHALL only be valid in config mode.

#### Scenario: Valid allowlist config
- **WHEN** the YAML config contains `allowlists` with valid URLs and group references
- **THEN** the config loads successfully and allowlists are available for reconciliation

#### Scenario: Duplicate allowlist URL
- **WHEN** the YAML config contains two allowlist entries with the same URL
- **THEN** config validation SHALL fail with a descriptive error

#### Scenario: Allowlists in sync mode
- **WHEN** the mode is `sync` and the YAML config contains `allowlists`
- **THEN** the allowlists field is ignored (sync reads from primary, not YAML)

### Requirement: Allowlist reconciliation
The system SHALL reconcile allowlists using the `[forseti]` managed marker in the comment field. Reconciliation order SHALL place allowlists after adlists (block-lists) and before deny domains. The system SHALL support add, update (in-place via PUT), and delete operations.

#### Scenario: Add new allowlist
- **WHEN** the desired config contains an allowlist URL not present on the target
- **THEN** the system SHALL create the allowlist via `POST /api/lists?type=allow` with the `[forseti]` marker in the comment

#### Scenario: Update existing allowlist groups
- **WHEN** an allowlist exists on the target with the `[forseti]` marker but has different group assignments than desired
- **THEN** the system SHALL update the allowlist in-place via `PUT /api/lists/{address}?type=allow`

#### Scenario: Delete orphaned allowlist
- **WHEN** a `[forseti]`-marked allowlist exists on the target but is not in the desired config
- **THEN** the system SHALL delete the allowlist via batch delete or individual DELETE

#### Scenario: Manual allowlists preserved
- **WHEN** an allowlist exists on the target without the `[forseti]` marker
- **THEN** the system SHALL NOT modify or delete it

#### Scenario: Gravity trigger on allowlist change
- **WHEN** allowlists are added, updated, or deleted during apply
- **THEN** the system SHALL set `NeedsGravity` to trigger a gravity update (if `gravity_on_change` is enabled)

### Requirement: Allowlist sync
The system SHALL sync allowlists from primary to replicas in sync mode. Group IDs SHALL be translated from primary to replica using the existing group-name-based translation. The `[forseti-sync]` marker SHALL be used for sync-managed allowlists.

#### Scenario: Sync allowlists to replica
- **WHEN** the primary has allowlists and the replica does not
- **THEN** the system SHALL create the missing allowlists on the replica with translated group IDs and the `[forseti-sync]` marker

#### Scenario: Delete orphaned synced allowlists
- **WHEN** a `[forseti-sync]`-marked allowlist exists on the replica but not on the primary
- **THEN** the system SHALL delete it from the replica

### Requirement: Allowlist plan output
The system SHALL include allowlists in `plan` output, showing adds, updates, and deletes alongside other resource types.

#### Scenario: Plan shows allowlist changes
- **WHEN** the user runs `forseti plan` and allowlist differences exist
- **THEN** the plan output SHALL list allowlist additions, updates, and deletions
