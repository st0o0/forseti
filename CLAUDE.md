# Forseti

Declarative GitOps controller and sync tool for Pi-hole v6. Two modes:
**config** (YAML → Pi-hole reconciliation) and **sync** (primary → replica
sync, NebulaSync replacement). Built-in Prometheus metrics with
collect-on-scrape caching and per-target gravity scheduling.

## Architecture

```
cmd/forseti          CLI entry point (plan, apply, watch, version, healthcheck)
internal/config      YAML parsing with env-var expansion (${VAR}), mode-aware validation
internal/pihole      Pi-hole v6 REST API client (session auth, auto-reauth on 401)
internal/session     Session pool: 1 persistent session per target, shared across subsystems
internal/reconcile   Diff engine: desired (YAML) vs actual (API), plan/apply (mode: config)
internal/sync        Primary → replica sync engine (mode: sync)
internal/collector   Stats collector: collect-on-scrape with TTL cache, parallel per target
internal/gravity     Cron-based gravity scheduler per target + on-demand triggers
internal/metrics     Prometheus /metrics endpoint (forseti_* + pihole_*)
```

Forseti manages gravity.db content (adlists, domains, groups, clients, local DNS).
Docker Compose / FTLCONF_ env vars manage pihole.toml settings (upstreams, rate limits, privacy).

## Build & Test

```
go test -race ./...
go vet ./...
golangci-lint run
docker build -t forseti:ci .
```

## Conventions

- Conventional commits: feat/fix/perf/docs/chore/refactor/test/ci/build/deps
- commitlint enforced, release-please derives versions
- golangci-lint v2, hadolint for Dockerfile
- All code, docs, specs, and commits in English

## Critical Invariants

- **Managed-marker purge**: forseti only touches entries tagged `[forseti]`
  (config mode) or `[forseti-sync]` (sync mode) in the comment field.
  Manual UI entries are never modified or deleted.
- **Reconcile order**: groups -> adlists -> deny -> allow -> local DNS -> clients.
  Groups must exist before anything references them.
- **Gravity trigger**: via gravity scheduler — on adlist changes (if
  gravity_on_change: true) and on cron schedule per target. Never for
  domain/client/DNS changes alone.
- **Session pool**: 1 persistent session per target, auto-reauth on 401.
  Pi-hole allows max 16 concurrent sessions. Pool.Close() on shutdown.
- **Modes are exclusive**: config and sync cannot run simultaneously.
  Config mode uses YAML as source of truth. Sync mode uses primary Pi-hole.
- **CNAME restart**: adding/removing CNAME records triggers FTL restart
  (brief DNS outage).
