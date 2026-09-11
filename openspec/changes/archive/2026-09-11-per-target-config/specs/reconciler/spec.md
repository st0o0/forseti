## MODIFIED Requirements

### Requirement: Reconcile ordering
The reconciler SHALL process resource types in this fixed order: settings → groups → adlists → deny domains → allow domains → local DNS → CNAME → clients. Settings MUST be reconciled first because some settings (e.g., force_on_disk) affect how the Pi-hole processes subsequent content changes. Groups MUST be reconciled before content types that reference groups.

#### Scenario: Settings applied before content
- **WHEN** the effective config for a target has both settings changes and adlist changes
- **THEN** the reconciler SHALL apply settings changes first, then proceed with content reconciliation in the established order

#### Scenario: No settings declared
- **WHEN** the effective config for a target has no settings section
- **THEN** the reconciler SHALL skip the settings step and proceed directly with groups

### Requirement: Multi-target reconciliation
The reconciler SHALL apply each target's effective config independently. A failure against one target SHALL NOT prevent reconciliation of other targets. Each target's effective config MAY differ due to per-target overrides.

#### Scenario: One target unreachable
- **WHEN** target pihole-router is unreachable but pihole-pi is healthy
- **THEN** the reconciler SHALL reconcile pihole-pi with its effective config successfully and report the failure for pihole-router

#### Scenario: Targets with different effective configs
- **WHEN** pihole-kids has extra deny entries from its override file and pihole-office uses pure global defaults
- **THEN** the reconciler SHALL apply the extended deny list to pihole-kids and the base deny list to pihole-office
