## Context

Forseti reconciles six resource types against Pi-hole: groups, adlists, deny domains, allow domains, local DNS, and clients. Five of these use the Pi-hole comment field with a `[forseti]` / `[forseti-sync]` marker to distinguish managed entries from manual ones. Local DNS records are the exception — the Pi-hole v6 API stores them as flat `"IP Domain"` strings under `/api/config/dns/hosts` with no metadata or comment support.

Currently `diffDNS` treats all actual DNS records as managed, deleting anything not in the desired state. This can destroy manually created DNS entries.

Separately, `main.go:165` checks for server shutdown with a string comparison instead of `errors.Is`.

## Goals / Non-Goals

**Goals:**
- Make DNS reconciliation safe by default — never delete entries forseti didn't create
- Provide an opt-in purge mode for users who want forseti to fully own DNS records
- Fix the fragile error string comparison

**Non-Goals:**
- Implementing a local state file to track forseti-created DNS entries (adds operational complexity)
- Modifying the Pi-hole API or requesting comment support upstream
- Changing how other resource types (groups, adlists, etc.) handle markers

## Decisions

### 1. Additive-only as default, opt-in purge via `local_dns_purge`

**Choice**: `diffDNS` defaults to add-only mode (no deletions). A new boolean config field `local_dns_purge` enables the current delete-everything-not-desired behavior.

**Why over alternatives**:
- *Local state file*: Adds filesystem dependency (problematic in scratch containers), state drift risk, and backup/migration burden. Over-engineered for this use case.
- *Naming convention* (e.g. `.forseti` suffix): Changes actual DNS resolution behavior, confusing and fragile.
- *Always delete*: Unsafe default, violates the managed-marker invariant's spirit.

Additive-only is the simplest safe default. Users who want full DNS ownership add one line to their config.

### 2. Pass `purge` flag through `diffDNS` signature

**Choice**: Change `diffDNS(desired, actual)` → `diffDNS(desired, actual, purge bool)`. When `purge` is false, `Deletes` is always empty.

**Why**: Keeps the diff logic self-contained. The reconciler already passes `marker` to `diffClients` and `diffGroups` — this follows the same pattern of parameterizing diff behavior.

### 3. `errors.Is` for shutdown check

**Choice**: Replace `err.Error() != "http: Server closed"` with `!errors.Is(err, http.ErrServerClosed)`.

**Why**: Standard Go error handling. The string could change across Go versions; the sentinel is stable.

## Risks / Trade-offs

- **Breaking change for DNS-delete users** → Mitigated by documenting `local_dns_purge: true` in release notes. Users who relied on deletion must add this flag.
- **Stale DNS entries accumulate in add-only mode** → Expected trade-off. Users can manually clean up or enable purge mode. Safety is more important than automatic cleanup.
- **Plan output changes** → `plan` command will no longer show DNS deletions by default. This is the desired behavior change.
