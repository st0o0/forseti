## Why

Missing input validation allows invalid configurations to pass silently: `reconcile.interval` and `sync.interval` accept zero or negative durations, which would cause tight loops or panics. Additionally, the project has no `.gitignore`, leaving coverage output files (`*_cov.out`, `coverage/`) and build artifacts untracked but visible in `git status`.

## What Changes

- **Interval validation**: Reject `reconcile.interval`, `sync.interval`, and `metrics.scrape_interval` values that are zero or negative after defaults are applied
- **Minimum interval guard**: Enforce a minimum of 10 seconds for reconcile/sync intervals and 5 seconds for scrape interval to prevent accidental tight loops
- **`.gitignore`**: Add a `.gitignore` covering coverage outputs, Go build artifacts, and editor files

## Capabilities

### New Capabilities

- `config-interval-validation`: Validation rules for duration-based config fields with minimum bounds

### Modified Capabilities

_(none)_

## Impact

- `internal/config/config.go`: additional validation in `validate()` function
- `internal/config/config_test.go`: new test cases for invalid intervals
- `.gitignore`: new file in project root
