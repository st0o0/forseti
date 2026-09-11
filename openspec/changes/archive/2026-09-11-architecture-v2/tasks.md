# Tasks: Architecture v2

## Phase 1: Session Pool

- [x] Create `internal/session/pool.go` — Pool struct with Get/Close
- [x] Add auto-reauth to `pihole.Client.doJSON` (detect 401, re-Login, retry once)
- [x] Add `pihole.Client` methods to check session validity
- [x] Tests: pool concurrency, auto-reauth on 401, close all sessions
- [x] Refactor `reconcileAll()` in main.go to use session pool instead of `NewClient()` per call

## Phase 2: Stats Collector

- [x] Create `internal/collector/collector.go` — Collector with scrape+cache logic
- [x] Wrap promhttp handler to call `Collect()` before serving
- [x] Parallel collection across targets with 10s context timeout
- [x] TTL cache per target (keyed on target name, expires after `scrape_interval`)
- [x] Remove stats scraping from `reconcileAll()` (main.go:196-224)
- [x] Fix GaugeVec reset race — replace `Reset()` + re-set with explicit label management
- [x] Wire `scrape_interval` config field to collector TTL
- [x] Tests: cache hit/miss, parallel collection, timeout behavior

## Phase 3: Gravity Scheduler

- [x] Add `gravity.schedule` field to `config.Target` struct
- [x] Add cron parser (built-in, no external dependency — network unavailable)
- [x] Create `internal/gravity/scheduler.go` — Scheduler with cron-based triggering
- [x] Add `TriggerNow(targetName, reason)` for adlist-change triggers
- [x] Register gravity metrics (runs, duration, last_run, errors)
- [x] Config validation: parse cron expressions at load time
- [x] Wire into main.go runWatch
- [x] Tests: schedule parsing, trigger recording, staggered scheduling

## Phase 4: Mode system

- [x] Add `mode` field to config (validate: "config" | "sync")
- [x] Validate mode-specific fields (reconcile block only in config, sync block only in sync)
- [x] Add `role` field to target (primary/replica, required in sync mode)
- [x] Add `sync` config block (interval, primary, resources)
- [x] Create `internal/sync/syncer.go` — read primary, diff, write replicas
- [x] Sync uses same marker pattern as reconciler ([forseti-sync])
- [x] Refactor main.go `runWatch` to switch on mode
- [x] Tests: sync diff, primary→replica flow

## Phase 5: Cleanup

- [x] Slim down `internal/metrics/metrics.go` — collection logic moved to collector in Phase 2
- [x] Remove double-login pattern from main.go — done in Phase 1 (session pool)
- [x] Update CLAUDE.md architecture section
- [x] Update forseti.dev.yml with new config fields (mode, gravity)
