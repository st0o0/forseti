## ADDED Requirements

### Requirement: Change counters count applied changes
`forseti_reconcile_changes_total` SHALL count only changes that were successfully applied, in both modes. Planned changes whose write failed SHALL NOT be counted. For a failed batch delete, every item in the batch SHALL be treated as failed.

#### Scenario: Failed create in config mode
- **WHEN** config mode plans 3 adlist additions on `pihole-router` and one of the creates fails
- **THEN** `forseti_reconcile_changes_total{target="pihole-router", type="adlists", action="add"}` SHALL increment by 2

#### Scenario: Failed batch delete
- **WHEN** a batch delete of 4 deny domains fails
- **THEN** `forseti_reconcile_changes_total{type="deny", action="delete"}` SHALL NOT increment for that batch
