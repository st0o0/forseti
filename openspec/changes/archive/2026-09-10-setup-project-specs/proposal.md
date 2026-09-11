## Why

The repo has solid CI/CD, release pipelines, and project scaffolding, but all domain knowledge lives in a loose design document (`pihole-gitops-controller.md`) in the root. There is no `CLAUDE.md`, the OpenSpec config has no project context, and no specs exist. Claude Code and OpenSpec have zero project awareness, making every future change start from scratch.

## What Changes

- Create `CLAUDE.md` with architecture overview, build commands, conventions, and critical invariants
- Update `openspec/config.yaml` with project context (tech stack, domain, conventions, package structure)
- Create four structural capability specs covering the core system
- Delete `pihole-gitops-controller.md` after its content is fully distributed

## Capabilities

### New Capabilities

- `config`: YAML config format, env-var expansion, validation, multi-target support
- `pihole-api`: Pi-hole v6 REST API client contract — session auth, CRUD endpoints, batch operations, stats, constraints
- `reconciler`: Reconcile logic — marker-based ownership, three-way diff, ordering constraints, gravity trigger, plan/apply/watch modes
- `metrics`: Prometheus metrics endpoint — forseti_* and pihole_* metric families, labels, scrape configuration

### Modified Capabilities

_None — this is a greenfield project with no existing specs._

## Impact

- No code changes — this is purely documentation and project configuration
- `pihole-gitops-controller.md` is deleted (content redistributed, recoverable from git history)
- All future OpenSpec changes gain project context and can reference capability specs
- Claude Code gains always-loaded project awareness via CLAUDE.md
