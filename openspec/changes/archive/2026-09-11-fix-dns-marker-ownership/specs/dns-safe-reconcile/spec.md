## ADDED Requirements

### Requirement: DNS reconciliation defaults to additive-only mode

The DNS diff engine SHALL only produce add operations by default. Deletion of DNS records not present in the desired state SHALL NOT occur unless the user explicitly opts in via the `local_dns_purge` configuration option.

#### Scenario: Additive-only mode preserves unmanaged DNS records

- **WHEN** the desired state contains `[192.168.1.1 router.local]` and the actual Pi-hole state contains `[192.168.1.1 router.local, 10.0.0.1 manual.local]` and `local_dns_purge` is false or unset
- **THEN** the diff SHALL produce zero deletions and zero additions, with `manual.local` preserved untouched

#### Scenario: Additive-only mode adds missing DNS records

- **WHEN** the desired state contains `[192.168.1.1 router.local, 192.168.1.2 new.local]` and the actual Pi-hole state contains `[192.168.1.1 router.local]` and `local_dns_purge` is false or unset
- **THEN** the diff SHALL produce one addition (`192.168.1.2 new.local`) and zero deletions

### Requirement: Opt-in purge mode enables full DNS ownership

When `local_dns_purge` is set to `true` in the target or global configuration, the DNS diff engine SHALL delete actual DNS records that are not present in the desired state.

#### Scenario: Purge mode deletes unmanaged DNS records

- **WHEN** the desired state contains `[192.168.1.1 router.local]` and the actual Pi-hole state contains `[192.168.1.1 router.local, 10.0.0.1 stale.local]` and `local_dns_purge` is true
- **THEN** the diff SHALL produce one deletion (`10.0.0.1 stale.local`)

#### Scenario: Purge mode with empty desired state deletes all

- **WHEN** the desired state is empty and the actual Pi-hole state contains DNS records and `local_dns_purge` is true
- **THEN** the diff SHALL produce deletions for all actual records

### Requirement: Config field validation for local_dns_purge

The `local_dns_purge` field SHALL be a boolean that defaults to `false` when omitted.

#### Scenario: Config without local_dns_purge defaults to safe mode

- **WHEN** the YAML config omits the `local_dns_purge` field
- **THEN** the parsed config SHALL have `LocalDNSPurge` set to `false`

### Requirement: Error sentinel for HTTP server shutdown

The metrics server shutdown check SHALL use `errors.Is(err, http.ErrServerClosed)` instead of string comparison.

#### Scenario: Server shutdown is detected via sentinel

- **WHEN** the metrics server returns `http.ErrServerClosed` (including when wrapped)
- **THEN** the error SHALL be recognized as a graceful shutdown and not logged as an error
