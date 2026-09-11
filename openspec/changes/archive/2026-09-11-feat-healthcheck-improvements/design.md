## Context

The healthcheck subcommand (`main.go:38-39`) is a no-op that prints "ok". It accepts no flags and performs no I/O. The session pool (`session.Pool`) manages persistent sessions for daemon mode, but the healthcheck is a one-shot command — it does not need or use the pool.

Pi-hole connectivity is already validated by `pihole.Client.Login()`, which POSTs to `/api/auth` and returns an error on failure. This is sufficient as a health probe.

## Goals / Non-Goals

**Goals:**
- Healthcheck verifies each configured Pi-hole target is reachable via Login
- Per-target status output for operator diagnostics
- Bounded execution time via per-target timeout
- Non-zero exit code on any target failure

**Non-Goals:**
- Session pool health (pool is only relevant in watch mode, not in the one-shot healthcheck)
- Readiness probe semantics (liveness is sufficient — forseti recovers on next reconcile cycle)
- Custom timeout flag (hardcoded 5s is adequate for health probes; can be added later)

## Decisions

### 1. Reuse Login as connectivity probe

`pihole.Client.Login()` already verifies API reachability and valid credentials. Creating a separate "ping" endpoint would duplicate logic. Login, check, immediately Close.

### 2. One-shot clients, no session pool

The healthcheck creates a fresh `pihole.Client` per target, calls `Login()`, then `Close()`. It does not interact with the session pool since it is a short-lived command, not a daemon.

### 3. Per-target timeout via context

Wrap each `Login()` in a context with 5-second deadline. If the target does not respond within the timeout, report it as unreachable. This prevents the healthcheck from hanging in Docker/Kubernetes probe scenarios.

### 4. Require `--config` flag

The healthcheck needs to know which targets to probe. This makes it consistent with `plan`, `apply`, and `watch`. The Dockerfile HEALTHCHECK CMD will need the config path.

## Risks / Trade-offs

- **Login creates a session** → Each healthcheck invocation creates and immediately closes one session per target. Pi-hole allows max 16 concurrent sessions. At typical probe intervals (10-30s) this is not a concern, but very aggressive probing could accumulate sessions if Close fails.
- **`--config` required** → Breaks the current zero-argument healthcheck. Existing Dockerfiles without `--config` will get an error. This is intentional — the old check was meaningless.
