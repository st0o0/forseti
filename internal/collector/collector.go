package collector

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/st0o0/forseti/internal/config"
	"github.com/st0o0/forseti/internal/metrics"
	"github.com/st0o0/forseti/internal/pihole"
	"github.com/st0o0/forseti/internal/session"
)

type cachedStats struct {
	stats     *pihole.Stats
	blocking  bool
	upstreams []pihole.UpstreamStats
	fetchedAt time.Time
}

type Collector struct {
	pool    *session.Pool
	targets []config.Target
	ttl     time.Duration
	metrics *metrics.Server

	mu    sync.RWMutex
	cache map[string]*cachedStats
}

func NewCollector(pool *session.Pool, targets []config.Target, ttl time.Duration, m *metrics.Server) *Collector {
	return &Collector{
		pool:    pool,
		targets: targets,
		ttl:     ttl,
		metrics: m,
		cache:   make(map[string]*cachedStats),
	}
}

func (c *Collector) Collect(ctx context.Context) {
	start := time.Now()
	defer func() {
		c.metrics.ObserveCollectorDuration(time.Since(start))
	}()

	var stale []config.Target

	c.mu.RLock()
	now := start
	for _, t := range c.targets {
		cached, ok := c.cache[t.Name]
		if !ok || now.Sub(cached.fetchedAt) >= c.ttl {
			stale = append(stale, t)
		} else {
			c.metrics.RecordCollectorCacheHit(t.Name)
		}
	}
	c.mu.RUnlock()

	if len(stale) == 0 {
		return
	}

	var wg sync.WaitGroup
	for _, target := range stale {
		wg.Add(1)
		go func(t config.Target) {
			defer wg.Done()
			c.collectTarget(ctx, t)
		}(target)
	}
	wg.Wait()
}

func (c *Collector) collectTarget(ctx context.Context, target config.Target) {
	client, err := c.pool.Get(target)
	if err != nil {
		slog.Error("collector session error", "target", target.Name, "error", err)
		c.metrics.MarkTargetUnreachable(target.Name)
		c.metrics.RecordCollectorFetch(target.Name, "error")
		return
	}

	type statsResult struct {
		stats *pihole.Stats
		err   error
	}
	type blockingResult struct {
		blocking bool
		err      error
	}
	type upstreamsResult struct {
		upstreams []pihole.UpstreamStats
		err       error
	}

	statsCh := make(chan statsResult, 1)
	blockingCh := make(chan blockingResult, 1)
	upstreamsCh := make(chan upstreamsResult, 1)

	go func() {
		s, err := client.GetStats()
		statsCh <- statsResult{s, err}
	}()
	go func() {
		b, err := client.GetBlockingStatus()
		blockingCh <- blockingResult{b, err}
	}()
	go func() {
		u, err := client.GetUpstreams()
		upstreamsCh <- upstreamsResult{u, err}
	}()

	select {
	case <-ctx.Done():
		slog.Warn("collector timeout", "target", target.Name)
		c.metrics.RecordCollectorFetch(target.Name, "error")
		return
	case sr := <-statsCh:
		if sr.err != nil {
			slog.Error("stats error", "target", target.Name, "error", sr.err)
			c.metrics.RecordCollectorFetch(target.Name, "error")
			return
		}
		c.metrics.RecordCollectorFetch(target.Name, "success")

		br := <-blockingCh
		ur := <-upstreamsCh

		entry := &cachedStats{
			stats:     sr.stats,
			fetchedAt: time.Now(),
		}

		c.metrics.UpdateStats(target.Name, sr.stats)

		if br.err != nil {
			slog.Warn("blocking status error", "target", target.Name, "error", br.err)
		} else {
			entry.blocking = br.blocking
			c.metrics.UpdateBlockingStatus(target.Name, br.blocking)
		}

		if ur.err != nil {
			slog.Warn("upstreams error", "target", target.Name, "error", ur.err)
		} else {
			entry.upstreams = ur.upstreams
			c.metrics.UpdateUpstreams(target.Name, ur.upstreams)
		}

		c.mu.Lock()
		c.cache[target.Name] = entry
		c.mu.Unlock()
	}
}
