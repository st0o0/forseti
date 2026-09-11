## Context

Forseti applies the same gravity.db content to all Pi-hole targets and has no control over Pi-hole FTL settings (`pihole.toml`). Operators who need per-instance differences (e.g., `force_on_disk` on a low-RAM Pi, stricter blocking for a kids network) must manage FTL settings outside Forseti and cannot vary content per target at all.

Pi-hole v6 exposes the full `pihole.toml` configuration tree via its REST API at `/api/config`. Forseti already uses this endpoint for local DNS and CNAME records. The infrastructure for reading and writing config values is partially in place.

## Goals / Non-Goals

**Goals:**
- Per-target configuration overrides via separate YAML files linked with `file:` on each target
- A curated `settings:` section in Forseti's own format that maps to Pi-hole v6 config API paths
- Merge engine: global defaults + target appends − target excludes = effective config per target
- Settings reconciliation: read actual Pi-hole config, diff against desired, apply changes
- Backwards compatibility: existing configs without `settings:` or `file:` work unchanged

**Non-Goals:**
- Full 1:1 mapping of all pihole.toml keys — only a curated, operationally relevant subset
- Cascading multi-layer overrides (only two layers: global + target)
- Managing pihole.toml keys that require FTL restart (beyond CNAME which already does this)
- Config file generation or templating features (Helm-style)

## Decisions

### 1. Two-layer merge model (global + target)

The effective config for a target is computed by merging the global config with an optional target override file. No intermediate layers, no inheritance between targets.

**Why not deeper layering:** Two layers cover all identified use cases (shared baseline + per-target exceptions). Deeper layering adds merge-order complexity with no demonstrated benefit. Can be revisited if needed.

### 2. Target override file linked via `file:` field

Each target entry can optionally declare `file: path/to/overrides.yaml`. The path is relative to the main config file's directory. The override file has the same top-level structure as the global config for content sections (groups, adlists, deny, allow, local_dns, cname, clients) and settings, plus an `exclude:` block.

**Why explicit `file:` over convention-based:** Explicit linking is visible in the main config — you can see at a glance which targets have overrides. Convention-based (e.g., `targets/<name>.yaml`) hides this relationship and introduces magic path resolution.

### 3. Merge semantics: append for lists, deep merge for settings, exclude for removals

- **Settings (maps/scalars):** Deep merge. Target values overwrite global values at the leaf level. Unspecified keys inherit from global.
- **Content lists (groups, adlists, deny, allow, ...):** Append. Target entries are added to the global list. No implicit replacement.
- **Exclude block:** Removes specific entries from the effective list by matching key (domain for deny/allow, url for adlists, match for clients, name for groups, domain for local_dns/cname).

**Why not replace semantics:** Replace would require targets to re-declare the entire global list minus one entry. Append + exclude is more ergonomic for the common case (shared baseline with small per-target additions/removals).

### 4. Curated settings format with explicit mapping

Forseti defines its own settings struct, not a passthrough to pihole.toml. Each Forseti setting has a known mapping to a Pi-hole config API path. Initial scope:

| Forseti path | Pi-hole API path | Type |
|---|---|---|
| `dns.upstream` | `dns/upstreams` | `[]string` |
| `dns.cache.size` | `dns/cache/size` | `int` |
| `dns.cache.force_on_disk` | `dns/cache/optimizer` | `bool` |
| `dns.rate_limit.count` | `dns/rateLimit/count` | `int` |
| `dns.rate_limit.interval` | `dns/rateLimit/interval` | `int` |
| `blocking.mode` | `dns/blocking/mode` | `string` |
| `privacy.level` | `misc/privacylevel` | `int` |

**Why not passthrough:** A curated format lets Forseti validate values, provide clear error messages, and evolve independently of pihole.toml changes. Passthrough would tie Forseti's config format to Pi-hole's internal structure and prevent validation.

### 5. Settings reconciliation before content

The reconcile order becomes: **settings → groups → adlists → deny → allow → local_dns → cname → clients**. Settings are reconciled first because some settings (like `force_on_disk`) affect how the Pi-hole processes subsequent content changes.

Settings reconciliation uses `GET /api/config` to read current values and `PATCH /api/config` (or per-key `PUT /api/config/<path>/<value>`) to apply changes. Only changed values are written.

### 6. Effective config computed at load time

The merge engine runs during `config.Load()`. After loading, each target carries its fully resolved effective config. Downstream code (reconciler, plan, apply) does not need to know about overrides — it receives a flat, resolved config per target.

**Implementation:** `config.Load()` parses the main file, then for each target with a `file:` field, parses the override file and merges it. The result is a `ResolvedTarget` that contains the target's connection info plus its effective content lists and settings.

### 7. Validation runs on effective (merged) config

Validation happens after merge, on the effective config per target. This catches conflicts like an exclude referencing a domain that doesn't exist in the effective list (warning, not error) and duplicate entries introduced by the merge.

## Risks / Trade-offs

- **Merge surprises** → Clear documentation of merge rules; `forseti plan` shows the effective config per target so operators can verify before applying.
- **Settings API compatibility across Pi-hole versions** → Initial implementation targets Pi-hole v6 only. Settings struct is versioned internally; mapping table can be extended for future Pi-hole releases.
- **Exclude matching edge cases** (e.g., same domain in deny with different kinds) → Exclude matches on the natural key of each resource type (domain+kind for deny/allow, url for adlists, etc.), not just one field.
- **Config file path resolution** → `file:` paths are relative to the directory containing the main config file. Absolute paths are also accepted. Symlinks are followed.

## Open Questions

- Should `forseti plan` output show the raw global+override or only the effective merged result? Leaning toward effective-only with a `--verbose` flag for showing the merge source.
- Should settings reconciliation support a dry-run diff in plan mode? (Likely yes, for consistency with content plan.)
