# Config

YAML configuration parsing, environment variable expansion, validation, and multi-target support.

### Requirement: YAML configuration format
The system SHALL accept a single YAML file as its configuration source. The file defines metrics settings, Pi-hole targets, reconcile behavior, and all managed resources (adlists, deny domains, allow domains, local DNS records, groups, clients).

#### Scenario: Valid config with all sections
- **WHEN** a YAML file contains metrics, targets, reconcile, adlists, deny, allow, local_dns, groups, and clients sections
- **THEN** the system SHALL parse all sections and make them available to the reconciler

#### Scenario: Minimal config
- **WHEN** a YAML file contains only targets (with at least one entry) and no resource sections
- **THEN** the system SHALL accept the config with empty resource lists

### Requirement: Environment variable expansion
The system SHALL expand `${VAR}` references in string values using environment variables before YAML parsing. This enables secret injection (passwords) without storing them in the config file.

#### Scenario: Password from environment
- **WHEN** a target password is `${PIHOLE_PASSWORD}` and the environment variable `PIHOLE_PASSWORD` is set to `secret123`
- **THEN** the parsed target password SHALL be `secret123`

#### Scenario: Undefined variable
- **WHEN** a `${VAR}` reference has no matching environment variable
- **THEN** the system SHALL return a validation error naming the undefined variable

### Requirement: Multi-target support
The system SHALL support an array of Pi-hole targets under the `targets` key. Each target has a name, URL, and password. All resource definitions (adlists, domains, etc.) are reconciled against every target.

#### Scenario: Two targets
- **WHEN** config defines two targets (pihole-router and pihole-pi)
- **THEN** the reconciler SHALL apply the same desired state to both instances

### Requirement: Config validation
The system SHALL validate the parsed config before any reconciliation. Validation covers: target URLs are valid HTTP(S), domain entries are valid domain names, CIDR client matches are valid notation, group references in clients/adlists resolve to defined groups, and no duplicate entries within a resource type.

#### Scenario: Invalid target URL
- **WHEN** a target URL is `not-a-url`
- **THEN** the system SHALL return a validation error before contacting any Pi-hole instance

#### Scenario: Duplicate adlist URL
- **WHEN** two adlist entries have the same URL
- **THEN** the system SHALL return a validation error

### Requirement: Configurable reconcile settings
The system SHALL accept reconcile settings: `interval` (duration for watch mode), `marker` (string tag for managed entries, default `[forseti]`), and `gravity_on_change` (boolean, trigger gravity update after adlist changes).

#### Scenario: Custom marker
- **WHEN** marker is set to `[managed]`
- **THEN** the system SHALL use `[managed]` instead of `[forseti]` for tagging and identifying managed entries
