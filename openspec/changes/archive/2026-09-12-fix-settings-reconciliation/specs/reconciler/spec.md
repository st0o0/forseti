## MODIFIED Requirements

### Requirement: Reconcile ordering
The reconciler SHALL process resource types in this fixed order: settings → groups → adlists → deny domains → allow domains → local DNS → CNAME → clients. Settings MUST be reconciled first because some settings (e.g., force_on_disk) affect how the Pi-hole processes subsequent content changes. Groups MUST be reconciled before content types that reference groups.

A failure to apply one or more settings SHALL NOT prevent reconciliation of remaining settings or subsequent resource types. The reconciler SHALL collect all settings errors and log them, then proceed with group reconciliation.

#### Scenario: Settings applied before content
- **WHEN** the effective config for a target has both settings changes and adlist changes
- **THEN** the reconciler SHALL apply settings changes first, then proceed with content reconciliation in the established order

#### Scenario: Partial settings failure
- **WHEN** applying setting `dns/cache/optimizer` fails with an API error but `dns/listeningMode` and `dns/cnameDeepInspect` also need updating
- **THEN** the reconciler SHALL continue applying `dns/listeningMode` and `dns/cnameDeepInspect`, collect all errors, and proceed with group reconciliation

#### Scenario: No settings declared
- **WHEN** the effective config for a target has no settings section
- **THEN** the reconciler SHALL skip the settings step and proceed directly with groups

#### Scenario: Group created before client assignment
- **WHEN** config defines a new group `kids` and a client assigned to `kids`
- **THEN** the reconciler SHALL create the group before attempting to create the client

#### Scenario: Order enforced even on deletions
- **WHEN** a reconcile cycle includes both group deletions and client deletions
- **THEN** client deletions SHALL complete before group deletions to avoid referential integrity errors
