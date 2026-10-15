## Purpose

Defines how a sync run from the primary to a replica is classified, recorded in metrics, protected by the per-target API gate, and how local DNS records are mirrored.

## ADDED Requirements

### Requirement: Sync run outcome classification
Every sync run against a replica SHALL be classified as exactly one of `success`, `partial` or `error`:

- `error`: the replica session could not be obtained or the replica state could not be read; no writes were attempted.
- `partial`: the replica state was read but at least one item-level create or delete failed.
- `success`: the replica state was read and every attempted write succeeded (including runs with no changes).

The outcome SHALL be recorded in `forseti_reconcile_runs_total{target, status}` for every run, together with the run duration in `forseti_reconcile_duration_seconds{target}`.

#### Scenario: Run without changes
- **WHEN** the replica already matches the primary for all synced resources
- **THEN** `forseti_reconcile_runs_total{target="<replica>", status="success"}` SHALL increment by 1

#### Scenario: Item write fails
- **WHEN** creating one adlist on the replica fails and all other writes succeed
- **THEN** `forseti_reconcile_runs_total{target="<replica>", status="partial"}` SHALL increment by 1 and the `success` series SHALL NOT increment

#### Scenario: Replica state cannot be read
- **WHEN** listing groups on the replica fails
- **THEN** `forseti_reconcile_runs_total{target="<replica>", status="error"}` SHALL increment by 1, no writes SHALL be attempted, and the replica session SHALL be invalidated

#### Scenario: Replica session unavailable
- **WHEN** no session can be obtained for the replica
- **THEN** `forseti_reconcile_runs_total{target="<replica>", status="error"}` SHALL increment by 1 and `forseti_target_reachable{target="<replica>"}` SHALL be 0

### Requirement: Sync change accounting
Sync mode SHALL record successfully applied writes in `forseti_reconcile_changes_total{target, type, action}` using the same `type` values as config mode (`groups`, `adlists`, `deny`, `allow`, `local_dns`, `clients`) and `action` values `add` and `delete`. Failed writes SHALL NOT be counted as changes.

#### Scenario: Adds and deletes counted per resource
- **WHEN** a sync run adds 2 clients and deletes 1 deny domain on replica `pihole-2`
- **THEN** `forseti_reconcile_changes_total{target="pihole-2", type="clients", action="add"}` SHALL increment by 2 and `{type="deny", action="delete"}` SHALL increment by 1

#### Scenario: Failed write not counted
- **WHEN** creating an adlist on the replica fails
- **THEN** `forseti_reconcile_changes_total{type="adlists", action="add"}` SHALL NOT increment for that item

### Requirement: Sync honours the API concurrency gate
Sync mode SHALL acquire the per-target API concurrency gate before making API calls to the primary or to a replica and SHALL release it when the access completes, including on error paths.

#### Scenario: Collector and sync on a single-slot target
- **WHEN** a replica has `max_concurrent: 1` and the stats collector holds the slot
- **THEN** the sync run for that replica SHALL wait until the slot is released before calling the API

### Requirement: Configurable local DNS mirroring
The `sync.local_dns_purge` option SHALL control whether sync deletes local DNS records on replicas that do not exist on the primary. It SHALL default to `true` when omitted. When `false`, sync SHALL only add missing records and SHALL never delete local DNS records on replicas. Because Pi-hole local DNS records carry no comment field, this option is the explicit exception to the managed-marker rule for sync mode.

#### Scenario: Default mirrors primary
- **WHEN** `sync.local_dns_purge` is omitted and a replica has local DNS record `10.0.0.9 manual.lan` that the primary does not have
- **THEN** sync SHALL delete `10.0.0.9 manual.lan` from the replica

#### Scenario: Purge disabled preserves replica-only records
- **WHEN** `sync.local_dns_purge` is `false` and a replica has local DNS record `10.0.0.9 manual.lan` that the primary does not have
- **THEN** sync SHALL NOT delete the record and SHALL still add records missing on the replica
