# Reconciler (delta)

## ADDED Requirements

### Requirement: Reconciler exposed through interface
The reconciler SHALL expose its settings and content operations through interfaces (`SettingsReconciler` and `ContentReconciler`) that can be consumed by the TargetWorker and mocked in tests.

#### Scenario: Worker calls content reconciler
- **WHEN** the worker needs to reconcile content for a target
- **THEN** it SHALL call the ContentReconciler interface, not the reconcile package directly

#### Scenario: Test mocks reconciler
- **WHEN** a test creates a worker with a mock ContentReconciler
- **THEN** the worker SHALL use the mock and the test SHALL verify the interaction
