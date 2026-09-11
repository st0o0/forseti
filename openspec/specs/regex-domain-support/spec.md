## ADDED Requirements

### Requirement: Domain entries support a kind field

`DenyEntry` and `AllowEntry` SHALL accept an optional `kind` field with allowed values `exact` and `regex`. When omitted, `kind` SHALL default to `"exact"`.

#### Scenario: Config with explicit regex kind

- **WHEN** a deny entry specifies `kind: regex` and `domain: "(^|\\.)ads\\.example\\.com$"`
- **THEN** the parsed config entry SHALL have `Kind` set to `"regex"` and the domain value preserved verbatim

#### Scenario: Config without kind defaults to exact

- **WHEN** a deny entry specifies only `domain: "ads.example.com"` with no `kind` field
- **THEN** the parsed config entry SHALL have `Kind` set to `"exact"`

#### Scenario: Invalid kind value rejected

- **WHEN** a deny entry specifies `kind: wildcard`
- **THEN** `Load()` SHALL return a validation error indicating the invalid kind

### Requirement: Regex domains are validated by compilation

When `kind` is `regex`, the config loader SHALL validate the domain value by attempting to compile it as a Go regular expression. Hostname pattern validation SHALL be skipped for regex entries.

#### Scenario: Valid regex passes validation

- **WHEN** a deny entry has `kind: regex` and `domain: "(^|\\.)tracking\\."` 
- **THEN** `Load()` SHALL succeed without validation errors

#### Scenario: Invalid regex rejected at load time

- **WHEN** a deny entry has `kind: regex` and `domain: "(unclosed"`
- **THEN** `Load()` SHALL return a validation error indicating the regex is invalid

#### Scenario: Exact domain still validated as hostname

- **WHEN** a deny entry has `kind: exact` and `domain: "not a valid domain!!!"`
- **THEN** `Load()` SHALL return a validation error from the hostname pattern check

### Requirement: Per-kind domain reconciliation

The reconciler SHALL fetch and diff domains independently per kind. Exact and regex entries in the same deny/allow list SHALL NOT interfere with each other.

#### Scenario: Exact and regex entries reconciled independently

- **WHEN** desired state contains exact `ads.example.com` and regex `(^|\\.)tracking\\.` in deny, and Pi-hole has no deny entries
- **THEN** the diff SHALL produce two additions: one exact, one regex — each created via the correct Pi-hole API kind endpoint

#### Scenario: Regex entry not deleted when missing from exact diff

- **WHEN** desired state has only exact entries, and Pi-hole has both exact and regex deny entries
- **THEN** the reconciler SHALL NOT delete the regex entries (they are in a separate kind namespace)

### Requirement: Domain sync propagates kind

In sync mode, the syncer SHALL read domains from the primary per-kind and replicate them to replicas preserving the correct kind for each entry.

#### Scenario: Regex domains synced to replica

- **WHEN** the primary has regex deny entries and the replica has none
- **THEN** the syncer SHALL create the regex entries on the replica using `kind: regex`
