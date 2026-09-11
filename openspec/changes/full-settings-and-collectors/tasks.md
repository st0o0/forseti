## 1. Config — Extended Settings Structs

- [x] 1.1 Add `RevServerSettings` struct (Enabled *bool, CIDR, Target, Domain strings) and embed in `DNSSettings`
- [x] 1.2 Add `DHCPSettings` struct (Active *bool, Start, End, Router strings, LeaseTime *int, Domain string, IPv6 *bool, RapidCommit *bool)
- [x] 1.3 Add `WebserverSettings` struct (Port *int)
- [x] 1.4 Add `CheckSettings` struct (Load *bool, Disk *int, Shmem *int) and `MiscSettings` struct (Nice *int, DelayStartup *int, Check CheckSettings)
- [x] 1.5 Add new pointer fields to `DNSSettings`: Port *int, DomainNeeded *bool, BogusPriv *bool, DNSSEC *bool, ListeningMode string, QueryLogging *bool, CNAMEDeepInspect *bool, ResolveIPv4 *bool, ResolveIPv6 *bool
- [x] 1.6 Add new pointer fields to `BlockingSettings`: Active *bool, Timer *int
- [x] 1.7 Embed DHCPSettings, WebserverSettings, MiscSettings in `Settings` struct
- [x] 1.8 Update `Settings.IsEmpty()` to check all new fields

## 2. Config — Collector Toggles

- [x] 2.1 Add `CollectorToggles` struct with *bool fields: Stats, Upstreams, QueryTypes, Blocking, Reconcile, Gravity, Sessions, SettingsDrift, DHCP
- [x] 2.2 Embed `CollectorToggles` in `Metrics` struct as `Collectors`
- [x] 2.3 Add `CollectorToggles.IsEnabled(name string) bool` helper that returns true when pointer is nil or true
- [x] 2.4 Add defaults in `applyDefaults()` — no action needed since nil = enabled

## 3. Config — Validation

- [x] 3.1 Add validation for new DNS fields: port (1-65535), listening_mode (local/all/bind)
- [x] 3.2 Add validation for blocking: timer must be non-negative
- [x] 3.3 Add validation for DHCP: start+end+router must all be present if any is set, IPs must be valid
- [x] 3.4 Add validation for webserver: port (1-65535)
- [x] 3.5 Add validation for misc: disk (0-100), shmem (0-100), delay_startup non-negative
- [x] 3.6 Update `validateSettings()` to call all new validators
- [x] 3.7 Add tests for all new validation rules

## 4. Settings Reconciler — Extended Mappings

- [x] 4.1 Add new DNS field mappings to `buildDesiredSettings()` (port, domain_needed, bogus_priv, dnssec, listening_mode, query_logging, cname_deep_inspect, resolve_ipv4, resolve_ipv6)
- [x] 4.2 Add rev_server field mappings (4 fields)
- [x] 4.3 Add blocking field mappings (active, timer)
- [x] 4.4 Add DHCP field mappings (8 fields)
- [x] 4.5 Add webserver field mapping (port)
- [x] 4.6 Add misc field mappings (nice, delay_startup, check.load, check.disk, check.shmem)
- [x] 4.7 Add tests for all new mappings in settings_test.go

## 5. Pi-hole Client — DHCP API

- [x] 5.1 Add `APIDHCPLease` struct and `GetDHCPLeases()` method calling `GET /api/dhcp/leases`
- [x] 5.2 Add test for `GetDHCPLeases()`

## 6. Metrics — Rename pihole_* to forseti_*

- [x] 6.1 Rename all `pihole_*` metric names to `forseti_*` in `NewServer()` (18 metrics)
- [x] 6.2 Rename `pihole_status` to `forseti_blocking_status` and `pihole_gravity_last_update` to `forseti_gravity_last_update_timestamp`
- [x] 6.3 Update metrics_test.go to match new names
- [x] 6.4 Update any references in collector_test.go or reconciler_test.go

## 7. Metrics — Conditional Registration

- [x] 7.1 Change `NewServer()` to accept `CollectorToggles` parameter
- [x] 7.2 Wrap each metric group registration in a toggle check; store nil when disabled
- [x] 7.3 Add nil guards to all `Record*`/`Update*` methods (UpdateStats, UpdateUpstreams, UpdateBlockingStatus, RecordReconcile, RecordGravityRun, RecordSessionReauth, etc.)
- [x] 7.4 Add tests: disabled toggle → metric absent from Gather() output

## 8. Metrics — Settings Drift Gauge

- [x] 8.1 Add `forseti_settings_drift` GaugeVec with labels {target, setting} — registered only when settings_drift toggle is enabled
- [x] 8.2 Add `UpdateSettingsDrift(target string, drifted map[string]bool)` method on Server
- [x] 8.3 Integrate drift update into reconcile loop: call DiffSettings() and update gauge
- [x] 8.4 Add tests for drift metric

## 9. Metrics — DHCP Lease Gauge

- [x] 9.1 Add `forseti_dhcp_leases_active` GaugeVec with label {target} — registered only when dhcp toggle is enabled
- [x] 9.2 Add `UpdateDHCPLeases(target string, count int)` method on Server
- [x] 9.3 Add tests for DHCP metric

## 10. Collector — Toggle-Aware Collection

- [x] 10.1 Pass `CollectorToggles` to `NewCollector()`
- [x] 10.2 Skip `GetStats()` call when stats toggle is disabled
- [x] 10.3 Skip `GetBlockingStatus()` call when blocking toggle is disabled
- [x] 10.4 Skip `GetUpstreams()` call when upstreams toggle is disabled
- [x] 10.5 Add `GetDHCPLeases()` call when dhcp toggle is enabled
- [x] 10.6 Add tests for conditional collection

## 11. Integration — Wire Everything Together

- [x] 11.1 Update `cmd/forseti/main.go` to pass CollectorToggles to NewServer and NewCollector
- [x] 11.2 Wire settings drift update into reconcile loop in main.go
- [x] 11.3 Run `go test -race ./...` and `golangci-lint run` — fix all issues
