## Context

Pi-hole v6 REST API organizes domains under `/api/domains/{type}/{kind}` where `type` is `deny` or `allow` and `kind` is `exact` or `regex`. The pihole client already supports arbitrary kind values — `ListDomains(domType, kind)` and `CreateDomain(domType, kind, ...)` accept strings. The limitation is entirely in the config model (no `kind` field) and reconciler (hardcoded `"exact"`).

Current flow:
1. Config defines `deny` / `allow` entries with just a `domain` field
2. Reconciler calls `api.ListDomains("deny", "exact")` and diffs against config
3. Apply creates entries via `api.CreateDomain("deny", "exact", ...)`

## Goals / Non-Goals

**Goals:**
- Support `exact` and `regex` domain kinds in deny/allow config entries
- Reconcile each kind independently (regex entries don't interfere with exact entries)
- Validate regex patterns at config load time
- Propagate kind through sync mode

**Non-Goals:**
- Wildcard syntax sugar (e.g., `*.example.com` → regex) — users write regex directly
- Per-kind group assignments (groups are per-entry, kind doesn't affect them)
- CNAME records (separate change)

## Decisions

### 1. Optional `kind` field defaulting to `"exact"`

Add `Kind string \`yaml:"kind"\`` to `DenyEntry` and `AllowEntry`. In `applyDefaults()`, set empty `Kind` to `"exact"`. This is fully backward-compatible — existing configs without `kind` behave identically.

**Why over alternatives**:
- *Separate `deny_regex` list*: Duplicates config structure, harder to maintain
- *Auto-detect via pattern*: Fragile heuristic, explicit is better

### 2. Per-kind reconciliation loop

Instead of one `ListDomains("deny", "exact")` call, the reconciler iterates over the set of kinds present in config. For each kind, it fetches the actual state and diffs against desired entries of that kind.

This keeps exact and regex namespaces separate — an exact domain `ads.example.com` and a regex `(^|\.)ads\.example\.com$` can coexist without conflicts.

### 3. Regex validation replaces hostname validation for regex entries

When `kind` is `regex`, skip `validateDomain` (which checks hostname pattern). Instead, attempt `regexp.Compile(domain)` and report any compilation error. This catches typos early without being overly restrictive on regex syntax.

### 4. Diff key includes kind prefix

To avoid collisions between exact and regex entries with the same string, the diff key becomes `{kind}:{domain}`. This is internal to the diff engine and doesn't affect the API calls.

## Risks / Trade-offs

- **Regex validation is compile-time only** → A valid regex can still match nothing or too much. This is acceptable — forseti validates syntax, not semantics.
- **Additional API calls per kind** → One extra `ListDomains` call per kind per reconcile. Negligible overhead since Pi-hole v6 returns fast on empty lists.
