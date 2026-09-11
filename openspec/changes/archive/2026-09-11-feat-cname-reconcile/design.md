## Context

Pi-hole v6 stores CNAME records as flat `"domain,target"` strings under `/api/config/dns/cnameRecords` — no comment field, no IDs. The client parses them into `APICNAMERecord{Domain, Target}`. Adding or removing a CNAME triggers an FTL restart, causing a brief DNS outage.

The reconciler currently handles groups, adlists, deny, allow, local DNS, and clients. CNAME fits naturally after local DNS in the reconcile order.

## Goals / Non-Goals

**Goals:**
- Declarative CNAME management via YAML config
- Additive-only diffing by default (same pattern as local DNS, since no comment/marker is possible)
- Opt-in purge mode for full CNAME ownership
- Visible warning when CNAME changes will trigger FTL restart

**Non-Goals:**
- Batching CNAME operations to minimize restarts (Pi-hole restarts FTL per operation; no batch API exists)
- CNAME validation against existing A/AAAA records (Pi-hole handles resolution)

## Decisions

### 1. Config model: `CNAMEEntry{Domain, Target}`

New struct and `CNAME []CNAMEEntry` field on `Config`, with `yaml:"cname"`. Follows the `LocalDNSEntry` pattern. Key is `"domain,target"` (matching the Pi-hole storage format).

### 2. Additive-only by default with `cname_purge` opt-in

Same design rationale as `local_dns_purge` — no comment field means no marker-based ownership. Default is safe (add-only), `cname_purge: true` enables deletion of unmanaged records.

### 3. Reconcile order: after local DNS, before clients

CNAME records reference domain names, not groups. Placing them after local DNS and before clients keeps DNS-related resources together.

### 4. FTL restart warning in plan/apply output

When `DiffReport.CNAME.HasChanges()` is true, log `"WARNING: CNAME changes will trigger FTL restart (brief DNS outage)"`. This is informational — forseti does not suppress the operation.

### 5. `diffCNAME` follows `diffDNS` pattern

`diffCNAME(desired []config.CNAMEEntry, actual []pihole.APICNAMERecord, purge bool) ResourceDiff`. Key is `domain + "," + target`. When `purge` is false, `Deletes` is always empty.

## Risks / Trade-offs

- **Multiple CNAME changes = multiple FTL restarts** → No mitigation possible; Pi-hole has no batch CNAME API. Users with many CNAME changes should be aware of cumulative DNS downtime.
- **No marker ownership** → Same as local DNS. Users who mix manual and forseti CNAME records should not enable purge mode.
