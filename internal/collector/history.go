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

type HistoryCollector struct {
	pool     *session.Pool
	interval time.Duration
	metrics  *metrics.Server
	toggles  config.CollectorToggles
	log      *slog.Logger
	reach    Reachability

	mu      sync.RWMutex
	targets []config.Target
}

func NewHistoryCollector(pool *session.Pool, targets []config.Target, interval time.Duration, m *metrics.Server, toggles config.CollectorToggles) *HistoryCollector {
	return &HistoryCollector{
		pool:     pool,
		interval: interval,
		metrics:  m,
		toggles:  toggles,
		targets:  targets,
		log:      logging.Component("history"),
		reach:    targetstate.New(m),
	}
}

func (h *HistoryCollector) SetReachability(r Reachability) {
	h.reach = r
}

func (h *HistoryCollector) UpdateTargets(targets []config.Target) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.targets = targets
}

func (h *HistoryCollector) Start(ctx context.Context) {
	if !h.toggles.IsEnabled("history") {
		return
	}

	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.CollectOnce(ctx)
		}
	}
}

func (h *HistoryCollector) CollectOnce(ctx context.Context) {
	if !h.toggles.IsEnabled("history") {
		return
	}
	h.mu.RLock()
	targets := make([]config.Target, len(h.targets))
	copy(targets, h.targets)
	h.mu.RUnlock()

	var wg sync.WaitGroup
	for _, target := range targets {
		wg.Add(1)
		go func(t config.Target) {
			defer wg.Done()
			h.collectTarget(ctx, t)
		}(target)
	}
	wg.Wait()
}

func (h *HistoryCollector) collectTarget(_ context.Context, target config.Target) {
	if !h.pool.TryAcquire(target.Name) {
		h.log.Debug("target busy, skipping this cycle", "target", target.Name)
		return
	}
	defer h.pool.Release(target.Name)

	timeout := target.API.TimeoutOrDefault()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client, err := h.pool.Get(target)
	if err != nil {
		h.log.Debug("session error", "target", target.Name, "error", err)
		h.pool.Invalidate(target.Name)
		h.reach.Observe(target.Name, "history", err)
		return
	}

	_ = ctx
	entries, err := client.GetHistory()
	if err != nil {
		if pihole.IsNotFound(err) {
			h.log.Debug("history endpoint not available", "target", target.Name)
		} else {
			h.log.Debug("history fetch failed", "target", target.Name, "error", err)
		}
		return
	}

	h.metrics.UpdateHistory(target.Name, entries)
}
