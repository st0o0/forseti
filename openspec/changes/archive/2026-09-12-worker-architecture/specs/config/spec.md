# Config (delta)

## MODIFIED Requirements

### Requirement: Hot-reload syncs worker lifecycle
When the config file changes and hot-reload triggers, the system SHALL synchronize the worker map with the new config. Existing targets SHALL receive config updates without state loss. New targets SHALL get new workers. Removed targets SHALL have their workers stopped and cleaned up.

#### Scenario: Target added during hot-reload
- **WHEN** a new target "pihole-office" is added to the YAML config
- **THEN** the system SHALL create a new worker for it and include it in the next reconcile cycle without restarting

#### Scenario: Target removed during hot-reload
- **WHEN** target "pihole-old" is removed from the YAML config
- **THEN** the system SHALL stop and remove its worker, releasing the session

#### Scenario: Target config changed during hot-reload
- **WHEN** target "mikrotik" has its adlists changed in the YAML config
- **THEN** the system SHALL update the worker's resolved config while preserving its health state and backoff timer
