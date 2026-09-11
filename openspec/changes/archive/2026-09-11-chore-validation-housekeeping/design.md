## Context

The `validate()` function in `internal/config/config.go` checks for missing targets, duplicate names, invalid domains, and mode-specific constraints. Duration fields are defaulted in `applyDefaults()` but never checked for validity — a user-supplied `interval: -5m` passes through silently.

## Goals / Non-Goals

**Goals:**
- Reject invalid duration values at config load time with clear error messages
- Establish minimum interval bounds to prevent operational mistakes
- Clean up untracked file noise with a proper `.gitignore`

**Non-Goals:**
- Maximum interval validation (users may legitimately want hourly reconciles)
- Validating YAML structure beyond what `yaml.v3` already catches

## Decisions

### 1. Validate after defaults are applied

Validation runs after `applyDefaults()`, so only explicitly negative values and values below the minimum are rejected. Zero-value fields that were defaulted pass fine.

### 2. Minimum bounds: 10s reconcile/sync, 5s scrape

These prevent accidental tight loops while allowing fast iteration for testing. The bounds are hardcoded constants — no need for configurability.

### 3. Straightforward `.gitignore`

Cover `*_cov.out`, `coverage/`, Go binary output, and common editor files. Keep it minimal.

## Risks / Trade-offs

- **Existing configs with sub-minimum intervals break** → Unlikely in practice since defaults are 30s/5m. Error message will clearly state the minimum.
