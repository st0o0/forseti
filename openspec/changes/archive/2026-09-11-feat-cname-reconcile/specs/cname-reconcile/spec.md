## ADDED Requirements

### Requirement: CNAME entries in config model

The config model SHALL support a `cname` YAML field containing a list of entries, each with `domain` (string) and `target` (string) fields.

#### Scenario: Valid CNAME config

- **WHEN** the YAML config contains `cname: [{domain: "app.local", target: "server.local"}]`
- **THEN** `Load()` SHALL parse it into `Config.CNAME` with one `CNAMEEntry{Domain: "app.local", Target: "server.local"}`

#### Scenario: Empty CNAME config

- **WHEN** the YAML config omits the `cname` field
- **THEN** `Config.CNAME` SHALL be nil/empty and no CNAME reconciliation SHALL occur

### Requirement: CNAME diffing defaults to additive-only

The CNAME diff engine SHALL only produce add operations by default. Deletion of CNAME records not present in the desired state SHALL NOT occur unless `cname_purge` is true.

#### Scenario: Add missing CNAME record

- **WHEN** the desired state contains `[app.local → server.local]` and the actual Pi-hole state contains no CNAME records
- **THEN** the diff SHALL produce one addition with key `"app.local,server.local"`

#### Scenario: Preserve unmanaged CNAME in additive mode

- **WHEN** the desired state contains `[app.local → server.local]` and the actual state contains `[app.local → server.local, old.local → legacy.local]` and `cname_purge` is false
- **THEN** the diff SHALL produce zero additions and zero deletions

#### Scenario: Purge mode deletes unmanaged CNAME records

- **WHEN** the desired state contains `[app.local → server.local]` and the actual state contains `[app.local → server.local, old.local → legacy.local]` and `cname_purge` is true
- **THEN** the diff SHALL produce one deletion with key `"old.local,legacy.local"`

### Requirement: CNAME reconcile order

CNAME reconciliation SHALL execute after local DNS and before clients in both `Plan()` and `Apply()`.

#### Scenario: Plan includes CNAME diff

- **WHEN** `Plan()` is called with a config containing CNAME entries
- **THEN** `DiffReport.CNAME` SHALL contain the computed diff

#### Scenario: Apply creates and deletes CNAME records

- **WHEN** `Apply()` is called and the CNAME diff contains adds and deletes
- **THEN** forseti SHALL call `AddCNAMERecord` for each add and `DeleteCNAMERecord` for each delete

### Requirement: FTL restart warning on CNAME changes

When CNAME changes are detected, forseti SHALL log a warning indicating that FTL will restart and a brief DNS outage will occur.

#### Scenario: Warning logged on CNAME add

- **WHEN** the CNAME diff contains at least one add or delete
- **THEN** a log message SHALL be emitted containing "CNAME changes will trigger FTL restart"

#### Scenario: No warning when CNAME unchanged

- **WHEN** the CNAME diff has no changes
- **THEN** no FTL restart warning SHALL be logged

### Requirement: PiholeAPI interface includes CNAME methods

The `PiholeAPI` interface SHALL include `ListCNAMERecords`, `AddCNAMERecord`, and `DeleteCNAMERecord` methods.

#### Scenario: Interface satisfies client

- **WHEN** the `pihole.Client` implements the updated `PiholeAPI` interface
- **THEN** compilation SHALL succeed without errors
