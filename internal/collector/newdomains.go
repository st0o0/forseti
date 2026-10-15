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

const newDomainsWindow = 24 * time.Hour

// NewDomainsCollector detects, per target, domains blocked for the first
// time within a rolling 24h window. It runs on its own interval, separate
// from the shared TTL-cached stats collector, since /api/queries over a
// window is a heavier call than the summary endpoints and does not need
// scrape-level freshness.
type NewDomainsCollector struct {
	pool     *session.Pool
	interval time.Duration
	metrics  *metrics.Server
	toggles  config.CollectorToggles
	log      *slog.Logger
	reach    Reachability

	mu      sync.RWMutex
	targets []config.Target

	// logged remembers, per target and domain, the first-seen time of
	// domains already reported via new_blocked_domain, so each is logged
	// once per window. In-memory only; it does not affect the gauge.
	loggedMu sync.Mutex
	logged   map[string]map[string]time.Time
}

func NewNewDomainsCollector(pool *session.Pool, targets []config.Target, interval time.Duration, m *metrics.Server, toggles config.CollectorToggles) *NewDomainsCollector {
	return &NewDomainsCollector{
		pool:     pool,
		interval: interval,
		metrics:  m,
		toggles:  toggles,
		targets:  targets,
		log:      logging.Component("newdomains"),
		reach:    targetstate.New(m),
		logged:   make(map[string]map[string]time.Time),
	}
}

// SetReachability replaces the reachability tracker (see Collector).
func (c *NewDomainsCollector) SetReachability(r Reachability) {
	c.reach = r
}

// shouldLog reports whether domain on target has not been logged within its
// current window, and records it. Entries older than the window are pruned.
func (c *NewDomainsCollector) shouldLog(target, domain string, firstSeen, now time.Time) bool {
	c.loggedMu.Lock()
	defer c.loggedMu.Unlock()
	seen := c.logged[target]
	if seen == nil {
		seen = make(map[string]time.Time)
		c.logged[target] = seen
	}
	for d, t := range seen {
		if now.Sub(t) > newDomainsWindow {
			delete(seen, d)
		}
	}
	if _, ok := seen[domain]; ok {
		return false
	}
	seen[domain] = firstSeen
	return true
}

func (c *NewDomainsCollector) UpdateTargets(targets []config.Target) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.targets = targets
}

// Start runs the detection loop until ctx is cancelled. If the new_domains
// toggle is disabled, Start returns immediately without making any API calls.
func (c *NewDomainsCollector) Start(ctx context.Context) {
	if !c.toggles.IsEnabled("new_domains") {
		return
	}

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.CollectOnce(ctx)
		}
	}
}

// CollectOnce runs a single detection cycle across all targets. Start calls
// this on its own ticker; it is also exported so callers (and tests) can
// trigger a cycle deterministically without waiting on the interval.
func (c *NewDomainsCollector) CollectOnce(ctx context.Context) {
	if !c.toggles.IsEnabled("new_domains") {
		return
	}
	c.mu.RLock()
	targets := make([]config.Target, len(c.targets))
	copy(targets, c.targets)
	c.mu.RUnlock()

	var wg sync.WaitGroup
	for _, target := range targets {
		wg.Add(1)
		go func(t config.Target) {
			defer wg.Done()
			c.collectTarget(ctx, t)
		}(target)
	}
	wg.Wait()
}

func (c *NewDomainsCollector) collectTarget(_ context.Context, target config.Target) {
	if !c.pool.TryAcquire(target.Name) {
		c.log.Debug("target busy, skipping this cycle", "target", target.Name)
		return
	}
	defer c.pool.Release(target.Name)

	timeout := target.API.TimeoutOrDefault()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client, err := c.pool.Get(target)
	if err != nil {
		c.log.Debug("session error", "target", target.Name, "error", err)
		c.pool.Invalidate(target.Name)
		c.reach.Observe(target.Name, "newdomains", err)
		return
	}

	now := time.Now()
	windowStart := now.Add(-newDomainsWindow)

	candidates, err := client.GetQueries(pihole.QueryHistoryOptions{
		From:     windowStart,
		Until:    now,
		Statuses: pihole.BlockedQueryStatuses,
	})
	if err != nil {
		c.log.Warn("query history failed", "target", target.Name, "error", err)
		return
	}

	type candidate struct {
		firstSeen time.Time
		count     int
	}
	byDomain := make(map[string]*candidate)
	for _, rec := range candidates {
		cand, ok := byDomain[rec.Domain]
		if !ok {
			cand = &candidate{firstSeen: time.Unix(int64(rec.Time), 0)}
			byDomain[rec.Domain] = cand
		}
		cand.count++
		if t := time.Unix(int64(rec.Time), 0); t.Before(cand.firstSeen) {
			cand.firstSeen = t
		}
	}

	select {
	case <-ctx.Done():
		c.log.Warn("detection timed out", "target", target.Name)
		return
	default:
	}

	newCount := 0
	for domain, cand := range byDomain {
		prior, err := client.GetQueries(pihole.QueryHistoryOptions{
			Until:    windowStart,
			Domain:   domain,
			Statuses: pihole.BlockedQueryStatuses,
			Length:   1,
		})
		if err != nil {
			c.log.Debug("existence check failed", "target", target.Name, "domain", domain, "error", err)
			continue
		}
		if len(prior) > 0 {
			continue
		}

		newCount++
		if !c.shouldLog(target.Name, domain, cand.firstSeen, now) {
			continue
		}
		c.log.Info("new_blocked_domain",
			"target", target.Name,
			"domain", domain,
			"first_seen", cand.firstSeen.Format(time.RFC3339),
			"count_24h", cand.count,
		)
	}

	c.metrics.SetNewBlockedDomains24h(target.Name, newCount)
}
