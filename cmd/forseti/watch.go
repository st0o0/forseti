package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/st0o0/forseti/internal/collector"
	"github.com/st0o0/forseti/internal/config"
	"github.com/st0o0/forseti/internal/gravity"
	"github.com/st0o0/forseti/internal/logging"
	"github.com/st0o0/forseti/internal/metrics"
	"github.com/st0o0/forseti/internal/reconcile"
	"github.com/st0o0/forseti/internal/redact"
	"github.com/st0o0/forseti/internal/session"
	forsetisync "github.com/st0o0/forseti/internal/sync"
	"github.com/st0o0/forseti/internal/targetstate"
	"github.com/st0o0/forseti/internal/worker"
)

func tryReloadConfig(cfgPath string, lastMtime *time.Time) (*config.Config, []config.ResolvedTarget, error) {
	info, err := os.Stat(cfgPath)
	if err != nil {
		return nil, nil, fmt.Errorf("stat config: %w", err)
	}
	if !info.ModTime().After(*lastMtime) {
		return nil, nil, nil
	}
	// Record the mtime before loading so a broken modification is attempted
	// and reported once, not on every tick until the file changes again.
	*lastMtime = info.ModTime()
	newCfg, resolved, err := config.Load(cfgPath)
	if err != nil {
		return nil, nil, err
	}
	return newCfg, resolved, nil
}

func loopInterval(cfg *config.Config) time.Duration {
	if cfg.Mode == config.ModeSync {
		return cfg.Sync.Interval.Duration
	}
	return cfg.Reconcile.Interval.Duration
}

// logConfigLoaded logs the mode-specific configuration summary.
func logConfigLoaded(log *slog.Logger, cfg *config.Config) {
	attrs := []any{"mode", cfg.Mode, "targets", len(cfg.Targets), "interval", loopInterval(cfg), "log_level", cfg.LogLevel}
	if cfg.Mode == config.ModeSync {
		var replicas []string
		for _, t := range cfg.Targets {
			if t.Role == "replica" {
				replicas = append(replicas, t.Name)
			}
		}
		attrs = append(attrs,
			"primary", cfg.Sync.Primary,
			"replicas", replicas,
			"resources", cfg.Sync.Resources,
			"local_dns_purge", cfg.Sync.PurgeLocalDNS(),
		)
	} else {
		attrs = append(attrs,
			"groups", len(cfg.Groups),
			"adlists", len(cfg.Adlists),
			"deny", len(cfg.Deny),
			"allow", len(cfg.Allow),
			"local_dns", len(cfg.LocalDNS),
			"cname", len(cfg.CNAME),
			"clients", len(cfg.Clients),
			"gravity_on_change", cfg.Reconcile.GravityOnChange != nil && *cfg.Reconcile.GravityOnChange,
		)
	}
	log.Info("config loaded", attrs...)
}

