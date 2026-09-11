## MODIFIED Requirements

### Requirement: YAML configuration format
The system SHALL accept a single YAML file as its primary configuration source. The file defines metrics settings, Pi-hole targets, reconcile behavior, Pi-hole settings, and all managed resources (adlists, deny domains, allow domains, local DNS records, groups, clients). Targets MAY reference external override files via the `file:` field.

#### Scenario: Valid config with all sections
- **WHEN** a YAML file contains metrics, targets, reconcile, settings, adlists, deny, allow, local_dns, groups, and clients sections
- **THEN** the system SHALL parse all sections and make them available to the reconciler

#### Scenario: Minimal config
- **WHEN** a YAML file contains only targets (with at least one entry) and no resource or settings sections
- **THEN** the system SHALL accept the config with empty resource lists and no settings reconciliation

### Requirement: Multi-target support
The system SHALL support an array of Pi-hole targets under the `targets` key. Each target has a name, URL, password, and an optional `file` path to an override config. The effective config for each target is computed by merging the global config with the target's override file (if present). Downstream reconciliation receives the effective config per target.

#### Scenario: Two targets with different effective configs
- **WHEN** config defines two targets, one with `file: targets/kids.yaml` containing extra deny entries and one without a `file:` field
- **THEN** the first target's effective deny list SHALL include the extra entries, and the second target's effective deny list SHALL equal the global deny list

#### Scenario: Two targets without overrides
- **WHEN** config defines two targets, neither with a `file:` field
- **THEN** the reconciler SHALL apply the same desired state to both instances (existing behavior)
