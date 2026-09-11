## Context

The `internal/config` package is an empty stub (`package config` only). The config spec defines YAML parsing, env-var expansion, multi-target support, and validation. This package has no dependencies on other internal packages and is imported by all of them, making it the natural first implementation target.

The dev config file (`forseti.dev.yml`) already exists as a reference for the expected YAML structure.

## Goals / Non-Goals

**Goals:**
- Single `Load(path) (*Config, error)` entry point that reads, expands, parses, and validates
- Struct types that serve as the shared vocabulary for all other packages
- Thorough validation with actionable error messages
- Unit tests covering parsing, expansion, and all validation rules

**Non-Goals:**
- Config hot-reload (watch mode re-reads the file each cycle)
- Config file generation or migration tooling
- Schema versioning in the YAML file

## Decisions

### Env expansion before YAML parsing
Expand `${VAR}` references via `os.ExpandEnv` on the raw file bytes before feeding to `yaml.v3`. This is simpler than a custom YAML unmarshaler and handles variables in any position (keys, values, nested). Alternative: custom YAML node walker — rejected because it adds complexity for no benefit.

### Flat struct hierarchy
```
Config
├── Metrics     (port, path, scrape_interval)
├── Targets[]   (name, url, password)
├── Reconcile   (interval, marker, gravity_on_change)
├── Groups[]    (name, comment)
├── Adlists[]   (url, comment, groups[])
├── Deny[]      (domain)
├── Allow[]     (domain)
├── LocalDNS[]  (domain, ip)
└── Clients[]   (match, comment, groups[])
```
Structs are exported so other packages can import them directly. No interfaces — this is a data-only package.

### Validation as a separate pass
`Load` calls `parse` then `validate` as distinct steps. Validation collects all errors (not fail-fast) and returns them joined. This gives the user a complete picture of what's wrong. Alternative: validate during unmarshal — rejected because it couples parsing with business rules and makes testing harder.

### Duration fields as `time.Duration`
`interval` and `scrape_interval` are unmarshaled as `time.Duration` via a custom `UnmarshalYAML` on a wrapper type. YAML values like `30s`, `5m` are parsed by `time.ParseDuration`.

### Defaults
- `reconcile.marker`: `[forseti]`
- `reconcile.gravity_on_change`: `true`
- `metrics.port`: `9099`
- `metrics.path`: `/metrics`
- `metrics.scrape_interval`: `30s`

Applied after parsing, before validation.

## Risks / Trade-offs

- **`os.ExpandEnv` expands all env vars, not just `${}`** → Use `os.Expand` with a custom mapping function that only matches `${VAR}` pattern and returns error for undefined variables
- **No schema version** → If the YAML format changes in the future, we'll need migration. Acceptable for now since v1 hasn't shipped yet.
- **Validation completeness** → Domain and URL validation use `net/url` and basic regex rather than full RFC compliance. Sufficient for the use case.
