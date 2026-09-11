## MODIFIED Requirements

### Requirement: Settings field mapping
Each Forseti setting SHALL map to a specific Pi-hole v6 config API path. The supported settings SHALL be:

**DNS (existing)**:
- `dns.upstream` → `dns/upstreams` (list of upstream DNS servers)
- `dns.cache.size` → `dns/cache/size` (cache entry count)
- `dns.cache.force_on_disk` → `dns/cache/optimizer` (force gravity DB to disk)
- `dns.rate_limit.count` → `dns/rateLimit/count` (queries per interval)
- `dns.rate_limit.interval` → `dns/rateLimit/interval` (rate limit window in seconds)

**DNS (new)**:
- `dns.port` → `dns/port` (DNS listening port, integer)
- `dns.domain_needed` → `dns/domainNeeded` (never forward non-FQDNs, boolean)
- `dns.bogus_priv` → `dns/bogusPriv` (never forward reverse lookups for private ranges, boolean)
- `dns.dnssec` → `dns/dnssec` (DNSSEC validation, boolean)
- `dns.listening_mode` → `dns/listeningMode` (local, all, or bind)
- `dns.query_logging` → `dns/queryLogging` (enable query logging, boolean)
- `dns.cname_deep_inspect` → `dns/cnameDeepInspect` (deep CNAME inspection, boolean)
- `dns.resolve_ipv4` → `dns/resolveIPv4` (resolve IPv4 addresses, boolean)
- `dns.resolve_ipv6` → `dns/resolveIPv6` (resolve IPv6 addresses, boolean)
- `dns.rev_server.enabled` → `dns/revServer/active` (conditional forwarding, boolean)
- `dns.rev_server.cidr` → `dns/revServer/cidr` (CIDR range for conditional forwarding)
- `dns.rev_server.target` → `dns/revServer/target` (target DNS server for conditional forwarding)
- `dns.rev_server.domain` → `dns/revServer/domain` (domain for conditional forwarding)

**Blocking (extended)**:
- `blocking.mode` → `dns/blocking/mode` (NULL, IP, NXDOMAIN)
- `blocking.active` → `dns/blocking/active` (blocking enabled, boolean)
- `blocking.timer` → `dns/blocking/timer` (auto-re-enable after N minutes, 0 = permanent, integer)

**DHCP (new)**:
- `dhcp.active` → `dhcp/active` (DHCP server enabled, boolean)
- `dhcp.start` → `dhcp/start` (lease range start IP)
- `dhcp.end` → `dhcp/end` (lease range end IP)
- `dhcp.router` → `dhcp/router` (default gateway IP)
- `dhcp.lease_time` → `dhcp/leaseTime` (lease duration in hours, integer)
- `dhcp.domain` → `dhcp/domain` (DHCP domain name)
- `dhcp.ipv6` → `dhcp/ipv6` (enable IPv6 DHCP/SLAAC, boolean)
- `dhcp.rapid_commit` → `dhcp/rapidCommit` (DHCPv4 rapid commit, boolean)

**Webserver (new)**:
- `webserver.port` → `webserver/port` (web interface port)

**Privacy (existing)**:
- `privacy.level` → `misc/privacylevel` (0-4)

**Misc (new)**:
- `misc.nice` → `misc/nice` (process niceness, integer)
- `misc.delay_startup` → `misc/delay_startup` (startup delay in seconds, integer)
- `misc.check.load` → `misc/check/load` (monitor CPU load, boolean)
- `misc.check.disk` → `misc/check/disk` (disk usage threshold percentage, integer 0-100)
- `misc.check.shmem` → `misc/check/shmem` (shared memory threshold percentage, integer 0-100)

#### Scenario: Map new DNS setting to API path
- **WHEN** the effective settings contain `dns.dnssec: true`
- **THEN** the system SHALL map this to Pi-hole config API path `dns/dnssec` with value `true`

#### Scenario: Map DHCP settings to API path
- **WHEN** the effective settings contain `dhcp.active: true` and `dhcp.start: "192.168.1.100"`
- **THEN** the system SHALL map these to `dhcp/active` and `dhcp/start` respectively

#### Scenario: Map rev_server block
- **WHEN** the effective settings contain `dns.rev_server.enabled: true`, `dns.rev_server.cidr: "192.168.1.0/24"`, `dns.rev_server.target: "192.168.1.1"`, `dns.rev_server.domain: "lan"`
- **THEN** the system SHALL map these to `dns/revServer/active`, `dns/revServer/cidr`, `dns/revServer/target`, `dns/revServer/domain`

### Requirement: Settings validation
The system SHALL validate settings values at config load time. Invalid values (wrong type, out of range) SHALL produce validation errors before any API calls.

#### Scenario: Invalid blocking mode
- **WHEN** settings contain `blocking.mode: INVALID`
- **THEN** the system SHALL return a validation error listing valid modes (NULL, IP, NXDOMAIN)

#### Scenario: Negative cache size
- **WHEN** settings contain `dns.cache.size: -1`
- **THEN** the system SHALL return a validation error

#### Scenario: Invalid listening mode
- **WHEN** settings contain `dns.listening_mode: "invalid"`
- **THEN** the system SHALL return a validation error listing valid modes (local, all, bind)

#### Scenario: DHCP range validation
- **WHEN** settings contain `dhcp.start` but not `dhcp.end`
- **THEN** the system SHALL return a validation error requiring both start and end when either is set

#### Scenario: Invalid disk threshold
- **WHEN** settings contain `misc.check.disk: 120`
- **THEN** the system SHALL return a validation error (must be 0-100)

#### Scenario: Negative DNS port
- **WHEN** settings contain `dns.port: -1`
- **THEN** the system SHALL return a validation error (must be 1-65535)

### Requirement: Settings IsEmpty check
The `IsEmpty()` method SHALL return true only when no settings field across any area is set. This determines whether settings reconciliation is skipped.

#### Scenario: Only new fields set
- **WHEN** settings contain only `dhcp.active: true` and no other fields
- **THEN** `IsEmpty()` SHALL return false
