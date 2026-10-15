package collector

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/st0o0/forseti/internal/config"
	"github.com/st0o0/forseti/internal/logging"
	"github.com/st0o0/forseti/internal/metrics"
	"github.com/st0o0/forseti/internal/pihole"
	"github.com/st0o0/forseti/internal/session"
	"github.com/st0o0/forseti/internal/targetstate"
)

type cachedStats struct {
	stats     *pihole.Stats
	blocking  bool
	upstreams []pihole.UpstreamStats
	fetchedAt time.Time
	// failed marks a negative cache entry: the last fetch failed and the
	// target is not retried until the TTL expires.
	failed bool
}

// Reachability receives reachability observations (see targetstate.Tracker).
type Reachability interface {
	Observe(target, source string, err error)
}

type Collector struct {
	pool    *session.Pool
	targets []config.Target
	ttl     time.Duration
	metrics *metrics.Server
	toggles config.CollectorToggles
	log     *slog.Logger
	reach   Reachability

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
		log:     logging.Component("collector"),
		reach:   targetstate.New(m),
		cache:   make(map[string]*cachedStats),
	}
}

// SetReachability replaces the reachability tracker, so all subsystems share
// one tracker and report transitions once.
func (c *Collector) SetReachability(r Reachability) {
	c.reach = r
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
		switch {
		case !ok || now.Sub(cached.fetchedAt) >= c.ttl:
			stale = append(stale, t)
		case !cached.failed:
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
		c.log.Debug("target busy, serving stale", "target", target.Name)
		c.metrics.RecordCollectorCacheStale(target.Name)
		return
	}
	defer c.pool.Release(target.Name)

	timeout := target.API.TimeoutOrDefault()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client, err := c.pool.Get(target)
	if err != nil {
		c.fetchFailed(target.Name, "session", err)
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
		c.fetchFailed(target.Name, "timeout", ctx.Err())
		return
	case sr := <-statsCh:
		if c.toggles.IsEnabled("stats") {
			if sr.err != nil {
				c.fetchFailed(target.Name, "stats", sr.err)
				return
			}
			c.metrics.UpdateStats(target.Name, sr.stats)
		}
		c.reach.Observe(target.Name, "collector", nil)
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
				c.log.Debug("blocking status error", "target", target.Name, "error", br.err)
			} else {
				entry.blocking = br.blocking
				c.metrics.UpdateBlockingStatus(target.Name, br.blocking)
			}
		}

		if c.toggles.IsEnabled("upstreams") {
			if ur.err != nil {
				c.log.Debug("upstreams error", "target", target.Name, "error", ur.err)
			} else {
				entry.upstreams = ur.upstreams
				c.metrics.UpdateUpstreams(target.Name, ur.upstreams)
			}
		}

		if c.toggles.IsEnabled("dhcp") {
			if dr.err != nil {
				c.log.Debug("dhcp leases error", "target", target.Name, "error", dr.err)
			} else {
				c.metrics.UpdateDHCPLeases(target.Name, len(dr.leases))
			}
		}

		c.mu.Lock()
		c.cache[target.Name] = entry
		c.mu.Unlock()
	}
}

// fetchFailed reports a failed fetch: the target is marked unreachable (logged
// once by the tracker), the session is dropped and a negative cache entry
// delays the next attempt until the TTL expires.
func (c *Collector) fetchFailed(target, stage string, err error) {
	c.log.Debug("stats fetch failed", "target", target, "stage", stage, "error", err)
	if stage != "timeout" {
		c.pool.Invalidate(target)
	}
	c.reach.Observe(target, "collector", err)
	c.metrics.RecordCollectorFetch(target, "error")

	c.mu.Lock()
	c.cache[target] = &cachedStats{fetchedAt: time.Now(), failed: true}
	c.mu.Unlock()
}
