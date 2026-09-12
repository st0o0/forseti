## MODIFIED Requirements

### Requirement: Settings field mapping
Each Forseti setting SHALL map to a specific Pi-hole v6 config API path. The supported settings SHALL be:

**DNS (existing)**:
- `dns.upstream` → `dns/upstreams` (list of upstream DNS servers)
- `dns.cache.size` → `dns/cache/size` (cache entry count)
- `dns.cache.force_on_disk` → `dns/cache/optimizer` (force gravity DB to disk)
- `dns.rate_limit.count` → `dns/rateLimit/count` (queries per interval)
- `dns.rate_limit.interval` → `dns/rateLimit/interval` (rate limit window in seconds)

**DNS (extended)**:
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

**Blocking**:
- `blocking.mode` → `dns/blocking/mode` (NULL, IP, NXDOMAIN)
- `blocking.active` → `dns/blocking/active` (blocking enabled, boolean)
- `blocking.timer` → `dns/blocking/timer` (auto-re-enable after N minutes, 0 = permanent, integer)

**DHCP**:
- `dhcp.active` → `dhcp/active` (DHCP server enabled, boolean)
- `dhcp.start` → `dhcp/start` (lease range start IP)
- `dhcp.end` → `dhcp/end` (lease range end IP)
- `dhcp.router` → `dhcp/router` (default gateway IP)
- `dhcp.lease_time` → `dhcp/leaseTime` (lease duration in hours, integer)
- `dhcp.domain` → `dhcp/domain` (DHCP domain name)
- `dhcp.ipv6` → `dhcp/ipv6` (enable IPv6 DHCP/SLAAC, boolean)
- `dhcp.rapid_commit` → `dhcp/rapidCommit` (DHCPv4 rapid commit, boolean)

**Webserver**:
- `webserver.port` → `webserver/port` (web interface port)

**Privacy**:
- `privacy.level` → `misc/privacylevel` (0-4)

**Misc**:
- `misc.nice` → `misc/nice` (process niceness, integer)
- `misc.delay_startup` → `misc/delay_startup` (startup delay in seconds, integer)
- `misc.check.load` → `misc/check/load` (monitor CPU load, boolean)
- `misc.check.disk` → `misc/check/disk` (disk usage threshold percentage, integer 0-100)
- `misc.check.shmem` → `misc/check/shmem` (shared memory threshold percentage, integer 0-100)

Boolean settings SHALL be sent to the Pi-hole API as native boolean values (`true`/`false`). The YAML config format SHALL continue to accept `true`/`false` values. The `boolToInt` coercion SHALL NOT be used.

#### Scenario: Map Forseti boolean setting to API boolean
- **WHEN** the effective settings contain `dns.cache.force_on_disk: true`
- **THEN** the system SHALL send the value `true` (boolean) to Pi-hole config API path `dns/cache/optimizer`

#### Scenario: Map Forseti boolean false to API boolean false
- **WHEN** the effective settings contain `dns.dnssec: false`
- **THEN** the system SHALL send the value `false` (boolean) to Pi-hole config API path `dns/dnssec`

#### Scenario: Non-boolean settings sent as-is
- **WHEN** the effective settings contain `dns.cache.size: 10000`
- **THEN** the system SHALL send the value `10000` (integer) to Pi-hole config API path `dns/cache/size` without type conversion

#### Scenario: Map rev_server block
- **WHEN** the effective settings contain `dns.rev_server.enabled: true`, `dns.rev_server.cidr: "192.168.1.0/24"`, `dns.rev_server.target: "192.168.1.1"`, `dns.rev_server.domain: "lan"`
- **THEN** the system SHALL map these to `dns/revServer/active`, `dns/revServer/cidr`, `dns/revServer/target`, `dns/revServer/domain`

### Requirement: Type-aware settings comparison
The system SHALL compare desired and actual settings values using type-aware normalization. Boolean values SHALL be treated as equivalent to their integer counterparts for comparison purposes. Floating-point numbers without fractional parts SHALL be treated as equivalent to their integer counterparts. String comparisons SHALL be case-insensitive.

#### Scenario: Bool and integer comparison
- **WHEN** the Pi-hole API returns `0` (integer, unmarshalled as `float64`) for `dns/cache/optimizer` and the desired value is `false` (boolean)
- **THEN** the system SHALL consider these values equal and report no drift

#### Scenario: Float64 and integer comparison
- **WHEN** the Pi-hole API returns `10000` (unmarshalled as `float64(10000)`) for `dns/cache/size` and the desired value is `10000` (int)
- **THEN** the system SHALL consider these values equal and report no drift

#### Scenario: Case-insensitive string enum comparison
- **WHEN** the Pi-hole API returns `"LOCAL"` for `dns/listeningMode` and the desired value is `"local"`
- **THEN** the system SHALL consider these values equal and report no drift

#### Scenario: Different string values still detected
- **WHEN** the Pi-hole API returns `"LOCAL"` for `dns/listeningMode` and the desired value is `"all"`
- **THEN** the system SHALL report drift
