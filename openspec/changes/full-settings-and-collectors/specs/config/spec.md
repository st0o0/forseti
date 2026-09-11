## MODIFIED Requirements

### Requirement: YAML configuration format
The system SHALL accept a single YAML file as its primary configuration source. The file defines metrics settings (including collector toggles), Pi-hole targets, reconcile behavior, Pi-hole settings (DNS, blocking, DHCP, webserver, privacy, misc), and all managed resources (adlists, deny domains, allow domains, local DNS records, groups, clients). Targets MAY reference external override files via the `file:` field.

#### Scenario: Valid config with all sections
- **WHEN** a YAML file contains metrics (with collectors), targets, reconcile, settings (with dns, blocking, dhcp, webserver, privacy, misc), adlists, deny, allow, local_dns, groups, and clients sections
- **THEN** the system SHALL parse all sections and make them available to the reconciler

#### Scenario: Minimal config
- **WHEN** a YAML file contains only targets (with at least one entry) and no resource, settings, or collectors sections
- **THEN** the system SHALL accept the config with empty resource lists, no settings reconciliation, and all collectors enabled

## ADDED Requirements

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
