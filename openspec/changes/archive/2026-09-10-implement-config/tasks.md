## 1. Struct Types

- [x] 1.1 Define Config, Metrics, Target, Reconcile, Group, Adlist, DenyEntry, AllowEntry, LocalDNSEntry, ClientEntry structs with YAML tags
- [x] 1.2 Implement Duration wrapper type with custom UnmarshalYAML for `time.Duration` parsing

## 2. Env Expansion and Parsing

- [x] 2.1 Implement `expandEnv` function using `os.Expand` with custom mapper that errors on undefined `${VAR}` references
- [x] 2.2 Implement `parse` function: read file, expand env vars, unmarshal YAML, apply defaults

## 3. Validation

- [x] 3.1 Implement target validation: at least one target, valid HTTP(S) URLs, non-empty passwords
- [x] 3.2 Implement domain validation for deny/allow entries (valid domain name format)
- [x] 3.3 Implement CIDR validation for client match fields
- [x] 3.4 Implement group reference validation: groups referenced by adlists/clients must exist in the groups list
- [x] 3.5 Implement duplicate detection: no duplicate adlist URLs, no duplicate deny/allow domains, no duplicate group names

## 4. Public API

- [x] 4.1 Implement `Load(path string) (*Config, error)` that calls expandEnv, parse, applyDefaults, and validate

## 5. Tests

- [x] 5.1 Test valid config parsing with all sections populated
- [x] 5.2 Test minimal config (targets only, empty resource lists)
- [x] 5.3 Test env var expansion (set var, undefined var error)
- [x] 5.4 Test duration parsing (valid durations, invalid format)
- [x] 5.5 Test validation errors: invalid URL, invalid domain, invalid CIDR, missing group reference, duplicates
- [x] 5.6 Test defaults are applied when fields are omitted
- [x] 5.7 Run `go vet` and `golangci-lint run` clean
