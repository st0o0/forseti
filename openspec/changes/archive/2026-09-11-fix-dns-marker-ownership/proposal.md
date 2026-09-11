## Why

Forseti's critical invariant states it only touches entries tagged with its managed marker (`[forseti]` / `[forseti-sync]`). DNS records violate this: `diffDNS` marks ALL actual DNS records not in the desired state for deletion, including manually created entries. Unlike groups, adlists, domains, and clients, the Pi-hole DNS hosts API (`/api/config/dns/hosts`) stores entries as plain `"IP Domain"` strings with no comment field — so the marker pattern cannot be applied directly.

Additionally, `main.go:165` uses a fragile string comparison (`err.Error() != "http: Server closed"`) instead of `errors.Is(err, http.ErrServerClosed)`.

## What Changes

- **DNS reconciliation safety**: Switch DNS diffing from "delete everything not desired" to an additive-only model — forseti adds missing DNS records but never deletes entries it didn't create. Since the API has no comment/marker support, forseti cannot distinguish its own entries from manual ones, so deletion must be opt-in.
- **Opt-in purge mode**: Add a `local_dns_purge: true` config option that restores the current delete-all behavior for users who want forseti to be the sole owner of DNS records.
- **Error sentinel**: Replace string comparison with `errors.Is(err, http.ErrServerClosed)`.

## Capabilities

### New Capabilities

- `dns-safe-reconcile`: Safe DNS reconciliation that defaults to additive-only mode, with opt-in purge for full ownership.

### Modified Capabilities

_(none — no existing specs)_

## Impact

- `internal/reconcile/reconciler.go`: `diffDNS` signature and logic changes
- `internal/reconcile/reconciler_test.go`: new test cases for additive-only vs purge modes
- `internal/config/config.go`: new `local_dns_purge` field
- `cmd/forseti/main.go`: error sentinel fix
- Existing users who rely on DNS deletion must add `local_dns_purge: true` — **BREAKING** for that workflow
