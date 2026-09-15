## MODIFIED Requirements

### Requirement: Marker-based ownership
The reconciler SHALL only manage entries tagged with the configured marker (default `[forseti]`) in the comment field. Entries without the marker are considered manually created and SHALL never be modified or deleted.

When a desired entry matches an existing entry by key (URL for adlists, domain for deny/allow, client IP for clients) but the existing entry's comment does not contain the marker, the reconciler SHALL emit an Update to set the comment to `<marker> <user comment>`. This ensures pre-existing entries (e.g. Pi-hole defaults) receive the marker and become fully managed.

#### Scenario: Three-way diff
- **WHEN** desired state is `[A, B, C]` and actual state is `[A, C, D[forseti], E]`
- **THEN** the reconciler SHALL: skip A (exists, has marker), add B with marker, skip C (exists, has marker), delete D (has marker but not in desired), leave E alone (no marker)

#### Scenario: Manual UI entries preserved
- **WHEN** a user manually adds an adlist through the Pi-hole UI (no marker)
- **THEN** the reconciler SHALL never touch that entry, even if it is not in the YAML config

#### Scenario: Pre-existing entry receives marker
- **WHEN** Pi-hole has a default adlist (StevenBlack) with comment "Migrated from /etc/pihole/adlists.list" and the forseti config includes the same URL with comment "StevenBlack unified hosts"
- **THEN** the reconciler SHALL update the comment to "[forseti] StevenBlack unified hosts" and the entry becomes fully managed

#### Scenario: Marker already present
- **WHEN** an adlist exists with comment "[forseti] EasyPrivacy" and the config defines the same URL
- **THEN** the reconciler SHALL count it as Unchanged (no update needed)
