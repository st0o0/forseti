# Forseti Architecture v2

## Summary

Refactor Forseti from a monolithic reconcile loop into a mode-based architecture
with four decoupled subsystems: Session Pool, Stats Collector, Gravity Scheduler,
and a mode-exclusive operational core (config or sync).

## Motivation

- Stats collection is coupled to the reconcile loop — data is only as fresh as `reconcile.interval`
- Double login per target per tick wastes sessions (max 16 concurrent on Pi-hole)
- `scrape_interval` config field exists but is unused
- No scheduled gravity updates — only triggered on adlist changes
- No NebulaSync-equivalent (primary → replica sync)

## Modes

Forseti operates in one of two mutually exclusive modes:

| Mode     | Source of Truth | What it does                              |
|----------|----------------|-------------------------------------------|
| `config` | YAML file      | GitOps reconcile: desired state → Pi-hole |
| `sync`   | Primary Pi-hole | Read primary, write replica(s)            |

Both modes share: Session Pool, Stats Collector, Gravity Scheduler, Metrics endpoint.

## Architecture

```
forseti watch
├── Session Pool (always active)
│   └── 1 persistent session per target, auto-reauth on 401
│
├── Mode: config (OR)
│   └── Reconcile loop on reconcile.interval
│       groups → adlists → deny → allow → dns → clients
│
├── Mode: sync (OR)
│   └── Sync loop on sync.interval
│       Read primary → diff → write replica(s)
│
├── Stats Collector (always active)
│   └── Collect-on-scrape + TTL cache (scrape_interval)
│       GetStats + GetBlockingStatus + GetUpstreams (parallel per target)
│
├── Gravity Scheduler (always active)
│   └── Cron schedule per target → TriggerGravity()
│
└── GET /metrics
    └── Serves cached gauges (<1ms typical)
```

## Config Structure

```yaml
mode: config          # "config" | "sync"

metrics:
  port: 9099
  path: /metrics
  scrape_interval: 30s

targets:
  - name: pihole-alpha
    url: http://pihole-alpha:80
    password: ${PIHOLE_ALPHA_PASSWORD}
    gravity:
      schedule: "0 3 * * 0"      # Sunday 03:00

  - name: pihole-beta
    url: http://pihole-beta:80
    password: ${PIHOLE_BETA_PASSWORD}
    gravity:
      schedule: "0 4 * * 0"      # Sunday 04:00 (staggered)

# --- mode: config fields ---
reconcile:
  interval: 30s
  marker: "[forseti]"
  gravity_on_change: true

groups: [...]
adlists: [...]
deny: [...]
allow: [...]
local_dns: [...]
clients: [...]

# --- mode: sync fields ---
sync:
  interval: 5m
  primary: pihole-alpha          # reference to target name
  resources:
    - adlists
    - deny
    - allow
    - local_dns
    - groups
    - clients
```

## Components

```
internal/
  config/       Config parsing + validation (mode-aware)
  pihole/       Pi-hole v6 REST API client (unchanged)
  session/      NEW — Session Pool (persistent, auto-reauth)
  reconcile/    GitOps reconciler (mode: config, refactored to use session pool)
  sync/         NEW — Primary → Replica sync (mode: sync)
  collector/    NEW — Stats Collector (collect-on-scrape + TTL cache)
  gravity/      NEW — Cron-based Gravity Scheduler per target
  metrics/      Registry + metric definitions (slimmed down, no collection logic)
```

## Stats Collector: Scrape + Cache

```
Scrape #1 (t=0s)   → cache empty  → 3 API calls parallel → ~300ms → fill cache
Scrape #2 (t=15s)  → cache valid  → serve cached → <1ms
Scrape #3 (t=30s)  → cache valid  → <1ms
Scrape #4 (t=45s)  → cache expired → 3 API calls → ~300ms → refresh cache
```

- Parallel collection across targets
- 10s context timeout per target (guarantees scraper gets response)
- API endpoints per target: /api/stats/summary, /api/dns/blocking, /api/stats/upstreams

## Session Pool

- One persistent `pihole.Client` per target
- SID cached and reused across all subsystems (reconcile/sync, collector, gravity)
- On HTTP 401: automatic re-Login(), retry the failed request
- On shutdown: Close() all sessions (respects Pi-hole session limit)

## Gravity Scheduler

- Cron expression per target (parsed at startup, validated)
- Staggered schedules to avoid simultaneous DNS outages
- Triggers via existing `pihole.Client.TriggerGravity()`
- Also triggered on adlist changes in mode: config (existing behavior preserved)

### Gravity Metrics

```
forseti_gravity_runs_total{target, trigger}       # trigger: "scheduled" | "adlist_change"
forseti_gravity_duration_seconds{target}
forseti_gravity_last_run_timestamp{target}
forseti_gravity_errors_total{target}
```

## Implementation Order

1. **Session Pool** — foundation for everything else
2. **Stats Collector** — decouple from reconcile, implement scrape+cache
3. **Gravity Scheduler** — cron per target
4. **Mode: sync** — NebulaSync replacement

## Design Decisions

- **Scrape+Cache over pure live-collect**: Guarantees fast response even under load.
  Pure live-collect risks timeout if Pi-hole is slow. Cache miss = live collect.
  Inspired by nbx3/pihole-exporter (TTL cache) + eko/pihole-exporter (parallel collect).
- **Modes are mutually exclusive**: Avoids conflicting writes. Config mode owns the
  YAML source of truth. Sync mode treats the primary Pi-hole as source of truth.
  Running both would create write conflicts.
- **Session pool shared across subsystems**: Pi-hole allows max 16 concurrent sessions.
  Reusing sessions keeps usage at 1 per target regardless of how many subsystems run.
- **Gravity cron per target (not global)**: Different targets can run gravity at
  different times, preventing simultaneous DNS disruption across all instances.
