## ADDED Requirements

### Requirement: Settings configuration format
The system SHALL accept a `settings` section at the top level of the config file. This section uses Forseti's own format — a curated subset of Pi-hole FTL configuration values, not a 1:1 mirror of pihole.toml.

#### Scenario: Settings with DNS and blocking config
- **WHEN** the config file contains a `settings:` section with `dns.upstream`, `dns.cache.size`, and `blocking.mode`
- **THEN** the system SHALL parse these values and make them available for reconciliation

#### Scenario: No settings section
- **WHEN** the config file omits the `settings:` section
- **THEN** the system SHALL skip settings reconciliation entirely — existing Pi-hole settings are not touched

### Requirement: Settings field mapping
Each Forseti setting SHALL map to a specific Pi-hole v6 config API path. The initial supported settings SHALL be:

- `dns.upstream` → `dns/upstreams` (list of upstream DNS servers)
- `dns.cache.size` → `dns/cache/size` (cache entry count)
- `dns.cache.force_on_disk` → `dns/cache/optimizer` (force gravity DB to disk)
- `dns.rate_limit.count` → `dns/rateLimit/count` (queries per interval)
- `dns.rate_limit.interval` → `dns/rateLimit/interval` (rate limit window in seconds)
- `blocking.mode` → `dns/blocking/mode` (NULL, IP, NXDOMAIN)
- `privacy.level` → `misc/privacylevel` (0-4)

#### Scenario: Map Forseti setting to API path
- **WHEN** the effective settings contain `dns.cache.force_on_disk: true`
- **THEN** the system SHALL map this to Pi-hole config API path `dns/cache/optimizer` with value `true`

### Requirement: Settings validation
The system SHALL validate settings values at config load time. Invalid values (wrong type, out of range) SHALL produce validation errors before any API calls.

#### Scenario: Invalid blocking mode
- **WHEN** settings contain `blocking.mode: INVALID`
- **THEN** the system SHALL return a validation error listing valid modes (NULL, IP, NXDOMAIN)

#### Scenario: Negative cache size
- **WHEN** settings contain `dns.cache.size: -1`
- **THEN** the system SHALL return a validation error

### Requirement: Settings reconciliation (read actual state)
The system SHALL read the current Pi-hole configuration via `GET /api/config` and extract the values corresponding to declared Forseti settings. Only settings declared in the effective config are compared.

#### Scenario: Read current settings
- **WHEN** the system reconciles settings for a target
- **THEN** the system SHALL fetch the current config from the Pi-hole API and compare each declared setting against the actual value

### Requirement: Settings reconciliation (apply changes)
The system SHALL apply only changed settings to the Pi-hole via the config API. Unchanged settings SHALL not be written. Settings not declared in Forseti config SHALL not be touched.

#### Scenario: One setting differs
- **WHEN** desired `dns.cache.force_on_disk` is `true` but actual is `false`, and all other settings match
- **THEN** the system SHALL write only `dns/cache/optimizer = true` via the config API

#### Scenario: All settings match
- **WHEN** all declared settings already match the Pi-hole's actual config
- **THEN** the system SHALL make no config API write calls

#### Scenario: Undeclared settings preserved
- **WHEN** the Pi-hole has `misc/privacylevel = 3` but the Forseti config does not declare `privacy.level`
- **THEN** the system SHALL not modify `misc/privacylevel`

### Requirement: Settings plan output
In plan mode, the system SHALL display a diff of settings changes per target, showing the setting name, current value, and desired value.

#### Scenario: Plan with settings changes
- **WHEN** the user runs `forseti plan` and one target has settings differences
- **THEN** the output SHALL show each changed setting with its current and desired values
