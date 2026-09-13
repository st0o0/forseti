# Config

YAML configuration parsing, environment variable expansion, validation, and multi-target support.

### Requirement: YAML configuration format
The system SHALL accept a single YAML file as its primary configuration source. The file defines metrics settings (including collector toggles), Pi-hole targets, reconcile behavior, Pi-hole settings (DNS, blocking, DHCP, webserver, privacy, misc), and all managed resources (adlists, deny domains, allow domains, local DNS records, groups, clients). Targets MAY reference external override files via the `file:` field. Each target MAY include an `api` block with `timeout` (duration) and `max_concurrent` (integer) fields to control per-target API behavior.

#### Scenario: Valid config with all sections
- **WHEN** a YAML file contains metrics (with collectors), targets, reconcile, settings (with dns, blocking, dhcp, webserver, privacy, misc), adlists, deny, allow, local_dns, groups, and clients sections
- **THEN** the system SHALL parse all sections and make them available to the reconciler

#### Scenario: Minimal config
- **WHEN** a YAML file contains only targets (with at least one entry) and no resource, settings, or collectors sections
- **THEN** the system SHALL accept the config with empty resource lists, no settings reconciliation, and all collectors enabled

#### Scenario: Target with API config
- **WHEN** a target includes `api.timeout: 45s` and `api.max_concurrent: 1`
- **THEN** the parsed target SHALL have APIConfig with Timeout=45s and MaxConcurrent=1

#### Scenario: Target without API config
- **WHEN** a target omits the `api` block entirely
- **THEN** the parsed target SHALL use defaults: Timeout=30s, MaxConcurrent=4

### Requirement: Environment variable expansion
The system SHALL expand `${VAR}` references in string values using environment variables before YAML parsing. This enables secret injection (passwords) without storing them in the config file.

#### Scenario: Password from environment
- **WHEN** a target password is `${PIHOLE_PASSWORD}` and the environment variable `PIHOLE_PASSWORD` is set to `secret123`
- **THEN** the parsed target password SHALL be `secret123`

#### Scenario: Undefined variable
- **WHEN** a `${VAR}` reference has no matching environment variable
- **THEN** the system SHALL return a validation error naming the undefined variable

### Requirement: Multi-target support
The system SHALL support an array of Pi-hole targets under the `targets` key. Each target has a name, URL, password, and an optional `file` path to an override config. The effective config for each target is computed by merging the global config with the target's override file (if present). Downstream reconciliation receives the effective config per target.

#### Scenario: Two targets with different effective configs
- **WHEN** config defines two targets, one with `file: targets/kids.yaml` containing extra deny entries and one without a `file:` field
- **THEN** the first target's effective deny list SHALL include the extra entries, and the second target's effective deny list SHALL equal the global deny list

#### Scenario: Two targets without overrides
- **WHEN** config defines two targets, neither with a `file:` field
- **THEN** the reconciler SHALL apply the same desired state to both instances (existing behavior)

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

### Requirement: Collector toggles configuration section
The system SHALL accept a `metrics.collectors` section with boolean fields for each collector area. Each field SHALL default to `true` when omitted.

#### Scenario: Parse collector toggles
- **WHEN** the config contains `metrics.collectors.stats: false` and `metrics.collectors.dhcp: false`
- **THEN** the parsed config SHALL have `Stats` and `DHCP` toggles set to `false` and all others defaulting to `true`

#### Scenario: No collectors section
- **WHEN** the config omits `metrics.collectors` entirely
- **THEN** all collector toggles SHALL default to `true`

### Requirement: Extended settings sections
The config SHALL accept new settings subsections for DHCP, webserver, and misc, in addition to the existing DNS, blocking, and privacy sections. All new fields use pointer types to distinguish "not set" from zero values.

#### Scenario: DHCP settings parsed
- **WHEN** the config contains `settings.dhcp.active: true` and `settings.dhcp.start: "192.168.1.100"`
- **THEN** the parsed config SHALL have DHCPSettings with Active=true and Start="192.168.1.100"

#### Scenario: Webserver settings parsed
- **WHEN** the config contains `settings.webserver.port: 8080`
- **THEN** the parsed config SHALL have WebserverSettings with Port=8080

#### Scenario: Misc settings parsed
- **WHEN** the config contains `settings.misc.nice: -10` and `settings.misc.check.disk: 90`
- **THEN** the parsed config SHALL have MiscSettings with Nice=-10 and Check.Disk=90

### Requirement: Extended settings validation
The system SHALL validate all new settings fields at config load time.

#### Scenario: DHCP start without end
- **WHEN** settings contain `dhcp.start: "192.168.1.100"` but no `dhcp.end`
- **THEN** the system SHALL return a validation error

#### Scenario: Valid DHCP range
- **WHEN** settings contain `dhcp.start: "192.168.1.100"` and `dhcp.end: "192.168.1.200"` and `dhcp.router: "192.168.1.1"`
- **THEN** the system SHALL accept the config

#### Scenario: DNS port range
- **WHEN** settings contain `dns.port: 0`
- **THEN** the system SHALL return a validation error (must be 1-65535)

#### Scenario: Listening mode validation
- **WHEN** settings contain `dns.listening_mode: "all"`
- **THEN** the system SHALL accept the config (valid modes: local, all, bind)

### Requirement: API config validation
The system SHALL validate per-target API configuration at config load time. `timeout` MUST be a positive duration. `max_concurrent` MUST be a positive integer (minimum 1).

#### Scenario: Invalid timeout
- **WHEN** a target specifies `api.timeout: 0s`
- **THEN** the system SHALL return a validation error

#### Scenario: Invalid max_concurrent
- **WHEN** a target specifies `api.max_concurrent: 0`
- **THEN** the system SHALL return a validation error

#### Scenario: Negative timeout
- **WHEN** a target specifies `api.timeout: -5s`
- **THEN** the system SHALL return a validation error

### Requirement: Hot-reload syncs worker lifecycle
When the config file changes and hot-reload triggers, the system SHALL synchronize the worker map with the new config. Existing targets SHALL receive config updates without state loss. New targets SHALL get new workers. Removed targets SHALL have their workers stopped and cleaned up.

#### Scenario: Target added during hot-reload
- **WHEN** a new target "pihole-office" is added to the YAML config
- **THEN** the system SHALL create a new worker for it and include it in the next reconcile cycle without restarting

#### Scenario: Target removed during hot-reload
- **WHEN** target "pihole-old" is removed from the YAML config
- **THEN** the system SHALL stop and remove its worker, releasing the session

#### Scenario: Target config changed during hot-reload
- **WHEN** target "mikrotik" has its adlists changed in the YAML config
- **THEN** the system SHALL update the worker's resolved config while preserving its health state and backoff timer
