## MODIFIED Requirements

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

## ADDED Requirements

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
