# Forseti

[![Release](https://img.shields.io/github/v/release/st0o0/forseti?sort=semver&style=flat-square)](https://github.com/st0o0/forseti/releases)
[![GHCR](https://img.shields.io/badge/ghcr.io-st0o0%2Fforseti-2496ED?style=flat-square&logo=docker&logoColor=white)](https://github.com/st0o0/forseti/pkgs/container/forseti)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg?style=flat-square)](LICENSE.md)

Declarative GitOps controller and sync tool for [Pi-hole v6](https://pi-hole.net/).

Two modes: **config** reconciles a YAML file against one or more Pi-hole instances.
**sync** replicates a primary Pi-hole to any number of replicas.
Both include a built-in Prometheus metrics endpoint for Pi-hole stats, reconciliation telemetry, and settings drift detection.

Named after [Forseti](https://en.wikipedia.org/wiki/Forseti), the Norse god of justice -- he reconciles desired state with actual state and ensures every instance is in order.

## Features

- **Declarative config** -- adlists, allow/deny lists, local DNS, CNAME records, groups, clients, and Pi-hole settings defined in one YAML file
- **Sync mode** -- replicate a primary Pi-hole to any number of replicas (NebulaSync replacement)
- **Multi-instance** -- manage any number of Pi-hole v6 targets from one config
- **Per-target API control** -- configurable timeouts and concurrency limits per target, preventing overload on resource-constrained hardware
- **Managed-marker safety** -- only touches entries it created (`[forseti]` / `[forseti-sync]` tag), manual UI changes are left alone
- **Diff-based** -- compares desired vs actual state and applies the minimum set of changes
- **Settings reconciliation** -- manage DNS cache, rate limits, blocking mode, privacy level, and more
- **Gravity scheduling** -- per-target cron schedules and automatic gravity updates on adlist changes
- **Built-in Prometheus metrics** -- `/metrics` endpoint with Pi-hole stats, reconciliation telemetry, settings drift, session health, and collector stale-serve tracking
- **Readable logs** -- a startup summary, one line per run and cycle (also when nothing changed), and health/reachability transitions instead of repeated errors
- **Status API** -- `/status` and `/status/history` show per-target health, last success and recent runs; example Grafana dashboard included
- **Plan/Apply workflow** -- dry-run before committing changes
- **Watch mode** -- daemon with periodic reconcile loop, config hot-reload, and metrics server
- **Environment variable expansion** -- `${VAR}` syntax in YAML values for secrets management

## Quick start

```bash
# One-shot: see what would change
docker run --rm -v ./forseti.yml:/config/forseti.yml:ro \
  ghcr.io/st0o0/forseti:latest plan --config /config/forseti.yml

# One-shot: apply changes
docker run --rm -v ./forseti.yml:/config/forseti.yml:ro \
  ghcr.io/st0o0/forseti:latest apply --config /config/forseti.yml

# Daemon: watch mode with metrics
docker run -d -v ./forseti.yml:/config/forseti.yml:ro \
  -p 9099:9099 \
  ghcr.io/st0o0/forseti:latest watch --config /config/forseti.yml
```

## Commands

| Command | Description |
|---------|-------------|
| `forseti plan --config <path>` | Dry-run: show what would change without applying |
| `forseti apply --config <path>` | Reconcile desired state (one-shot) |
| `forseti watch --config <path>` | Daemon mode with periodic reconcile, metrics server, and gravity scheduler |
| `forseti healthcheck [--config <path> \| --addr <host:port>]` | Liveness probe: `GET /healthz` on the running daemon (3s timeout). Does not contact Pi-hole. |
| `forseti version` | Print version |

## Configuration

### Config mode example

```yaml
mode: config
log_level: info
log_format: text

metrics:
  port: 9099
  path: /metrics
  scrape_interval: 30s

targets:
  - name: pihole-1
    url: http://pihole-host:80
    password: ${PIHOLE_PASSWORD}
    gravity:
      schedule: "0 3 * * 0"
    api:
      timeout: 30s
      max_concurrent: 4

reconcile:
  interval: 5m
  marker: "[forseti]"
  gravity_on_change: true

groups:
  - name: default
  - name: ads

adlists:
  - url: https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts
    groups: [default]

deny:
  - domain: ads.example.com
  - domain: "*.tracking.example.com"
    kind: regex

allow:
  - domain: safe.example.com

local_dns:
  - domain: nas.lan
    ip: 192.168.1.50

cname:
  - domain: app.lan
    target: nas.lan

clients:
  - match: 192.168.1.0/24
    groups: [default]

settings:
  dns:
    upstream: [1.1.1.1, 8.8.8.8]
    cache:
      size: 10000
    rate_limit:
      count: 1000
      interval: 60
    dnssec: false
    query_logging: true
  blocking:
    mode: "NULL"
    active: true
  privacy:
    level: 0
```

### Sync mode example

Replicates entries from a primary Pi-hole to replicas using the `[forseti-sync]` marker.

```yaml
mode: sync

targets:
  - name: primary
    url: http://pihole-primary:80
    password: ${PRIMARY_PASSWORD}
    role: primary
  - name: replica-1
    url: http://pihole-replica:80
    password: ${REPLICA_PASSWORD}
    role: replica

sync:
  interval: 5m
  primary: primary
  resources: [groups, adlists, deny, allow, local_dns, clients]
```

### Full configuration reference

All values support `${VAR}` environment variable expansion.
Fields marked with `*` are pointers -- omit them to keep Pi-hole's existing value.

#### Top-level

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `mode` | string | `config` | Operating mode: `config` or `sync` |
| `log_level` | string | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `log_format` | string | `text` | Log format: `text` or `json` |

#### `metrics`

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `port` | int | `9099` | Prometheus metrics server port |
| `path` | string | `/metrics` | Metrics endpoint path |
| `scrape_interval` | duration | `30s` | Collect-on-scrape cache TTL (min 5s) |
| `new_domains_interval` | duration | `15m` | How often to check for newly blocked domains (min 1m), independent of `scrape_interval` |

#### `metrics.collectors`

All toggles are `*bool`, default enabled (omit = true). Set `false` to disable a metric family.

| Key | Metrics controlled |
|-----|--------------------|
| `stats` | DNS queries, blocked percentage, clients, gravity timestamp |
| `upstreams` | Upstream query counts and response times |
| `query_types` | Queries by type (A, AAAA, ...), status, and reply type |
| `blocking` | Blocking enabled/disabled status |
| `reconcile` | Reconcile runs, duration, changes, drift |
| `gravity` | Gravity runs, duration, timestamps, errors |
| `sessions` | Active sessions, re-auth events |
| `settings_drift` | Per-setting drift detection |
| `dhcp` | Active DHCP lease count |
| `new_domains` | Newly blocked domain count (see below) |

#### New blocked domain detection (`metrics.collectors.new_domains`)

On its own interval (`metrics.new_domains_interval`, default `15m`), Forseti checks each target's query history for domains blocked for the first time in the last 24 hours. This runs independently of `scrape_interval` because it is a heavier query than the summary stats endpoints and does not need scrape-level freshness.

Forseti does not keep its own record of "domains seen before" — each cycle derives this from the target's own Pi-hole query history, so a restart does not produce a false "everything is new" burst. If a target's Pi-hole history retention (`MAXDBDAYS`) is shorter than 24 hours, a domain blocked before that retention window may be misreported as new.

Per-domain detail is emitted as a structured log event (not a Prometheus label, to avoid unbounded metric cardinality):

```json
{"time":"...","level":"INFO","msg":"new_blocked_domain","component":"newdomains","target":"pizero","domain":"click.example.com","first_seen":"2026-10-05T08:11:00Z","count_24h":14}
```

Every newly detected domain is logged once per 24h window — there is no built-in count threshold. Later detection runs that still see the domain as new count it in the gauge but do not log it again (this de-duplication is in memory, so a restart may log a domain once more). Filter or alert on `count_24h` downstream (e.g., in Grafana/LogQL) if you want to ignore one-off queries.

The aggregated count per target is exposed as `forseti_new_blocked_domains_24h{target="..."}`, with no `domain` label.

#### `targets[]`

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `name` | string | -- | **required** Unique target identifier |
| `url` | string | -- | **required** Pi-hole base URL (http/https) |
| `password` | string | -- | **required** Pi-hole API password |
| `role` | string | -- | Sync mode only: `primary` or `replica` |
| `gravity.schedule` | string | -- | 5-field cron expression for periodic gravity updates |
| `api.timeout` | duration | `30s` | HTTP timeout for API requests to this target |
| `api.max_concurrent` | int | `4` | Max concurrent API calls to this target (all subsystems combined) |
| `file` | string | -- | Path to per-target override YAML (see below) |

#### `reconcile` (config mode)

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `interval` | duration | -- | **required** Reconcile loop interval (min 10s) |
| `marker` | string | `[forseti]` | Comment tag for managed entries |
| `gravity_on_change` | *bool | `true` | Trigger gravity after adlist changes |
| `local_dns_purge` | bool | `false` | Delete DNS entries not in config |
| `cname_purge` | bool | `false` | Delete CNAME records not in config |

#### `sync` (sync mode)

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `interval` | duration | `5m` | Sync loop interval (min 10s) |
| `primary` | string | -- | **required** Name of the primary target |
| `resources[]` | []string | -- | **required** Resources to sync: `groups`, `adlists`, `deny`, `allow`, `local_dns`, `clients` |
| `local_dns_purge` | bool | `true` | Delete replica local DNS records that do not exist on the primary. Set `false` to only add records. |

Pi-hole local DNS records have no comment field, so they cannot carry the `[forseti-sync]` marker. By default sync mirrors local DNS from the primary, which deletes replica-only records. Config mode defaults to `local_dns_purge: false` instead, because there the YAML co-exists with records managed in the UI.

#### `groups[]`

| Key | Type | Description |
|-----|------|-------------|
| `name` | string | **required** Group name (unique). `default` maps to Pi-hole's built-in Default group. |
| `comment` | string | Optional description |

#### `adlists[]`

| Key | Type | Description |
|-----|------|-------------|
| `url` | string | **required** Blocklist URL (unique) |
| `comment` | string | Optional description |
| `groups[]` | []string | Group names this adlist belongs to |

#### `deny[]` / `allow[]`

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `domain` | string | -- | **required** Domain name or regex pattern |
| `kind` | string | `exact` | Match type: `exact` or `regex` |

#### `local_dns[]`

| Key | Type | Description |
|-----|------|-------------|
| `domain` | string | **required** Local domain name |
| `ip` | string | **required** IP address (v4 or v6) |

#### `cname[]`

| Key | Type | Description |
|-----|------|-------------|
| `domain` | string | **required** Source domain |
| `target` | string | **required** CNAME target domain. Adding/removing CNAMEs triggers FTL restart. |

#### `clients[]`

| Key | Type | Description |
|-----|------|-------------|
| `match` | string | **required** Client IP address or CIDR subnet |
| `comment` | string | Optional description |
| `groups[]` | []string | Group names this client belongs to |

#### `settings`

All settings fields are optional pointers. Omit a field to leave Pi-hole's current value unchanged.

**`settings.dns`**

| Key | Type | Description |
|-----|------|-------------|
| `upstream[]` | []string | Upstream DNS servers |
| `cache.size` | *int | DNS cache size (entries) |
| `cache.force_on_disk` | *bool | Force cache database to disk |
| `rate_limit.count` | *int | Max queries per interval |
| `rate_limit.interval` | *int | Rate limit window (seconds) |
| `port` | *int | DNS listening port (1-65535) |
| `domain_needed` | *bool | Never forward non-FQDN queries |
| `bogus_priv` | *bool | Never forward reverse lookups for private ranges |
| `dnssec` | *bool | Enable DNSSEC validation |
| `listening_mode` | string | `local`, `all`, or `bind` |
| `query_logging` | *bool | Enable query logging |
| `cname_deep_inspect` | *bool | Deep CNAME inspection |
| `resolve_ipv4` | *bool | Resolve IPv4 addresses for hostnames |
| `resolve_ipv6` | *bool | Resolve IPv6 addresses for hostnames |
| `rev_server.enabled` | *bool | Enable conditional forwarding |
| `rev_server.cidr` | string | Local network CIDR |
| `rev_server.target` | string | Target DNS server for conditional forwarding |
| `rev_server.domain` | string | Local domain name |

**`settings.blocking`**

| Key | Type | Description |
|-----|------|-------------|
| `mode` | string | Blocking mode: `NULL`, `IP`, or `NXDOMAIN` |
| `active` | *bool | Enable/disable blocking |
| `timer` | *int | Auto-disable timer (seconds, 0 = permanent) |

**`settings.privacy`**

| Key | Type | Description |
|-----|------|-------------|
| `level` | *int | Privacy level 0-4 (0 = show everything, 4 = hide all) |

**`settings.dhcp`**

| Key | Type | Description |
|-----|------|-------------|
| `active` | *bool | Enable DHCP server |
| `start` | string | DHCP range start IP |
| `end` | string | DHCP range end IP |
| `router` | string | Default gateway IP |
| `lease_time` | *int | Lease time (seconds) |
| `domain` | string | DHCP domain |
| `ipv6` | *bool | Enable IPv6 DHCP (SLAAC + RA) |
| `rapid_commit` | *bool | Enable DHCPv4 rapid commit |

**`settings.webserver`**

| Key | Type | Description |
|-----|------|-------------|
| `port` | *int | Web interface port (1-65535) |

**`settings.misc`**

| Key | Type | Description |
|-----|------|-------------|
| `nice` | *int | Process niceness |
| `delay_startup` | *int | Startup delay (seconds) |
| `check.load` | *bool | Enable load average check |
| `check.disk` | *int | Disk usage warning threshold (0-100%) |
| `check.shmem` | *int | Shared memory warning threshold (0-100%) |

#### Per-target overrides

A target can reference an override file via `targets[].file`. The override YAML can add or replace `settings`, `groups`, `adlists`, `deny`, `allow`, `local_dns`, `cname`, and `clients` for that specific target.

It can also exclude entries from the base config:

```yaml
# pihole-2-overrides.yml
exclude:
  adlists:
    - url: https://example.com/list-not-for-this-target.txt
  deny:
    - domain: keep-on-other-targets.example.com

adlists:
  - url: https://example.com/extra-list-for-this-target.txt
    groups: [default]
```

## Logging

Forseti logs one line per event to stdout (`plan` and `apply` log to stderr so their output can be piped). Every line has a `component` (`main`, `config`, `http`, `session`, `worker`, `sync`, `reconcile`, `pihole`, `collector`, `newdomains`, `gravity`) and, where applicable, a `target`. `log_level` changes take effect on config reload; `log_format` changes need a restart. Set `TZ` (e.g. `TZ=Europe/Berlin`) to get local timestamps and gravity schedules.

**Levels**: `error` = something failed and needs attention, `warn` = degraded or self-healing, `info` = lifecycle and changes, `debug` = per-request, per-item and retry detail.

**Lifecycle events** (stable message strings, safe to alert on):

| Message | Level | Key attributes |
|---------|-------|----------------|
| `forseti starting` | info | `version`, `mode`, `config`, `tz` |
| `config loaded` | info | `mode`, `targets`, `interval`; sync: `primary`, `replicas`, `resources`; config: resource counts |
| `config warning` | warn | `target`, `reason` |
| `http server listening` | info | `addr`, `endpoints` |
| `target connected` / `target unreachable` | info / warn | `target`, `source`, `error` — logged on change only |
| `run finished` | debug (no changes) / info (changes) / warn (partial) / error (failed) | `target`, `role`, `status`, `stage`, `added`, `updated`, `deleted`, `failed`, `<resource>.<action>`, `errors` |
| `cycle complete` | info | `mode`, `targets`, `ok`, `partial`, `failed`, `skipped`, `changes`, `next_run` — every cycle, also when nothing changed |
| `target degraded` / `target down` / `target recovered` | warn / error / info | `target`, `consecutive_failures`, `retry_at`, `failures`, `downtime` |
| `config reloaded` / `config reload failed` | info / error | `changed`, `targets_added`, `targets_removed` / `error` |
| `gravity scheduled` / `gravity started` / `gravity finished` | info (finished with error: error) | `target`, `schedule`, `next_run`, `reason`, `status`, `duration` |
| `setting applied` | info | `target`, `setting`, `from`, `to` (secrets shown as `[REDACTED]`) |
| `shutting down` / `stopped` | info | `uptime` |

A target becomes `degraded` on its first failed run and `down` after 5 consecutive failures; failing targets back off (`min(failures × interval, 30m)`). In sync mode, if the primary cannot be read, replica runs in that cycle are `skipped`.

Example (sync mode, replica outage and recovery):

```
level=INFO msg="forseti starting" component=main version=0.2.0 mode=sync config=/config/forseti.yml tz=Europe/Berlin
level=INFO msg="config loaded" component=main mode=sync targets=2 interval=5m0s primary=pihole-alpha replicas=[pihole-beta] resources="[groups adlists clients]"
level=INFO msg="target connected" component=session target=pihole-alpha
level=WARN msg="target unreachable" component=session target=pihole-beta source=worker error="... connection refused"
level=ERROR msg="run finished" component=sync target=pihole-beta role=replica status=failed stage=session ... consecutive_failures=1
level=WARN msg="target degraded" component=sync target=pihole-beta consecutive_failures=1 retry_at=...
level=INFO msg="cycle complete" component=sync mode=sync targets=2 ok=1 partial=0 failed=1 skipped=0 changes=0 next_run=...
level=INFO msg="target connected" component=session target=pihole-beta
level=INFO msg="run finished" component=sync target=pihole-beta role=replica status=ok added=14 ... clients.added=12 adlists.added=2
level=INFO msg="target recovered" component=sync target=pihole-beta from=degraded failures=1 downtime=5m0s
```

URL credentials and query values (e.g. `?token=`) are redacted in logs and errors; passwords are never logged.

## Status API

The metrics port also serves a read-only JSON view of recent activity, kept in memory (last 1440 runs per target and 1440 cycles; lost on restart, not an audit log):

- `GET /status` — version, mode, uptime, last config reload, last cycle and next run, and per target: role, health, consecutive failures, last success, backoff, last error and last run with per-resource counts.
- `GET /status/history?target=<name>&since=24h&limit=500` — runs of one target, oldest first. Without `target`: cycle summaries.

```json
{
  "mode": "sync",
  "cycle": { "interval_seconds": 300, "last": { "ok": 1, "failed": 1, "changes": 0 }, "next_run_at": "..." },
  "targets": [
    { "name": "pihole-beta", "role": "replica", "health": "degraded", "consecutive_failures": 1,
      "last_success_at": "...", "backoff_until": "...",
      "last_error": { "at": "...", "message": "... connection refused" },
      "last_run": { "at": "...", "status": "failed", "stage": "session", "duration_ms": 3 } }
  ]
}
```

Responses never include passwords, session IDs or raw API bodies; error messages are redacted.

## Grafana dashboard

[`grafana/forseti.json`](grafana/forseti.json) is an example dashboard (import it and pick your Prometheus data source): time since last successful run, consecutive failures, health and reachability timelines, run outcomes per target over the last 24h, and changes / failed writes per resource type.

A simple "sync is stale" alert:

```promql
time() - forseti_reconcile_last_success_timestamp > 3 * 300   # 3 missed 5m intervals
```

## Health check

The metrics server also serves `GET /healthz`: `200 {"status":"ok"}` while the watch loop completes cycles, `503 {"status":"stalled"}` if no cycle finished within `max(3 × interval, interval + 2m)`. It reflects Forseti's own liveness, not Pi-hole reachability — use `forseti_target_reachable` / `forseti_target_health` for that. The image's Docker `HEALTHCHECK` runs `forseti healthcheck`, which probes this endpoint (port from `metrics.port`, default 9099) and keeps working while an invalid config edit is pending.

Do not expose port 9099 publicly; `/metrics`, `/healthz` and `/status` are unauthenticated.

## Docker Compose

```yaml
services:
  pihole:
    image: pihole/pihole:latest
    ports:
      - "80:80"
    environment:
      FTLCONF_webserver_api_password: ${PIHOLE_PASSWORD}

  forseti:
    image: ghcr.io/st0o0/forseti:latest
    depends_on:
      - pihole
    volumes:
      - ./forseti.yml:/config/forseti.yml:ro
    environment:
      PIHOLE_PASSWORD: ${PIHOLE_PASSWORD}
    ports:
      - "9099:9099"
    command: ["watch", "--config", "/config/forseti.yml"]
```

## Metrics

Forseti exposes a Prometheus endpoint (default `:9099/metrics`).
Individual metric families can be disabled via `metrics.collectors.*` toggles in the config.

### Reconciliation

| Metric | Labels | Description |
|--------|--------|-------------|
| `forseti_reconcile_runs_total` | `target`, `status` | Reconcile/sync run count. `success`: no errors; `partial`: target reached but some item writes failed; `error`: run could not complete |
| `forseti_reconcile_duration_seconds` | `target` | Reconcile cycle duration (histogram) |
| `forseti_reconcile_changes_total` | `target`, `type`, `action` | Changes successfully applied (add/update/delete per resource type); failed writes are not counted |
| `forseti_reconcile_drift` | `target`, `type` | Current drift count per resource type |
| `forseti_reconcile_item_errors_total` | `target`, `type` | Failed item writes per resource type (`other` if not tied to one) |
| `forseti_reconcile_last_run_timestamp` | `target` | Unix time of the last completed (not skipped) run |
| `forseti_reconcile_last_success_timestamp` | `target` | Unix time of the last `success` or `partial` run |

### Pi-hole stats

| Metric | Labels |
|--------|--------|
| `forseti_dns_queries` | `target` |
| `forseti_dns_queries_blocked` | `target` |
| `forseti_blocked_percentage` | `target` |
| `forseti_domains_blocked` | `target` |
| `forseti_clients_active` | `target` |
| `forseti_clients_seen` | `target` |
| `forseti_blocking_status` | `target` |

### Query details

| Metric | Labels |
|--------|--------|
| `forseti_dns_queries_by_type` | `target`, `query_type` |
| `forseti_dns_queries_by_status` | `target`, `status` |
| `forseti_dns_replies_by_type` | `target`, `reply_type` |

### Upstreams

| Metric | Labels |
|--------|--------|
| `forseti_upstream_queries` | `target`, `upstream`, `name`, `port` |
| `forseti_upstream_response_seconds` | `target`, `upstream`, `name`, `port` |

### Gravity

| Metric | Labels |
|--------|--------|
| `forseti_gravity_runs_total` | `target`, `trigger` |
| `forseti_gravity_duration_seconds` | `target` |
| `forseti_gravity_last_run_timestamp` | `target` |
| `forseti_gravity_last_update_timestamp` | `target` |

### Settings & sessions

| Metric | Labels |
|--------|--------|
| `forseti_settings_drift` | `target`, `setting` |
| `forseti_session_active` | -- |
| `forseti_session_reauth_total` | `target` |
| `forseti_dhcp_leases_active` | `target` |
| `forseti_new_blocked_domains_24h` | `target` |

### Operational

| Metric | Labels | Note |
|--------|--------|------|
| `forseti_build_info` | `version`, `mode` | |
| `forseti_target_reachable` | `target` | `1` = reachable, `0` = unreachable (session/readiness only, independent of run outcome) |
| `forseti_config_reload_total` | `result` | |
| `forseti_collector_fetches_total` | `target`, `status` | |
| `forseti_collector_cache_hits_total` | `target` | |
| `forseti_collector_cache_stale_total` | `target` | |
| `forseti_target_consecutive_failures` | `target` | Consecutive failed runs (0 after a success) |
| `forseti_target_health` | `target` | `0` = healthy, `1` = degraded, `2` = down (5+ consecutive failures); updated after every run in both modes |

## Critical invariants

- **Managed-marker safety**: Forseti only touches entries tagged `[forseti]` (config mode) or `[forseti-sync]` (sync mode) in the comment field. Manual UI entries are never modified or deleted. Local DNS records have no comment field: they are only deleted when `local_dns_purge` is enabled (opt-in in config mode, default in sync mode).
- **Reconcile order**: groups -> adlists -> deny -> allow -> local DNS -> CNAME -> clients. Groups must exist before anything references them.
- **Gravity trigger**: automatic on adlist changes (if `gravity_on_change: true`) and on cron schedule per target. Never for domain/client/DNS changes alone.
- **CNAME restart**: adding or removing CNAME records triggers an FTL restart (brief DNS outage).
- **Session pool**: 1 persistent session per target with auto-reauth on 401. Pi-hole allows max 16 concurrent sessions.
- **API concurrency gate**: all subsystems (reconciler, collector, gravity) share a per-target semaphore. The collector serves stale cached data when the target is busy instead of timing out. Configure `api.max_concurrent: 1` for resource-constrained targets to serialize all API access.
- **Modes are exclusive**: config and sync cannot run simultaneously.

## Building

```bash
go build -o forseti ./cmd/forseti
go test -race ./...
docker build -t forseti .
```

## License

[MIT](LICENSE.md)
