## ADDED Requirements

### Requirement: Run freshness and failure metrics
The system SHALL expose, per target, in both modes:

- `forseti_reconcile_last_run_timestamp{target}` - unix time of the last completed (not skipped) run
- `forseti_reconcile_last_success_timestamp{target}` - unix time of the last run with status `success` or `partial`
- `forseti_target_consecutive_failures{target}` - number of consecutive failed runs (0 after a success)
- `forseti_reconcile_item_errors_total{target, type}` - counter of failed item writes per resource type (`other` for errors not tied to a resource)

#### Scenario: Alert on stale sync
- **WHEN** replica `pihole-2` has not completed a successful run for 30 minutes
- **THEN** `time() - forseti_reconcile_last_success_timestamp{target="pihole-2"}` SHALL exceed 1800

#### Scenario: Failures reset on success
- **WHEN** a target fails twice and then succeeds
- **THEN** `forseti_target_consecutive_failures` SHALL read 2 after the second failure and 0 after the success

#### Scenario: Item errors counted
- **WHEN** two client writes fail during a partial run on `pihole-2`
- **THEN** `forseti_reconcile_item_errors_total{target="pihole-2", type="clients"}` SHALL increase by 2
