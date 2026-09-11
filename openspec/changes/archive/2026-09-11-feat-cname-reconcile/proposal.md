## Why

Forseti's Pi-hole client already implements `ListCNAMERecords`, `AddCNAMERecord`, and `DeleteCNAMERecord`, but neither the config model nor the reconciler uses them. Users who manage CNAME records declaratively must do so manually through the Pi-hole UI, defeating forseti's purpose as a GitOps controller for all gravity.db content. CNAME is the last gap in DNS-related resource coverage.

## What Changes

- **Config model**: Add a `cname` field to `Config` accepting a list of `{domain, target}` entries
- **Reconcile engine**: Add `diffCNAME` function following the same additive-only pattern as DNS records (CNAME entries in Pi-hole are stored as `"domain,target"` strings with no comment field, same limitation as local DNS)
- **Plan/Apply**: Wire CNAME diffing and application into `Plan()` and `Apply()`, placed after local DNS in the reconcile order
- **DiffReport**: Add `CNAME ResourceDiff` field to `DiffReport`
- **PiholeAPI interface**: Add `ListCNAMERecords`, `AddCNAMERecord`, `DeleteCNAMERecord` methods
- **FTL restart warning**: Log a warning when CNAME changes are detected, since Pi-hole restarts FTL on CNAME modifications (brief DNS outage)
- **Opt-in purge**: Add `cname_purge: bool` config option (same pattern as `local_dns_purge` from the dns-marker fix)

## Capabilities

### New Capabilities

- `cname-reconcile`: Declarative CNAME record management with diff, plan, and apply support

### Modified Capabilities

_(none — no existing specs)_

## Impact

- `internal/config/config.go`: new `CNAMEEntry` struct and `CNAME` field on `Config`
- `internal/reconcile/reconciler.go`: new `diffCNAME` function, `PiholeAPI` interface additions, `DiffReport.CNAME` field
- `internal/reconcile/reconciler_test.go`: new test cases for CNAME diffing
- `cmd/forseti/main.go`: FTL restart log warning (informational only)