func runWatch(args []string) int {
	cfgPath := parseConfigFlag(args, "watch")
	cfg, resolved, err := loadConfig(cfgPath, os.Stdout)
	if err != nil {
		return 1
	}
	log := logging.Component("main")
	started := time.Now()

	log.Info("forseti starting", "version", version, "mode", cfg.Mode, "config", cfgPath, "tz", time.Local.String())
	logConfigLoaded(log, cfg)

	info, err := os.Stat(cfgPath)
	if err != nil {
		log.Error("stat config failed", "error", err)
		return 1
	}
	lastMtime := info.ModTime()

	pool := session.NewPool()

	srv := metrics.NewServer(cfg.Metrics.Port, cfg.Metrics.Path, cfg.Metrics.Collectors)
	srv.SetBuildInfo(version, string(cfg.Mode))
	srv.SetLivenessInterval(loopInterval(cfg))
	pool.SetCallbacks(session.PoolCallbacks{
		OnNewSession: func(_ string) { srv.IncSessionActive() },
		OnInvalidate: func(_ string) { srv.DecSessionActive() },
		OnReauth:     func(target string) { srv.RecordSessionReauth(target) },
		OnClose:      func() { srv.ResetSessionActive() },
	})
	tracker := targetstate.New(srv)

	targets := resolvedToTargets(resolved)
	pool.SetAPIConfig(buildAPIConfigs(targets))
	srv.SetScrapeTimeout(targets)
	coll := collector.NewCollector(pool, targets, cfg.Metrics.ScrapeInterval.Duration, srv, cfg.Metrics.Collectors)
	coll.SetReachability(tracker)
	srv.SetCollectFunc(coll.Collect)

	newDomainsColl := collector.NewNewDomainsCollector(pool, targets, cfg.Metrics.NewDomainsInterval.Duration, srv, cfg.Metrics.Collectors)
	newDomainsColl.SetReachability(tracker)

	gravSched, err := gravity.NewScheduler(pool, targets, srv)
	if err != nil {
		log.Error("gravity scheduler init failed", "error", err)
		return 1
	}
	gravSched.SetReachability(tracker)

	httpLog := logging.Component("http")
	go func() {
		httpLog.Info("http server listening", "addr", fmt.Sprintf(":%d", cfg.Metrics.Port), "endpoints", []string{cfg.Metrics.Path, "/healthz"})
		if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			httpLog.Error("http server failed", "addr", fmt.Sprintf(":%d", cfg.Metrics.Port), "error", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// applyTargets propagates a reloaded target set to every runtime component.
	applyTargets := func(newTargets []config.Target) {
		pool.SetAPIConfig(buildAPIConfigs(newTargets))
		srv.SetScrapeTimeout(newTargets)
		coll.UpdateTargets(newTargets)
		newDomainsColl.UpdateTargets(newTargets)
		gravSched.UpdateTargets(newTargets)
		keep := make(map[string]bool, len(newTargets))
		for _, t := range newTargets {
			keep[t.Name] = true
		}
		tracker.Forget(keep)
	}

	// reload checks the config file and applies a changed, valid config to
	// the shared runtime. It returns the new config, or nil if unchanged or
	// invalid.
	reload := func() (*config.Config, []config.ResolvedTarget) {
		newCfg, newResolved, err := tryReloadConfig(cfgPath, &lastMtime)
		if err != nil {
			log.Error("config reload failed", "error", redact.Text(err.Error()))
			srv.RecordConfigReload(false)
			return nil, nil
		}
		if newCfg == nil {
			return nil, nil
		}
		d := config.Diff(cfg, newCfg)
		log.Info("config reloaded", "changed", d.Changed, "targets_added", d.Added, "targets_removed", d.Removed)
		srv.RecordConfigReload(true)
		if level, err := logging.ParseLevel(newCfg.LogLevel); err == nil {
			logging.SetLevel(level)
		}
		if newCfg.LogFormat != logging.Format() {
			log.Warn("log_format change requires restart", "current", logging.Format(), "configured", newCfg.LogFormat)
		}
		logConfigWarnings(newResolved)
		applyTargets(resolvedToTargets(newResolved))
		srv.SetLivenessInterval(loopInterval(newCfg))
		return newCfg, newResolved
	}

	go gravSched.Start(ctx)
	go newDomainsColl.Start(ctx)

	shutdown := func() int {
		log.Info("shutting down")
		_ = pool.Close()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		log.Info("stopped", "uptime", time.Since(started).Round(time.Second))
		return 0
	}

	interval := loopInterval(cfg)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	switch cfg.Mode {
	case config.ModeConfig:
		srv.SetConfigMetrics(cfg)

		deps := worker.Dependencies{
			Sessions:        pool,
			Settings:        reconcile.Settings{},
			Content:         reconcile.Content{},
			Gravity:         gravSched,
			Recorder:        srv,
			Reachability:    tracker,
			Interval:        interval,
			Marker:          cfg.Reconcile.Marker,
			LocalDNSPurge:   cfg.Reconcile.LocalDNSPurge,
			CNAMEPurge:      cfg.Reconcile.CNAMEPurge,
			GravityOnChange: cfg.Reconcile.GravityOnChange != nil && *cfg.Reconcile.GravityOnChange,
		}

		workers := worker.SyncWorkers(nil, resolved, deps)
		worker.ReconcileWorkers(workers, time.Now().Add(interval))
		srv.Heartbeat()

		for {
			select {
			case <-ticker.C:
				if newCfg, newResolved := reload(); newCfg != nil {
					if d := loopInterval(newCfg); d != interval {
						interval = d
						ticker.Reset(interval)
					}
					cfg = newCfg
					deps.Interval = interval
					deps.Marker = cfg.Reconcile.Marker
					deps.LocalDNSPurge = cfg.Reconcile.LocalDNSPurge
					deps.CNAMEPurge = cfg.Reconcile.CNAMEPurge
					deps.GravityOnChange = cfg.Reconcile.GravityOnChange != nil && *cfg.Reconcile.GravityOnChange
					srv.SetConfigMetrics(cfg)
					workers = worker.SyncWorkers(workers, newResolved, deps)
				}
				worker.ReconcileWorkers(workers, time.Now().Add(interval))
				srv.Heartbeat()
			case <-ctx.Done():
				return shutdown()
			}
		}

	case config.ModeSync:
		syncer := forsetisync.NewSyncer(pool, cfg, srv, tracker)
		syncer.Cycle(time.Now().Add(interval))
		srv.Heartbeat()

		for {
			select {
			case <-ticker.C:
				if newCfg, _ := reload(); newCfg != nil {
					if d := loopInterval(newCfg); d != interval {
						interval = d
						ticker.Reset(interval)
					}
					cfg = newCfg
					syncer.Update(cfg)
				}
				syncer.Cycle(time.Now().Add(interval))
				srv.Heartbeat()
			case <-ctx.Done():
				return shutdown()
			}
		}
	}

	return 0
}
