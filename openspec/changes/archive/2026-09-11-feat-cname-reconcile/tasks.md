## 1. Config model

- [x] 1.1 Add `CNAMEEntry` struct with `Domain` and `Target` fields and yaml tags to `internal/config/config.go`
- [x] 1.2 Add `CNAME []CNAMEEntry` field to `Config` struct with `yaml:"cname"` tag
- [x] 1.3 Add `CNAMEPurge bool` field to `Config` (or `Reconcile`) struct with `yaml:"cname_purge"` tag
- [x] 1.4 Add config validation for CNAME entries (non-empty domain and target)
- [x] 1.5 Add config parsing tests for valid CNAME, empty CNAME, and invalid entries

## 2. Reconciler: CNAME diffing

- [x] 2.1 Add `ListCNAMERecords`, `AddCNAMERecord`, `DeleteCNAMERecord` to `PiholeAPI` interface
- [x] 2.2 Add `CNAME ResourceDiff` field to `DiffReport`
- [x] 2.3 Implement `diffCNAME(desired []config.CNAMEEntry, actual []pihole.APICNAMERecord, purge bool) ResourceDiff`
- [x] 2.4 Wire `diffCNAME` into `Plan()` after local DNS, before clients
- [x] 2.5 Wire CNAME add/delete into `Apply()` after local DNS, before clients
- [x] 2.6 Add FTL restart warning log when `DiffReport.CNAME.HasChanges()` is true

## 3. Tests

- [x] 3.1 Add unit tests for `diffCNAME`: add, delete, unchanged, mixed, additive-only, purge mode
- [x] 3.2 Update mock in reconciler tests to implement new `PiholeAPI` interface methods
- [x] 3.3 Add integration-level tests for Plan/Apply with CNAME entries

## 4. Verify

- [x] 4.1 Run `go test -race ./...` and confirm all tests pass
- [x] 4.2 Run `go vet ./...` and `golangci-lint run`
