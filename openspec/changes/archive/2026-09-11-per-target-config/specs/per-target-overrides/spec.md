## ADDED Requirements

### Requirement: Target override file loading
The system SHALL support an optional `file:` field on each target entry. When present, the system SHALL load the referenced YAML file and merge its contents with the global config to produce the effective config for that target. The path SHALL be resolved relative to the main config file's directory. Absolute paths SHALL also be accepted.

#### Scenario: Target with override file
- **WHEN** a target declares `file: targets/kids.yaml` and the file exists
- **THEN** the system SHALL parse the override file and merge it with global defaults to produce the effective config for that target

#### Scenario: Target without override file
- **WHEN** a target has no `file:` field
- **THEN** the target SHALL receive the unmodified global config as its effective config

#### Scenario: Override file not found
- **WHEN** a target declares `file: targets/missing.yaml` and the file does not exist
- **THEN** the system SHALL return a validation error naming the missing file

### Requirement: Override file structure
The override file SHALL accept the same content sections as the global config: `settings`, `groups`, `adlists`, `deny`, `allow`, `local_dns`, `cname`, and `clients`. It SHALL additionally accept an `exclude` block. It SHALL NOT contain target definitions, mode, metrics, or reconcile settings.

#### Scenario: Override with settings and content
- **WHEN** an override file contains `settings:`, `deny:`, and `adlists:` sections
- **THEN** the system SHALL merge each section with the corresponding global section

#### Scenario: Override with disallowed fields
- **WHEN** an override file contains `targets:` or `mode:`
- **THEN** the system SHALL return a validation error indicating these fields are not allowed in override files

### Requirement: Content list merge (append)
For content list sections (groups, adlists, deny, allow, local_dns, cname, clients), the override file's entries SHALL be appended to the global list. The global list is not replaced.

#### Scenario: Additional deny entries
- **WHEN** global config defines deny entries `[A, B]` and the override file defines deny entries `[C]`
- **THEN** the effective deny list SHALL be `[A, B, C]`

#### Scenario: Empty override section
- **WHEN** the override file has an empty `deny:` section (or omits it)
- **THEN** the effective deny list SHALL equal the global deny list

### Requirement: Settings merge (deep merge)
For the `settings` section, the override file's values SHALL be deep-merged with the global settings. Scalar values in the override replace the corresponding global value. Map keys not present in the override inherit from global.

#### Scenario: Override single nested setting
- **WHEN** global settings define `dns.cache.size: 10000` and `dns.cache.force_on_disk: false`, and the override defines only `dns.cache.force_on_disk: true`
- **THEN** the effective settings SHALL have `dns.cache.size: 10000` and `dns.cache.force_on_disk: true`

#### Scenario: Override with no settings section
- **WHEN** the override file omits the `settings:` section
- **THEN** the effective settings SHALL equal the global settings

### Requirement: Exclude block
The override file SHALL support an `exclude` block that removes specific entries from the effective content lists. Each sub-key under `exclude` corresponds to a content section. Entries are matched by their natural key: `domain` for deny/allow/local_dns/cname, `url` for adlists, `match` for clients, `name` for groups.

#### Scenario: Exclude a global allow entry
- **WHEN** global config allows `gaming.example.com` and the override's `exclude.allow` lists `domain: gaming.example.com`
- **THEN** the effective allow list SHALL NOT contain `gaming.example.com`

#### Scenario: Exclude non-existent entry
- **WHEN** the override's `exclude.deny` references `domain: not-in-global.com` which does not exist in the global deny list
- **THEN** the system SHALL log a warning but not fail validation

#### Scenario: Exclude and append in same section
- **WHEN** the override file both appends new deny entries and excludes a global deny entry
- **THEN** the system SHALL first append the new entries, then apply excludes to produce the effective list

### Requirement: Effective config validation
Validation SHALL run on the effective (merged) config for each target, not on the global or override configs individually. Duplicate detection, group reference validation, and domain format validation SHALL apply to the merged result.

#### Scenario: Duplicate introduced by merge
- **WHEN** global deny contains `tracker.example.com` and the override also adds `tracker.example.com`
- **THEN** the system SHALL return a validation error for the duplicate entry

#### Scenario: Group reference from override
- **WHEN** the override adds an adlist referencing group `kids` which is defined in the override's groups section
- **THEN** validation SHALL pass because the group exists in the effective config
