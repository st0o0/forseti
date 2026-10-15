## MODIFIED Requirements

### Requirement: Structured log event for domain detail
For each domain detected as newly blocked, the system SHALL emit a structured log event named `new_blocked_domain`, carrying at minimum: the target name, the domain, its first-seen timestamp within the window, and the number of times it was queried within the window (`count_24h`). The system SHALL NOT apply any count threshold to which domains are logged. Each (target, domain) pair SHALL be logged once per detection window: a domain that is still reported as newly blocked on later runs within 24 hours of its first-seen time SHALL NOT be logged again. This de-duplication state SHALL be kept in memory only; after a restart a domain still within its window MAY be logged once more.

#### Scenario: Newly blocked domain is logged
- **WHEN** domain `click.example.com` is newly blocked on target `pizero` with 14 occurrences in the last 24h
- **THEN** the system SHALL emit a `new_blocked_domain` log event with `target=pizero`, `domain=click.example.com`, `count_24h=14`, and a `first_seen` timestamp

#### Scenario: No threshold suppression
- **WHEN** a newly blocked domain was queried only once in the last 24h
- **THEN** the system SHALL still emit a `new_blocked_domain` log event for it (count-based filtering is a downstream/Grafana concern, not Forseti's)

#### Scenario: Not repeated on later runs
- **WHEN** `click.example.com` was logged for `pizero` and the next detection run 15 minutes later still finds it newly blocked
- **THEN** no further `new_blocked_domain` event SHALL be logged for it, while `forseti_new_blocked_domains_24h` SHALL still count it

### Requirement: No Forseti-side persistence of seen domains
The system SHALL derive "newly blocked" status solely from the target's own query history on each run. The system SHALL NOT maintain a ledger, cache file, or database of previously seen domains that survives a restart. The in-memory log de-duplication state SHALL NOT influence which domains are counted.

#### Scenario: Restart does not cause a false "everything is new" burst
- **WHEN** Forseti restarts and performs its first new-blocked-domains check after startup
- **THEN** the system SHALL compute "newly blocked" using only the target's query history within the rolling 24h window, producing the same gauge value as if Forseti had not restarted
