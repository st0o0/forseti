# Design: Architecture v2

## Session Pool (`internal/session`)

### Interface

```go
type Pool struct {
    mu      sync.RWMutex
    clients map[string]*pihole.Client  // keyed by target name
}

func NewPool() *Pool
func (p *Pool) Get(target config.Target) (*pihole.Client, error)
func (p *Pool) Close() error
```

### Behavior

- `Get()`: returns cached client if SID is set; otherwise calls `Login()` and caches
- On 401 from any API call: client auto-re-authenticates (needs wrapper or hook in `pihole.Client.doJSON`)
- `Close()`: iterates all clients, calls `Close()` on each (DELETE /api/auth)
- Thread-safe: multiple goroutines (collector, reconcile, gravity) access concurrently

### Auto-reauth strategy

Extend `pihole.Client.doJSON` to detect 401, call `Login()`, and retry once.
Avoids leaking session management into every caller.

## Stats Collector (`internal/collector`)

### Interface

```go
type Collector struct {
    pool     *session.Pool
    targets  []config.Target
    ttl      time.Duration
    mu       sync.RWMutex
    cache    map[string]*cachedStats  // per target
    metrics  *metrics.Server
}

type cachedStats struct {
    stats     *pihole.Stats
    blocking  bool
    upstreams []pihole.UpstreamStats
    fetchedAt time.Time
}

func NewCollector(pool *session.Pool, targets []config.Target, ttl time.Duration, m *metrics.Server) *Collector
func (c *Collector) Collect(ctx context.Context) error  // called before /metrics serve
```

### Scrape flow

1. `promhttp.Handler` is wrapped: before serving, call `Collector.Collect(ctx)`
2. `Collect()` checks each target's cache TTL
3. Stale targets are refreshed in parallel (one goroutine per target, 10s timeout)
4. Fresh API data is written to gauges via existing `metrics.Server.UpdateStats()` etc.
5. `promhttp.Handler` then serializes the registry as usual

### Wrapping promhttp

```go
mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
    defer cancel()
    collector.Collect(ctx)
    promhttp.HandlerFor(reg, promhttp.HandlerOpts{}).ServeHTTP(w, r)
})
```

## Gravity Scheduler (`internal/gravity`)

### Interface

```go
type Scheduler struct {
    pool    *session.Pool
    entries []entry
    stop    chan struct{}
    metrics *metrics.Server
}

type entry struct {
    target   config.Target
    schedule cron.Schedule  // from a cron parsing library
    next     time.Time
}

func NewScheduler(pool *session.Pool, targets []config.Target, m *metrics.Server) (*Scheduler, error)
func (s *Scheduler) Start(ctx context.Context)
func (s *Scheduler) TriggerNow(targetName, reason string) error  // for adlist-change triggers
```

### Tick loop

Single goroutine, sleeps until next scheduled run across all entries.
On fire: calls `pool.Get(target).TriggerGravity()`, records metrics.

### Cron library

Use `github.com/robfig/cron/v3` for parsing. Only need `cron.ParseStandard()`
to get a `Schedule` interface — no need for the full cron runner.

## Metrics Server (`internal/metrics`)

Slimmed down: only holds metric definitions and the registry.
No collection logic — that moves to `internal/collector`.

### New gravity metrics

```go
gravityRuns     *prometheus.CounterVec   // labels: target, trigger
gravityDuration *prometheus.HistogramVec // labels: target
gravityLastRun  *prometheus.GaugeVec     // labels: target
gravityErrors   *prometheus.CounterVec   // labels: target
```

### Reset race fix

Replace `GaugeVec.Reset()` + re-set pattern with direct `WithLabelValues().Set()`.
Track known label sets and delete stale ones explicitly instead of resetting all.

## Mode: sync (`internal/sync`) — Phase 4

### Sync loop

```go
func SyncLoop(ctx context.Context, pool *session.Pool, cfg *config.Config) {
    primary := pool.Get(cfg.Sync.Primary)
    for each resource in cfg.Sync.Resources:
        actual := primary.List<Resource>()
        for each replica target:
            replicaActual := replica.List<Resource>()
            diff := computeDiff(actual, replicaActual)
            applyDiff(replica, diff)
}
```

Reuses existing diff functions from `internal/reconcile` where possible.

## Main loop refactor (`cmd/forseti/main.go`)

```go
func runWatch(args []string) int {
    cfg := loadConfig(args)

    pool := session.NewPool()
    defer pool.Close()

    srv := metrics.NewServer(cfg.Metrics.Port, cfg.Metrics.Path)
    collector := collector.NewCollector(pool, cfg.Targets, cfg.Metrics.ScrapeInterval.Duration, srv)
    srv.SetCollector(collector)  // wires collect-before-serve

    gravity := gravity.NewScheduler(pool, cfg.Targets, srv)
    go gravity.Start(ctx)

    switch cfg.Mode {
    case "config":
        srv.SetConfigMetrics(cfg)
        go reconcileLoop(ctx, cfg, pool, srv, gravity)
    case "sync":
        go syncLoop(ctx, cfg, pool, srv)
    }

    go srv.Start()
    <-ctx.Done()
    srv.Shutdown(...)
}
```
