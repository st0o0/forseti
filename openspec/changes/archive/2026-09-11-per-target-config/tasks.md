## 1. Config Types & Parsing

- [x] 1.1 Add `Settings` struct to `internal/config` with DNS, Blocking, and Privacy sub-structs matching the curated Forseti format
- [x] 1.2 Add `Settings` field to `Config` struct and `File` field to `Target` struct
- [x] 1.3 Add `TargetOverride` struct for the override file format (settings, content lists, exclude block)
- [x] 1.4 Implement override file loading: resolve `file:` path relative to main config, parse with env-var expansion, reject disallowed fields (targets, mode, metrics, reconcile)
- [x] 1.5 Add validation for settings values (blocking mode enum, non-negative cache size, privacy level 0-4)
- [x] 1.6 Write tests for settings parsing and override file loading (valid, missing file, disallowed fields)

## 2. Merge Engine

- [x] 2.1 Implement deep merge for settings (scalar overwrite at leaf level)
- [x] 2.2 Implement append merge for content lists (groups, adlists, deny, allow, local_dns, cname, clients)
- [x] 2.3 Implement exclude logic: remove entries by natural key per resource type (domain, url, match, name)
- [x] 2.4 Integrate merge into `config.Load()` — produce effective config per target after loading
- [x] 2.5 Run validation on effective (merged) config per target (duplicates, group refs, domain format)
- [x] 2.6 Write tests for merge engine: append, deep merge, exclude, exclude-nonexistent warning, duplicate-after-merge error

## 3. Resolved Target Model

- [x] 3.1 Introduce `ResolvedTarget` type that carries target connection info + effective content + effective settings
- [x] 3.2 Update `config.Load()` return type or add a method to produce `[]ResolvedTarget`
- [x] 3.3 Update all call sites that iterate `cfg.Targets` to use resolved targets (cmd/forseti plan, apply, watch, healthcheck)
- [x] 3.4 Update reconciler `Plan()` and `Apply()` signatures to accept effective config per target instead of global `*Config`

## 4. Pi-hole Config API Client

- [x] 4.1 Add `GetConfig()` method to pihole client — `GET /api/config`, parse response into a nested map or struct
- [x] 4.2 Add `PatchConfig(path string, value any)` method — `PUT /api/config/<path>/<value>` for setting individual config values
- [x] 4.3 Add settings field mapping: Forseti path → Pi-hole API path (table-driven)
- [x] 4.4 Write tests for GetConfig and PatchConfig with mock HTTP responses

## 5. Settings Reconciliation

- [x] 5.1 Implement `reconcileSettings()` — read actual via GetConfig, diff against effective settings, return list of changes
- [x] 5.2 Integrate settings diff into plan output (setting name, current value, desired value)
- [x] 5.3 Integrate settings apply into the reconcile loop (apply changes via PatchConfig, before content reconciliation)
- [x] 5.4 Add settings reconciliation to the `PiholeAPI` interface or keep it separate from content reconciliation
- [x] 5.5 Write tests for settings reconciliation: no changes, one change, multiple changes, no settings declared

## 6. Integration & CLI

- [x] 6.1 Update `reconcileAll()` in cmd/forseti to pass effective config per target
- [x] 6.2 Update plan command output to show per-target effective config differences and settings diffs
- [x] 6.3 Update sync mode to work with resolved targets (sync mode does not use settings or overrides — verify no regressions)
- [x] 6.4 Update collector and gravity scheduler to work with resolved targets
- [x] 6.5 End-to-end test: multi-target config with overrides, plan output, apply with settings changes
