## Context

Forseti's collector (`internal/collector/collector.go`) fetches aggregated stats per target (`GetStats`, `GetFTLInfo` via `internal/pihole/client.go`) on a shared TTL cache and exposes them as `GaugeVec`s keyed only by `target` (`internal/metrics/metrics.go`). Every metric follows this shape — none carry a `domain` label, since Prometheus/VictoriaMetrics cardinality grows with the number of distinct label value combinations, and domain counts are unbounded and attacker-influenced (anyone querying a blocked domain creates a new series).

Pi-hole v6 already retains long-term query history (observed: 84k+ entries spanning months) accessible via the REST API's `/api/queries` endpoint, with `from`/`until` as Unix timestamps and a `status` filter. Forseti authenticates against this API per target via a session pool (`internal/session`) and never touches Pi-hole's SQLite database directly.

Forseti already emits structured JSON logs via `slog` with a JSON handler (`cmd/forseti/main.go`), consumed downstream by an existing Alloy → VictoriaLogs → Grafana pipeline for high-cardinality / exploratory data (the established home for this class of information, see `structured-logging` capability).

## Goals / Non-Goals

**Goals:**
- Surface, per target, which domains were blocked for the first time within a rolling 24h window.
- Keep Prometheus cardinality bounded: one aggregated gauge per target, no domain labels.
- Avoid introducing new persistent state in Forseti for "have I seen this domain before."
- Reuse existing infrastructure (session pool, slog JSON handler, Alloy pipeline) rather than building new plumbing.

**Non-Goals:**
- No per-domain Prometheus metrics or labels.
- No alerting/threshold logic inside Forseti (e.g., "only report if count_24h > N") — that belongs in Grafana/LogQL.
- No cross-target merging of "new domain" results — each target is evaluated independently.
- No calendar-day ("since midnight") semantics.

## Decisions

**1. Two-stage query via Pi-hole's own query history, not a Forseti-side ledger.**
A single windowed query (`from=now-24h, until=now`) cannot distinguish "newly blocked" from "still being blocked" — every record it returns is, by construction, inside the window, so filtering by "first seen in window" on that result alone is a no-op. Each collection cycle therefore runs two stages:
1. `GetQueries(status=blocked, from=now-24h, until=now)` to collect the set of domains blocked at all in the last 24h, with their count and earliest timestamp in that window.
2. For each candidate domain, `GetQueries(status=blocked, until=now-24h, domain=<candidate>, length=1)` — an existence check: if this returns zero records, the domain has no blocked history before the window, i.e., it is genuinely new; if it returns any record, the domain was already being blocked before the window and is excluded.

Forseti does not persist "domains seen before" itself — a container restart does not cause a false "everything is new" burst, because the source of truth is Pi-hole's own history, not Forseti's state.
Alternatives considered:
- Forseti maintains its own seen-domains set (in memory or on disk). Rejected — adds persistence/restart-recovery complexity for no benefit, since Pi-hole already has the data.
- Single windowed query only (originally proposed). Rejected during implementation — found to be logically incapable of distinguishing "new" from "still active" (see above).
- Full-history query every cycle (`until=now`, no `from`) instead of two stages. Rejected — would scale with total retained history (potentially months, 84k+ rows) on every cycle, whereas the two-stage approach scales with the number of distinct domains blocked in the last 24h (the existence check per candidate is a cheap, `length=1`-bounded lookup).

**2. Rolling window, not calendar-day.**
`/api/queries` takes Unix `from`/`until` timestamps; there is no server-side "since midnight" concept. A rolling `now-24h` window requires no timezone handling or "last midnight" checkpoint state. Calendar-day semantics would require Forseti to track a reference point across runs — reintroducing the state problem Decision 1 avoids.

**3. Separate collector with its own interval, not folded into the existing stats cycle.**
`/api/queries` over a 24h window is a materially heavier call than `GetStats`/`GetFTLInfo` (summary endpoints). Running it on every scrape (`scrape_interval`, often short) would be wasteful since the new-domain set does not need sub-minute freshness. This follows the precedent set by the gravity scheduler (`internal/gravity`), which already runs on its own cadence independent of the collector loop.
Alternative considered: run inside `collectTarget` on the shared TTL cache. Rejected — couples an expensive, infrequently-needed check to a cycle designed for cheap, frequent polling.

**4. Domain-level detail goes to structured logs; Prometheus gets only a count.**
Emit `slog.Info("new_blocked_domain", "target", ..., "domain", ..., "first_seen", ..., "count_24h", ...)` per newly-seen domain, and set `forseti_new_blocked_domains_24h{target}` to the count of such domains for that cycle. This mirrors how Forseti already separates aggregated metrics (Prometheus) from detailed/exploratory data (logs via Alloy), and sidesteps the cardinality problem entirely rather than working around it.

**5. No threshold filtering in Forseti.**
Every domain newly seen in the window is logged, carrying `count_24h` so consumers can filter in LogQL/Grafana (e.g., "count_24h > 3") without redeploying Forseti to change a threshold. Forseti's job is detection and reporting, not alerting policy.

**6. New `GetQueries` client method, same pattern as existing calls.**
Added to `internal/pihole/client.go` alongside `GetStats`/`GetFTLInfo`, using the existing session-authenticated `doJSON`/`doRequest` path (auto-reauth on 401 included for free). No new HTTP client, no new auth path.

**7. Multiple status values are sent as repeated `status` query params, not comma-joined.**
Verified against a real Pi-hole v6 instance (6.7.1) in the Docker dev stack: `/api/queries?status=A,B` is rejected with `bad_request`, while `/api/queries?status=A&status=B` works as an OR filter. `GetQueries` builds the query string accordingly (`query.Add("status", s)` per value).

## Risks / Trade-offs

- **Pi-hole query history retention is operator-controlled** → if a target's `MAXDBDAYS`/history retention is shorter than 24h, "first seen" could be a false positive (domain existed before but outside retained history) → Mitigation: document this caveat in the capability spec; not fixable from Forseti's side.
- **Heavier `/api/queries` call could still strain a low-power target (e.g., pizero)** at the chosen interval → Mitigation: interval is operator-configurable per the new toggle, defaulting conservatively (e.g., 15 min, not every scrape).
- **Large blocked-domain volume in one window could produce a log burst** → Mitigation: this is inherent to raw reporting (Decision 5); acceptable since VictoriaLogs/Alloy is designed for this, and no threshold logic exists in Forseti to tune.
- **Two-stage query means one existence-check request per distinct candidate domain per target per cycle** → Mitigation: each check is bounded (`length=1`, single domain filter), and the interval is independently configurable (default 15m) precisely so this cost is not incurred on every scrape.
- **Pi-hole's `/api/queries` `domain` filter may match as a pattern rather than requiring an exact match** → Mitigation: not verified against Pi-hole's OpenAPI spec at implementation time; documented as an assumption in `QueryHistoryOptions.Domain`. If it turns out to be pattern-based, false negatives (missing a "prior history" hit due to over-broad matching is not a risk here, since a broader match can only return *more* prior records, making the system more conservative, never incorrectly reporting a domain as new).

## Resolved Questions

- The new-domain collector runs per-target in parallel (mirrors `collectTarget`'s pattern), on its own ticker (`NewDomainsCollector.Start`), separate from the scrape-triggered `Collector.Collect`.
- Config key names: `metrics.new_domains_interval` (duration, default 15m, min 1m) and `metrics.collectors.new_domains` (bool toggle, default true).
