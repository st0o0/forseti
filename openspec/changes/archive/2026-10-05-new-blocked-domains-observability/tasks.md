## 1. Config

- [x] 1.1 Add `new_domains` bool toggle to `CollectorToggles` in `internal/config/config.go`, defaulting to `true` per existing pattern
- [x] 1.2 Add a `new_domains_interval` (or equivalent) duration field for the independent collection interval, with a sane default (e.g., 15m)
- [x] 1.3 Add validation for the new interval field (reuse existing `Duration` validation patterns, see `config-interval-validation` capability)
- [x] 1.4 Add/extend config tests covering defaulting and override of `new_domains` and its interval

## 2. Pi-hole client

- [x] 2.1 Add `GetQueries` method to `internal/pihole/client.go` calling `GET /api/queries` with `from`/`until` (Unix timestamps) and a blocked-status filter, via the existing `doJSON` session-authenticated path
- [x] 2.2 Define response types for the query history payload (domain, timestamp, status at minimum)
- [x] 2.3 Add client tests for `GetQueries` (request shape, auto-reauth on 401, response parsing) following existing `client_test.go` / `client_reauth_test.go` patterns

## 3. Collector

- [x] 3.1 Add a new collector cycle (separate ticker/interval, not the shared TTL cache used by `collectTarget`) that, per target, fetches candidate blocked domains in the rolling 24h window
- [x] 3.2 Implement first-seen computation as a two-stage query (per design.md Decision 1, corrected from the original single-window approach during implementation): stage 1 collects 24h candidates with count/earliest-timestamp; stage 2 does a per-candidate existence check against history before the window (`until=windowStart, domain=X, length=1`) to confirm it is genuinely new, not just still active
- [x] 3.3 Emit `slog.Info("new_blocked_domain", "target", ..., "domain", ..., "first_seen", ..., "count_24h", ...)` for each detected domain, with no threshold filtering
- [x] 3.4 Respect the `new_domains` toggle: skip the API call and emit nothing when disabled
- [x] 3.5 Add collector tests: new domain detected, domain already seen before window is excluded, per-target independence, toggle disabled skips the call
- [x] 3.6 Wire `NewDomainsCollector` into `cmd/forseti/main.go` (start on its own goroutine like the gravity scheduler; update its targets on config reload)

## 4. Metrics

- [x] 4.1 Add `newBlockedDomains24h` `GaugeVec` keyed by `target` in `internal/metrics/metrics.go`, registered only when `new_domains` toggle is enabled
- [x] 4.2 Add a setter (e.g., `SetNewBlockedDomains24h(target string, count int)`) called from the new collector cycle
- [x] 4.3 Add metrics tests verifying the gauge value, absence when toggle disabled, and absence of any `domain` label on `/metrics` output

## 5. Documentation

- [x] 5.1 Document the `new_domains` toggle, its interval field, and the `new_blocked_domain` log event shape in README/config reference
- [x] 5.2 Note the Pi-hole query-history retention caveat (short `MAXDBDAYS` can cause false "first seen" positives) in the same documentation

## 6. Verification

- [x] 6.1 Run `go build ./...`, `go vet ./...`, `go test ./...` — all pass. `go test -race` could not run in this environment (CGO disabled on this Windows host); `golangci-lint` is not installed in this environment, so run it separately before merging.
- [x] 6.2 Manually verified against the local Docker dev stack (`docker-compose.dev.yml`, two Pi-hole targets): blocked `doubleclick.net`/`googleadservices.com` via DNS lookups, ran the collector at a temporary 1m interval, confirmed `new_blocked_domain` log events per target and `forseti_new_blocked_domains_24h{target="pihole-alpha"} 2` / `{target="pihole-beta"} 1` on `/metrics` with no `domain` label, and confirmed a second cycle re-reports the same domains identically (no Forseti-side ledger, as designed). This also caught and fixed a real bug: Pi-hole v6's `/api/queries` rejects a comma-joined `status` filter — it requires repeated `status=` params (OR semantics) instead (fixed in `GetQueries`).
