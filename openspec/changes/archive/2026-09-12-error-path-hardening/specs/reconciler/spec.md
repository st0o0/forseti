# Reconciler (delta)

## ADDED Requirements

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

## MODIFIED Requirements

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
