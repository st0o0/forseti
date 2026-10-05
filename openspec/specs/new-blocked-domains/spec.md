# New Blocked Domains

Per-target detection of domains newly blocked within a rolling 24-hour window, surfaced via structured log events and an aggregated Prometheus gauge.

### Requirement: New blocked domain detection per target
The system SHALL, on its own collection interval, query each target's blocked DNS history for a rolling 24-hour window (`now-24h` to `now`) and identify domains whose earliest occurrence in that window falls within the window itself (i.e., first seen within the last 24 hours). Detection SHALL run independently per target; results from different targets SHALL NOT be merged.

#### Scenario: Domain blocked for the first time
- **WHEN** a target's query history shows a blocked domain whose earliest timestamp in the last 24h is within that window
- **THEN** the system SHALL treat it as newly blocked for that target

#### Scenario: Domain blocked previously, still blocked now
- **WHEN** a target's query history contains a blocked domain that also appears outside the 24h window (i.e., it was already blocked earlier)
- **THEN** the system SHALL NOT treat it as newly blocked

#### Scenario: Multiple targets see different new domains
- **WHEN** target `mikrotik` and target `pizero` each have their own set of domains newly blocked in the last 24h
- **THEN** the system SHALL report each target's set independently, without combining them into a single cross-target result

### Requirement: Structured log event for domain detail
For each domain detected as newly blocked, the system SHALL emit a structured log event (via the existing JSON log handler) named `new_blocked_domain`, carrying at minimum: the target name, the domain, its first-seen timestamp within the window, and the number of times it was queried within the window (`count_24h`). The system SHALL NOT apply any threshold or filtering to which domains are logged — every newly-seen domain within the window SHALL be reported.

#### Scenario: Newly blocked domain is logged
- **WHEN** domain `click.example.com` is newly blocked on target `pizero` with 14 occurrences in the last 24h
- **THEN** the system SHALL emit a `new_blocked_domain` log event with `target=pizero`, `domain=click.example.com`, `count_24h=14`, and a `first_seen` timestamp

#### Scenario: No threshold suppression
- **WHEN** a newly blocked domain was queried only once in the last 24h
- **THEN** the system SHALL still emit a `new_blocked_domain` log event for it (count-based filtering is a downstream/Grafana concern, not Forseti's)

### Requirement: Aggregated Prometheus metric without domain label
The system SHALL expose a Prometheus gauge representing the count of newly blocked domains in the rolling 24h window, labeled only by `target`. The metric SHALL NOT carry a `domain` label or any other per-domain label.

#### Scenario: Gauge reflects count of new domains
- **WHEN** target `pizero` has 3 domains newly blocked in the last 24h
- **THEN** `forseti_new_blocked_domains_24h{target="pizero"}` SHALL report `3`

#### Scenario: No per-domain series created
- **WHEN** any number of distinct domains are newly blocked across any number of targets
- **THEN** the `/metrics` endpoint SHALL NOT contain a `domain` label on any series related to this feature

### Requirement: Independent collection interval
Detection of newly blocked domains SHALL run on its own configurable interval, separate from the `metrics.scrape_interval` used by other collectors (e.g., stats, upstreams). This SHALL avoid issuing the underlying query-history API call on every Prometheus scrape.

#### Scenario: Interval independent of scrape_interval
- **WHEN** `metrics.scrape_interval` is set to 15 seconds
- **THEN** the new-blocked-domains check SHALL NOT be re-executed on every 15-second scrape, but only according to its own configured interval

### Requirement: No Forseti-side persistence of seen domains
The system SHALL derive "newly blocked" status solely from the target's own query history on each run. The system SHALL NOT maintain a separate ledger, cache file, or database of previously seen domains across runs.

#### Scenario: Restart does not cause a false "everything is new" burst
- **WHEN** Forseti restarts and performs its first new-blocked-domains check after startup
- **THEN** the system SHALL compute "newly blocked" using only the target's query history within the rolling 24h window, producing the same result as if Forseti had not restarted
