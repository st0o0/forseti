## ADDED Requirements

### Requirement: Adlist group assignment drift detection

The adlist diff function SHALL detect when a managed adlist exists on the Pi-hole instance but its group assignments differ from the desired config. Such entries SHALL appear in the `Updates` slice of `ResourceDiff` rather than `Unchanged`.

#### Scenario: Adlist with changed groups is detected as update

- **WHEN** the desired config has adlist `https://example.com/list.txt` with groups `[ads, tracking]` and the actual Pi-hole state has the same adlist with groups `[ads]`
- **THEN** the diff SHALL produce one update entry with the adlist's key and ID, and zero adds or deletes for that entry

#### Scenario: Adlist with identical groups is unchanged

- **WHEN** the desired config has adlist `https://example.com/list.txt` with groups `[ads]` and the actual Pi-hole state has the same adlist with groups `[ads]`
- **THEN** the diff SHALL count the entry as unchanged and produce no update entry

#### Scenario: Group order does not trigger update

- **WHEN** the desired config has groups `[tracking, ads]` and the actual state has the same adlist with group IDs resolving to `[ads, tracking]`
- **THEN** the diff SHALL count the entry as unchanged (order-independent comparison)

### Requirement: Client group assignment drift detection

The client diff function SHALL detect when a managed client exists on the Pi-hole instance but its group assignments differ from the desired config.

#### Scenario: Client with changed groups is detected as update

- **WHEN** the desired config has client `192.168.1.100` with groups `[trusted]` and the actual Pi-hole state has the same client with groups `[default]`
- **THEN** the diff SHALL produce one update entry with the client's key and ID

#### Scenario: Unmanaged client with different groups is not an update

- **WHEN** the actual Pi-hole state has client `10.0.0.1` without the managed marker and that client is not in the desired config
- **THEN** the diff SHALL not produce an update, add, or delete entry for that client

### Requirement: Domain group assignment drift detection

The domain diff functions (deny and allow) SHALL detect when a managed domain exists but its group assignments differ from the desired config.

#### Scenario: Deny domain with changed groups is detected as update

- **WHEN** the desired config has deny domain `ads.example.com` with groups `[ads]` and the actual state has the same domain with different groups
- **THEN** the diff SHALL produce one update entry

### Requirement: ResourceDiff includes Updates

`ResourceDiff` SHALL have an `Updates` field of type `[]DiffEntry`. `HasChanges()` SHALL return true when `Updates` is non-empty, even if `Adds` and `Deletes` are empty.

#### Scenario: HasChanges with only updates

- **WHEN** a `ResourceDiff` has zero adds, zero deletes, and one update
- **THEN** `HasChanges()` SHALL return true

### Requirement: Pi-hole client update methods

The Pi-hole client SHALL provide `UpdateAdlist(id int, groups []int)`, `UpdateClient(id int, groups []int)`, and `UpdateDomain(id int, groups []int)` methods that send PUT requests to update group assignments on existing resources.

#### Scenario: UpdateAdlist sends PUT request

- **WHEN** `UpdateAdlist(42, []int{1, 3})` is called
- **THEN** the client SHALL send `PUT /api/lists/42` with body containing `{"groups": [1, 3]}`

#### Scenario: UpdateClient sends PUT request

- **WHEN** `UpdateClient(7, []int{1})` is called
- **THEN** the client SHALL send `PUT /api/clients/7` with body containing `{"groups": [1]}`

### Requirement: Apply processes updates

The `Apply()` function SHALL process update entries by calling the corresponding update API method with resolved group IDs. Updates SHALL be processed after adds and before deletes.

#### Scenario: Apply updates adlist groups

- **WHEN** the diff contains an update for adlist `https://example.com/list.txt` with ID 42 and the desired groups resolve to `[1, 3]`
- **THEN** `Apply()` SHALL call `UpdateAdlist(42, [1, 3])`
