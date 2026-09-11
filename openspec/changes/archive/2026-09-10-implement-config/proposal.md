## Why

The config package (`internal/config`) is currently an empty stub. It is the foundation for every other package — the reconciler, API client, and metrics server all depend on a parsed and validated configuration. Implementing it first unblocks all downstream work.

## What Changes

- Define Go struct types for the full YAML configuration schema: targets, reconcile settings, metrics, adlists, deny/allow domains, local DNS records, groups, and clients
- Implement environment variable expansion (`${VAR}` syntax) on raw YAML content before parsing
- Implement YAML parsing via `gopkg.in/yaml.v3`
- Implement validation: target URL format, domain name format, CIDR notation, group reference resolution, duplicate detection across resource types
- Expose a `Load(path string) (*Config, error)` function as the public API
- Add comprehensive unit tests covering happy paths, edge cases, and validation errors

## Capabilities

### New Capabilities

None. This change implements the existing `config` spec — no new capabilities introduced.

### Modified Capabilities

None. The `config` spec requirements are unchanged.

## Impact

- `internal/config/config.go` — full implementation replacing the empty stub
- `internal/config/config_test.go` — new test file
- `go.mod` / `go.sum` — `gopkg.in/yaml.v3` dependency will be restored by `go mod tidy`
- All downstream packages (`pihole`, `reconcile`, `metrics`, `cmd/forseti`) will import config types
