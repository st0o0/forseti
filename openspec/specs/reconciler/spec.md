# Reconciler

Reconcile logic: marker-based ownership, three-way diff, ordering constraints, gravity trigger, plan/apply/watch modes.

### Requirement: Marker-based ownership
The reconciler SHALL only manage entries tagged with the configured marker (default `[forseti]`) in the comment field. Entries without the marker are considered manually created and SHALL never be modified or deleted.

#### Scenario: Three-way diff
- **WHEN** desired state is `[A, B, C]` and actual state is `[A, C, D[forseti], E]`
- **THEN** the reconciler SHALL: skip A (exists), add B with marker, skip C (exists), delete D (has marker but not in desired), leave E alone (no marker)

#### Scenario: Manual UI entries preserved
- **WHEN** a user manually adds an adlist through the Pi-hole UI (no marker)
- **THEN** the reconciler SHALL never touch that entry, even if it is not in the YAML config

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

### Requirement: Gravity trigger on adlist changes only
The reconciler SHALL trigger a gravity update (`POST /api/action/gravity`) only when adlists have been actually added, updated, or successfully deleted (non-404). Changes to domains, DNS records, groups, or clients SHALL NOT trigger gravity. Phantom deletes (item already gone, 404) SHALL NOT trigger gravity.

#### Scenario: Adlists changed
- **WHEN** the reconciler adds 2 adlists and removes 1 successfully
- **THEN** the reconciler SHALL trigger a gravity update after adlist reconciliation

#### Scenario: Only deny domains changed
- **WHEN** the reconciler adds deny domains but no adlists changed
- **THEN** the reconciler SHALL NOT trigger a gravity update

#### Scenario: Only phantom deletes
- **WHEN** the reconciler's diff includes adlist deletes but all return 404
- **THEN** the reconciler SHALL NOT trigger a gravity update

### Requirement: Plan mode (dry-run)
The `plan` command SHALL compute the full diff between desired and actual state, display it to the user, and exit without making any changes.

#### Scenario: Plan output
- **WHEN** the user runs `forseti plan --config forseti.yml`
- **THEN** the system SHALL display additions, deletions, and unchanged counts per resource type per target

### Requirement: Apply mode
The `apply` command SHALL compute the diff and execute all changes against every configured target. It SHALL report what was changed per target.

#### Scenario: Apply to multiple targets
- **WHEN** config defines two targets and desired state differs from both
- **THEN** the reconciler SHALL apply changes to each target independently and report results per target

### Requirement: Watch mode (daemon)
The `watch` command SHALL run as a long-lived daemon that periodically executes a reconcile cycle (interval from config) and serves the Prometheus metrics endpoint.

#### Scenario: Periodic reconcile
- **WHEN** the reconcile interval is `5m`
- **THEN** the system SHALL reconcile every 5 minutes and keep the metrics server running between cycles

### Requirement: Idempotent delete handling
The reconciler SHALL treat a 404 response on any delete operation (adlists, domains, groups, clients, DNS records) as a successful deletion. The reconciler SHALL use `pihole.IsNotFound(err)` to detect this condition.

#### Scenario: Batch delete returns 404 for absent item
- **WHEN** the reconciler calls batch delete for adlists and Pi-hole returns 404
- **THEN** the reconciler SHALL NOT append an error to the reconcile report

### Requirement: Retry transient list errors
The reconciler SHALL retry list operations with exponential backoff (1s, 2s, 4s, max 3 attempts) when the error is transient as determined by `pihole.IsTransient(err)`. Each retry attempt SHALL be logged at WARN level with the target name and operation.

#### Scenario: Database temporarily unavailable
- **WHEN** `ListDomains("deny","exact")` returns 400 "Database not available" during FTL restart
- **THEN** the reconciler SHALL retry up to 3 times before failing the target

### Requirement: Multi-target reconciliation
The reconciler SHALL apply each target's effective config independently. A failure against one target SHALL NOT prevent reconciliation of other targets. Each target's effective config MAY differ due to per-target overrides. Transient errors on list operations SHALL be retried before declaring a target as failed.

#### Scenario: One target unreachable
- **WHEN** target pihole-router is unreachable but pihole-pi is healthy
- **THEN** the reconciler SHALL reconcile pihole-pi with its effective config successfully and report the failure for pihole-router

#### Scenario: Targets with different effective configs
- **WHEN** pihole-kids has extra deny entries from its override file and pihole-office uses pure global defaults
- **THEN** the reconciler SHALL apply the extended deny list to pihole-kids and the base deny list to pihole-office

#### Scenario: Target recovers on retry
- **WHEN** a target's list operation fails with a transient error but succeeds on retry
- **THEN** the reconciler SHALL proceed with reconciliation for that target

### Requirement: Reconciler exposed through interface
The reconciler SHALL expose its settings and content operations through interfaces (`SettingsReconciler` and `ContentReconciler`) that can be consumed by the TargetWorker and mocked in tests.

#### Scenario: Worker calls content reconciler
- **WHEN** the worker needs to reconcile content for a target
- **THEN** it SHALL call the ContentReconciler interface, not the reconcile package directly

#### Scenario: Test mocks reconciler
- **WHEN** a test creates a worker with a mock ContentReconciler
- **THEN** the worker SHALL use the mock and the test SHALL verify the interaction
