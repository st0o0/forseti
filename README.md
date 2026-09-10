# Forseti

Declarative GitOps controller for [Pi-hole v6](https://pi-hole.net/).
Reconciles adlists, allow/deny domains, local DNS, groups, and clients from a
single YAML config against one or more Pi-hole instances. Includes a built-in
Prometheus metrics endpoint for both Pi-hole stats and reconciliation telemetry.

Named after [Forseti](https://en.wikipedia.org/wiki/Forseti), the Norse god of
justice -- he reconciles desired state with actual state and ensures every
instance is in order.

## Features

- **Declarative config** -- adlists, allow/deny lists, local DNS, groups, and
  clients defined in one YAML file
- **Multi-instance** -- reconcile any number of Pi-hole v6 targets from one
  config
- **Managed-marker purge** -- only touches entries it created (`[forseti]` tag),
  manual UI changes are left alone
- **Diff-based** -- compares desired vs actual state and applies the minimum
  set of changes
- **Built-in metrics** -- Prometheus `/metrics` endpoint with Pi-hole stats and
  Forseti reconciliation telemetry
- **Plan/Apply workflow** -- dry-run before committing changes
- **Watch mode** -- daemon with periodic reconcile loop and metrics server

## Quick start

```bash
docker run --rm -v ./forseti.yml:/config/forseti.yml:ro \
  ghcr.io/st0o0/forseti:latest apply --config /config/forseti.yml
```

## Configuration

See [`.env.example`](.env.example) for required environment variables.

```yaml
# forseti.yml
metrics:
  port: 9099
  path: /metrics
  scrape_interval: 30s

targets:
  - name: pihole-1
    url: http://pihole-host:80
    password: ${PIHOLE_PASSWORD}

reconcile:
  interval: 5m
  marker: "[forseti]"
  gravity_on_change: true

adlists:
  - url: https://example.com/blocklist.txt
    comment: "Example blocklist"

deny:
  - domain: ads.example.com

allow:
  - domain: safe.example.com

local_dns:
  - domain: internal.lan
    ip: 192.168.1.100

groups:
  - name: default
    adlists: all
    deny: all
    allow: all

clients:
  - match: "192.168.1.0/24"
    group: default
```

## Commands

```bash
forseti plan    --config forseti.yml   # dry-run: show what would change
forseti apply   --config forseti.yml   # reconcile desired state
forseti watch   --config forseti.yml   # daemon mode with metrics + periodic reconcile
forseti version                        # print version
forseti healthcheck                    # liveness probe
```

## Metrics

Forseti exposes a Prometheus endpoint (default `:9099/metrics`) with:

**Reconciliation metrics:**
- `forseti_reconcile_runs_total{target, status}`
- `forseti_reconcile_duration_seconds{target}`
- `forseti_reconcile_changes_total{target, type, action}`

**Pi-hole metrics (per target):**
- `pihole_queries_total{target}`
- `pihole_blocked_total{target}`
- `pihole_gravity_size{target}`
- `pihole_cache_size{target}`
- `pihole_status{target}`

## Building

```bash
go build -o forseti ./cmd/forseti
docker build -t forseti .
```

## License

[MIT](LICENSE.md)
