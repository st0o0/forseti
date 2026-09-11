## Context

Forseti is a greenfield Go project with a complete CI/CD scaffold but stub-only code. All domain knowledge — Pi-hole v6 API details, reconcile logic, config format, metrics design — lives in a single design document (`pihole-gitops-controller.md`) in the repo root. There is no `CLAUDE.md` and the OpenSpec config is the default template.

This change is purely documentation redistribution. No application code is created or modified.

## Goals / Non-Goals

**Goals:**

- Give Claude Code always-loaded project context via `CLAUDE.md`
- Give OpenSpec project awareness via `config.yaml` context
- Capture the four core capabilities as structural specs that future changes can reference and extend
- Eliminate the loose design doc by distributing every line to a better home

**Non-Goals:**

- Writing behavioral specs with exhaustive edge cases and error scenarios
- Creating implementation code or tests
- Changing CI/CD, Dockerfile, or any existing project configuration
- Documenting deployment topology or operational runbooks

## Decisions

### CLAUDE.md scope: compact reference, not knowledge dump

Keep CLAUDE.md under ~60 lines. It loads on every conversation — bloat costs tokens. Include: architecture sketch (package → responsibility), build/test/lint commands, commit conventions, and the 5 critical invariants (marker purge, reconcile order, gravity trigger, session limit, CNAME restart). Everything else lives in specs where it's accessed on demand.

Alternative considered: Put all domain knowledge in CLAUDE.md. Rejected because it would be 200+ lines loaded every time, most of which is only relevant during specific implementation work.

### Spec depth: structural over behavioral

Specs describe shape, constraints, and key scenarios — enough to prevent wrong turns. They do not enumerate every error path or edge case. This fits a project at skeleton stage where the implementation will surface details that specs written now would get wrong.

Alternative considered: Full behavioral specs with acceptance-test-ready scenarios. Rejected because the Pi-hole API behavior is best discovered during implementation, and over-specified specs become maintenance burden.

### Content distribution: split and delete over archive

The design doc content maps cleanly onto the four specs + CLAUDE.md. After distribution, the original file is deleted. Git history preserves it. No `docs/original-design.md` archive copy.

Alternative considered: Keep as `docs/original-design.md`. Rejected because it would immediately be stale and diverge from the specs that replace it.

### Config.yaml context: terse project summary

The OpenSpec config context field gets a compact summary: tech stack, domain, conventions, package structure. This is injected into every artifact generation prompt — same token-efficiency argument as CLAUDE.md.

## Risks / Trade-offs

- **[Information loss in translation]** → Mitigated by systematic section-by-section mapping. Every section of the design doc has a named destination. Review the diff to verify nothing was dropped.
- **[Specs may be too thin]** → Acceptable. Structural specs are a starting point. Future changes extend them with detailed requirements as implementation surfaces real behavior.
- **[CLAUDE.md invariants may drift from specs]** → CLAUDE.md lists the critical invariants as quick-reference; specs are the source of truth. If they diverge, specs win.
