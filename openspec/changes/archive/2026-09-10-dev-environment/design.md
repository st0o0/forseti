## Context

Forseti has a skeleton CLI and empty package stubs but no way to test against real Pi-hole instances. The existing Dockerfile builds a scratch-based production image. We need a local dev environment that starts two Pi-hole v6 instances and can build/run Forseti against them.

## Goals / Non-Goals

**Goals:**
- Two Pi-hole v6 instances with distinct ports for multi-target testing
- Forseti container built from the existing Dockerfile
- A dev config file with sample resources covering all resource types
- Makefile for common dev workflows (build, test, lint, dev lifecycle)

**Non-Goals:**
- Production deployment configuration
- CI/CD pipeline setup
- Persistent Pi-hole data across restarts (ephemeral dev instances are fine)
- DNS resolution through dev Pi-holes from the host

## Decisions

### Two Pi-holes, not one
Multi-target reconciliation is a core feature. Testing with two instances from the start surfaces ordering and isolation bugs early. Both instances start clean on every `dev-up`.

### Use existing Dockerfile for Forseti container
The production Dockerfile already builds a static binary. Reusing it in Compose avoids a separate dev Dockerfile. For faster iteration, developers can also run `go run ./cmd/forseti` directly on the host against the exposed Pi-hole ports.

### Ephemeral Pi-hole data
No Docker volumes for Pi-hole data. Each `dev-up` starts fresh instances. This is intentional — Forseti is a reconciler, so starting from a clean state is the correct test scenario.

### Port mapping scheme
| Service | Web UI | DNS |
|---------|--------|-----|
| pihole-alpha | 8080 | 53001 |
| pihole-beta | 8081 | 53002 |

High ports avoid conflicts with host DNS (port 53) and common dev servers.

### Pi-hole v6 configuration
Passwords set via `FTLCONF_webserver_api_password`. Web interface on port 80 internally. The `/api/` endpoints are available without additional configuration on v6.

### Makefile over shell scripts
A Makefile is idiomatic for Go projects, provides tab-completion, and self-documents available targets via `make help`.

## Risks / Trade-offs

- **Pi-hole v6 image updates may break API** → Pin to a specific v6 tag once stable, use `latest` during initial development
- **Port conflicts** → Using high ports (8080/8081/53001/53002) minimizes conflicts but could still collide with other dev services
- **No DNS testing from host** → Dev Pi-holes serve DNS on non-standard ports; host resolver won't use them. Manual `dig @localhost -p 53001` works for verification
