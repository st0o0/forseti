## ADDED Requirements

### Requirement: Negative duration values are rejected

The config loader SHALL return a validation error when `reconcile.interval`, `sync.interval`, or `metrics.scrape_interval` is explicitly set to a negative value.

#### Scenario: Negative reconcile interval

- **WHEN** the config YAML contains `reconcile.interval: -1m`
- **THEN** `Load()` SHALL return an error containing "reconcile.interval must be at least 10s"

### Requirement: Sub-minimum duration values are rejected

The config loader SHALL enforce minimum bounds: 10s for `reconcile.interval` and `sync.interval`, 5s for `metrics.scrape_interval`.

#### Scenario: Reconcile interval below minimum

- **WHEN** the config YAML contains `reconcile.interval: 1s`
- **THEN** `Load()` SHALL return an error containing "reconcile.interval must be at least 10s"

#### Scenario: Scrape interval below minimum

- **WHEN** the config YAML contains `metrics.scrape_interval: 2s`
- **THEN** `Load()` SHALL return an error containing "scrape_interval must be at least 5s"

#### Scenario: Valid intervals pass validation

- **WHEN** the config YAML contains `reconcile.interval: 30s` and `metrics.scrape_interval: 10s`
- **THEN** `Load()` SHALL succeed without validation errors

### Requirement: Omitted intervals use defaults and pass validation

When duration fields are omitted, the defaults applied by `applyDefaults()` SHALL be above the minimum bounds.

#### Scenario: Default intervals pass

- **WHEN** the config YAML omits all interval fields
- **THEN** `Load()` SHALL succeed and the intervals SHALL have their default values (5m reconcile, 5m sync, 30s scrape)
