## Context

Watch mode in `cmd/forseti/main.go` runs a `select` loop with a ticker and context cancellation. The config pointer (`*config.Config`) is used by `reconcileAll` and `forsetisync.NewSyncer`. `config.Load()` is stateless — it reads the file, expands env vars, parses, validates, and returns a new `*Config` each time. This makes reload straightforward: call `Load()` again and swap the pointer.

The collector and gravity scheduler are initialized once from `cfg.Targets` and `cfg.Metrics.ScrapeInterval`. A full refresh of these subsystems on config change adds complexity. The design limits the reload scope to what matters most: the reconcile/sync config and target list.

## Goals / Non-Goals

**Goals:**
- Detect config file changes and reload validated config in watch mode
- Update ticker interval if reconcile/sync interval changed
- Expose reload success/failure metric
- Keep the existing config if reload fails (validation error, file missing, etc.)

**Non-Goals:**
- Hot-reloading the metrics server port/path (requires listener restart — out of scope)
- Hot-reloading collector scrape interval or gravity schedules (requires subsystem restart — future work)
- Supporting multiple config files or config directories

## Decisions

### 1. Poll-based change detection over fsnotify

**Choice**: Check the config file's mtime before each reconcile/sync tick. If mtime changed since last load, trigger a reload.

**Why over fsnotify**: Forseti runs in scratch Docker containers where inotify edge cases (volume mounts, ConfigMap atomic swaps, NFS) are common failure modes. Poll-based detection is universally reliable, adds zero dependencies, and the check cost (one `os.Stat`) is negligible against a 5-minute reconcile interval. fsnotify would add a dependency and a goroutine for marginal latency improvement on an operation that runs every few minutes.

### 2. Reload at the top of each tick, not on a separate goroutine

**Choice**: Before each reconcile/sync pass, stat the config file and reload if changed. No separate watcher goroutine.

**Why**: Simpler concurrency model. The config pointer is only read in the ticker goroutine, so no mutex is needed. The reload happens synchronously before the work that depends on it.

### 3. Full `config.Load()` call — no incremental diffing

**Choice**: Reload calls `config.Load()` which re-reads, re-expands env vars, re-parses, and re-validates the entire file. If it succeeds, the old config pointer is replaced wholesale.

**Why**: `Load()` is already idempotent and validates the entire config. Incremental diffing would add complexity for no benefit — the config file is small.

### 4. Ticker reset on interval change

**Choice**: If the new config's reconcile/sync interval differs from the current ticker, call `ticker.Reset(newInterval)`.

**Why**: `time.Ticker.Reset` is available since Go 1.15, is safe to call from the same goroutine, and avoids creating a new ticker.

## Risks / Trade-offs

- **Mtime granularity** → On filesystems with 1-second mtime resolution, rapid edits within the same second may be missed until the next tick. Acceptable given multi-minute reconcile intervals.
- **Env var changes not detected** → If `${VAR}` references change but the file mtime doesn't, the reload won't fire. This is expected — env var changes require a restart anyway (Docker semantics).
- **Collector/gravity not refreshed** → Adding or removing targets in the config won't update the collector or gravity scheduler until restart. Documented as a known limitation.
