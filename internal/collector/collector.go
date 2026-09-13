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
	toggles config.CollectorToggles

	mu    sync.RWMutex
	cache map[string]*cachedStats
}

func NewCollector(pool *session.Pool, targets []config.Target, ttl time.Duration, m *metrics.Server, toggles config.CollectorToggles) *Collector {
	return &Collector{
		pool:    pool,
		targets: targets,
		ttl:     ttl,
		metrics: m,
		toggles: toggles,
		cache:   make(map[string]*cachedStats),
	}
}

func (c *Collector) UpdateTargets(targets []config.Target) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.targets = targets
	for name := range c.cache {
		found := false
		for _, t := range targets {
			if t.Name == name {
				found = true
				break
			}
		}
		if !found {
			delete(c.cache, name)
		}
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

func (c *Collector) collectTarget(_ context.Context, target config.Target) {
	if !c.pool.TryAcquire(target.Name) {
		slog.Debug("target busy, serving stale", "target", target.Name)
		c.metrics.RecordCollectorCacheStale(target.Name)
		return
	}
	defer c.pool.Release(target.Name)

	timeout := target.API.TimeoutOrDefault()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client, err := c.pool.Get(target)
	if err != nil {
		slog.Error("collector session error", "target", target.Name, "error", err)
		c.pool.Invalidate(target.Name)
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
	type dhcpResult struct {
		leases []pihole.APIDHCPLease
		err    error
	}

	statsCh := make(chan statsResult, 1)
	blockingCh := make(chan blockingResult, 1)
	upstreamsCh := make(chan upstreamsResult, 1)
	dhcpCh := make(chan dhcpResult, 1)

	if c.toggles.IsEnabled("stats") {
		go func() {
			s, err := client.GetStats()
			statsCh <- statsResult{s, err}
		}()
	} else {
		statsCh <- statsResult{}
	}

	if c.toggles.IsEnabled("blocking") {
		go func() {
			b, err := client.GetBlockingStatus()
			blockingCh <- blockingResult{b, err}
		}()
	} else {
		blockingCh <- blockingResult{}
	}

	if c.toggles.IsEnabled("upstreams") {
		go func() {
			u, err := client.GetUpstreams()
			upstreamsCh <- upstreamsResult{u, err}
		}()
	} else {
		upstreamsCh <- upstreamsResult{}
	}

	if c.toggles.IsEnabled("dhcp") {
		go func() {
			l, err := client.GetDHCPLeases()
			dhcpCh <- dhcpResult{l, err}
		}()
	} else {
		dhcpCh <- dhcpResult{}
	}

	select {
	case <-ctx.Done():
		slog.Warn("collector timeout", "target", target.Name)
		c.metrics.RecordCollectorFetch(target.Name, "error")
		return
	case sr := <-statsCh:
		if c.toggles.IsEnabled("stats") {
			if sr.err != nil {
				slog.Error("stats error", "target", target.Name, "error", sr.err)
				c.pool.Invalidate(target.Name)
				c.metrics.MarkTargetUnreachable(target.Name)
				c.metrics.RecordCollectorFetch(target.Name, "error")
				return
			}
			c.metrics.UpdateStats(target.Name, sr.stats)
		}
		c.metrics.RecordCollectorFetch(target.Name, "success")

		br := <-blockingCh
		ur := <-upstreamsCh
		dr := <-dhcpCh

		entry := &cachedStats{
			stats:     sr.stats,
			fetchedAt: time.Now(),
		}

		if c.toggles.IsEnabled("blocking") {
			if br.err != nil {
				slog.Warn("blocking status error", "target", target.Name, "error", br.err)
			} else {
				entry.blocking = br.blocking
				c.metrics.UpdateBlockingStatus(target.Name, br.blocking)
			}
		}

		if c.toggles.IsEnabled("upstreams") {
			if ur.err != nil {
				slog.Warn("upstreams error", "target", target.Name, "error", ur.err)
			} else {
				entry.upstreams = ur.upstreams
				c.metrics.UpdateUpstreams(target.Name, ur.upstreams)
			}
		}

		if c.toggles.IsEnabled("dhcp") {
			if dr.err != nil {
				slog.Warn("dhcp leases error", "target", target.Name, "error", dr.err)
			} else {
				c.metrics.UpdateDHCPLeases(target.Name, len(dr.leases))
			}
		}

		c.mu.Lock()
		c.cache[target.Name] = entry
		c.mu.Unlock()
	}
}
