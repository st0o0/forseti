## ADDED Requirements

### Requirement: Cycle outcome is always recorded
After every reconcile cycle the worker SHALL record the run outcome (`success`, `partial` or `error`) and the resulting health state in metrics, on every exit path: session failure, readiness failure, settings failure, content failure, and success. A cycle whose content apply returned item-level errors SHALL be recorded as `partial` and SHALL count as a success for health and backoff purposes, because the target was reachable.

#### Scenario: Readiness failure recorded
- **WHEN** the readiness check fails for target `pizero`
- **THEN** the worker SHALL record run status `error`, mark the target unreachable, transition health to degraded, and publish the degraded health state

#### Scenario: Content apply error recorded
- **WHEN** content reconciliation returns an error for target `pizero`
- **THEN** the worker SHALL record run status `error` and publish the degraded health state

#### Scenario: Item-level errors are partial
- **WHEN** content reconciliation succeeds overall but reports one failed item write
- **THEN** the worker SHALL record run status `partial`, keep health at healthy, and not activate backoff
