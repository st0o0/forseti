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
The reconciler SHALL process resource types in this fixed order: groups -> adlists -> deny domains -> allow domains -> local DNS -> clients. Groups MUST be reconciled first because adlists, domains, and clients reference groups by ID.

#### Scenario: Group created before client assignment
- **WHEN** config defines a new group `kids` and a client assigned to `kids`
- **THEN** the reconciler SHALL create the group before attempting to create the client

#### Scenario: Order enforced even on deletions
- **WHEN** a reconcile cycle includes both group deletions and client deletions
- **THEN** client deletions SHALL complete before group deletions to avoid referential integrity errors

### Requirement: Gravity trigger on adlist changes only
The reconciler SHALL trigger a gravity update (`POST /api/action/gravity`) only when adlists have been added or removed. Changes to domains, DNS records, groups, or clients SHALL NOT trigger gravity.

#### Scenario: Adlists changed
- **WHEN** the reconciler adds 2 adlists and removes 1
- **THEN** the reconciler SHALL trigger a gravity update after adlist reconciliation

#### Scenario: Only deny domains changed
- **WHEN** the reconciler adds deny domains but no adlists changed
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

### Requirement: Multi-target reconciliation
The reconciler SHALL apply the same desired state to all configured targets independently. A failure against one target SHALL NOT prevent reconciliation of other targets.

#### Scenario: One target unreachable
- **WHEN** target pihole-router is unreachable but pihole-pi is healthy
- **THEN** the reconciler SHALL reconcile pihole-pi successfully and report the failure for pihole-router
