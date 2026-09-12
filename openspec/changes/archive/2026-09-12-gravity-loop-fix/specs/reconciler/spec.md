# Reconciler (delta)

## MODIFIED Requirements

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
