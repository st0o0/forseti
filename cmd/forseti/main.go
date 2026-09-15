package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/st0o0/forseti/internal/collector"
	"github.com/st0o0/forseti/internal/config"
	"github.com/st0o0/forseti/internal/gravity"
	"github.com/st0o0/forseti/internal/metrics"
	"github.com/st0o0/forseti/internal/pihole"
	"github.com/st0o0/forseti/internal/reconcile"
	"github.com/st0o0/forseti/internal/session"
	forsetisync "github.com/st0o0/forseti/internal/sync"
	"github.com/st0o0/forseti/internal/worker"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "plan":
		os.Exit(runPlan(os.Args[2:]))
	case "apply":
		os.Exit(runApply(os.Args[2:]))
	case "watch":
		os.Exit(runWatch(os.Args[2:]))
	case "healthcheck":
		os.Exit(runHealthcheck(os.Args[2:]))
	case "version":
		fmt.Printf("forseti %s\n", version)
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "forseti %s -- declarative GitOps controller for Pi-hole v6\n\n", version)
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  forseti plan    --config <path>   Show what would change (dry-run)")
	fmt.Fprintln(os.Stderr, "  forseti apply   --config <path>   Reconcile desired state")
	fmt.Fprintln(os.Stderr, "  forseti watch   --config <path>   Daemon mode with metrics server")
	fmt.Fprintln(os.Stderr, "  forseti version                   Print version")
	fmt.Fprintln(os.Stderr, "  forseti healthcheck --config <path>   Liveness probe (checks all targets)")
}

func parseConfigFlag(args []string, name string) string {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	configPath := fs.String("config", "", "path to forseti config file")
	_ = fs.Parse(args)
	if *configPath == "" {
		fmt.Fprintf(os.Stderr, "error: --config is required\n")
		os.Exit(1)
	}
	return *configPath
}

func runPlan(args []string) int {
	cfgPath := parseConfigFlag(args, "plan")
	cfg, resolved, err := config.Load(cfgPath)
	if err != nil {
		slog.Error("loading config failed", "error", err)
		return 1
	}

	setupLogger(cfg.LogLevel, cfg.LogFormat)

	hasChanges := false
	for _, rt := range resolved {
		client := pihole.NewClient(rt.URL, rt.Password, rt.API.TimeoutOrDefault())
		if err := client.Login(); err != nil {
			slog.Error("login failed", "target", rt.Name, "error", err)
			continue
		}
		if err := client.CheckReadiness(); err != nil {
			client.Close()
			slog.Error("target not ready", "target", rt.Name, "error", err)
			continue
		}
		report, err := reconcile.Plan(&rt, client, reconcile.ReconcileOptions{
			Marker:        cfg.Reconcile.Marker,
			LocalDNSPurge: cfg.Reconcile.LocalDNSPurge,
			CNAMEPurge:    cfg.Reconcile.CNAMEPurge,
		})
		if err != nil {
			client.Close()
			slog.Error("plan failed", "target", rt.Name, "error", err)
			continue
		}

		settingsDiff, err := reconcile.DiffSettings(&rt.Settings, client)
		client.Close()
		if err != nil {
			slog.Error("settings diff failed", "target", rt.Name, "error", err)
		}

		printDiffReport(report)
		if settingsDiff != nil && settingsDiff.HasChanges() {
			printSettingsDiff(rt.Name, settingsDiff)
			hasChanges = true
		}
		if hasDiff(report) {
			hasChanges = true
		}
	}

	if !hasChanges {
		fmt.Println("\nAll targets in sync.")
	}
	return 0
}

func runApply(args []string) int {
	cfgPath := parseConfigFlag(args, "apply")
	cfg, resolved, err := config.Load(cfgPath)
	if err != nil {
		slog.Error("loading config failed", "error", err)
		return 1
	}

	setupLogger(cfg.LogLevel, cfg.LogFormat)

	exitCode := 0
	for _, rt := range resolved {
		client := pihole.NewClient(rt.URL, rt.Password, rt.API.TimeoutOrDefault())
		if err := client.Login(); err != nil {
			slog.Error("login failed", "target", rt.Name, "error", err)
			exitCode = 1
			continue
		}
		if err := client.CheckReadiness(); err != nil {
			client.Close()
			slog.Error("target not ready", "target", rt.Name, "error", err)
			exitCode = 1
			continue
		}
		report, err := reconcile.Apply(&rt, client, reconcile.ReconcileOptions{
			Marker:        cfg.Reconcile.Marker,
			LocalDNSPurge: cfg.Reconcile.LocalDNSPurge,
			CNAMEPurge:    cfg.Reconcile.CNAMEPurge,
		})
		client.Close()
		if err != nil {
			slog.Error("apply failed", "target", rt.Name, "error", err)
			exitCode = 1
			continue
		}
		printDiffReport(&report.Diff)
		if len(report.Errors) > 0 {
			for _, e := range report.Errors {
				slog.Warn("reconcile warning", "target", rt.Name, "error", e)
			}
			exitCode = 1
		}
	}
	return exitCode
}

func runHealthcheck(args []string) int {
	cfgPath := parseConfigFlag(args, "healthcheck")
	_, resolved, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: reading config: %v\n", err)
		return 1
	}

	exitCode := 0
	for _, target := range resolved {
		client := pihole.NewClient(target.URL, target.Password, target.API.TimeoutOrDefault())

		done := make(chan error, 1)
		go func() {
			done <- client.Login()
		}()

		timer := time.NewTimer(5 * time.Second)
		select {
		case err := <-done:
			timer.Stop()
			if err != nil {
				fmt.Printf("[%s] error: %v\n", target.Name, err)
				exitCode = 1
			} else {
				fmt.Printf("[%s] ok\n", target.Name)
				client.Close()
			}
		case <-timer.C:
			fmt.Printf("[%s] error: timeout after 5s\n", target.Name)
			exitCode = 1
		}
	}
	return exitCode
}

func setupLogger(level, format string) {
	var slogLevel slog.Level
	switch level {
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: slogLevel}
	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	logger := slog.New(handler)
	slog.SetDefault(logger)
	log.SetOutput(io.Discard)
}

func tryReloadConfig(cfgPath string, lastMtime *time.Time) (*config.Config, []config.ResolvedTarget, error) {
	info, err := os.Stat(cfgPath)
	if err != nil {
		return nil, nil, fmt.Errorf("stat config: %w", err)
	}
	if !info.ModTime().After(*lastMtime) {
		return nil, nil, nil
	}
	newCfg, resolved, err := config.Load(cfgPath)
	if err != nil {
		return nil, nil, err
	}
	*lastMtime = info.ModTime()
	return newCfg, resolved, nil
}

func runWatch(args []string) int {
	cfgPath := parseConfigFlag(args, "watch")
	cfg, resolved, err := config.Load(cfgPath)
	if err != nil {
		slog.Error("loading config failed", "error", err)
		return 1
	}

	setupLogger(cfg.LogLevel, cfg.LogFormat)

	info, err := os.Stat(cfgPath)
	if err != nil {
		slog.Error("stat config failed", "error", err)
		return 1
	}
	lastMtime := info.ModTime()

	pool := session.NewPool()

	srv := metrics.NewServer(cfg.Metrics.Port, cfg.Metrics.Path, cfg.Metrics.Collectors)
	srv.SetBuildInfo(version, string(cfg.Mode))
	pool.SetCallbacks(session.PoolCallbacks{
		OnNewSession: func(_ string) { srv.IncSessionActive() },
		OnInvalidate: func(_ string) { srv.DecSessionActive() },
		OnReauth:     func(target string) { srv.RecordSessionReauth(target) },
		OnClose:      func() { srv.ResetSessionActive() },
	})

	targets := resolvedToTargets(resolved)
	pool.SetAPIConfig(buildAPIConfigs(targets))
	srv.SetScrapeTimeout(targets)
	coll := collector.NewCollector(pool, targets, cfg.Metrics.ScrapeInterval.Duration, srv, cfg.Metrics.Collectors)
	srv.SetCollectFunc(coll.Collect)

	gravSched, err := gravity.NewScheduler(pool, targets, srv)
	if err != nil {
		slog.Error("gravity scheduler init failed", "error", err)
		return 1
	}

	go func() {
		slog.Info("metrics server listening", "port", cfg.Metrics.Port, "path", cfg.Metrics.Path)
		if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("metrics server error", "error", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go gravSched.Start(ctx)

	slog.Info("starting", "mode", cfg.Mode)

	switch cfg.Mode {
	case config.ModeConfig:
		srv.SetConfigMetrics(cfg)

		gravityOnChange := cfg.Reconcile.GravityOnChange != nil && *cfg.Reconcile.GravityOnChange
		deps := worker.Dependencies{
			Sessions:        pool,
			Settings:        reconcile.Settings{},
			Content:         reconcile.Content{},
			Gravity:         gravSched,
			Recorder:        srv,
			Interval:        cfg.Reconcile.Interval.Duration,
			Marker:          cfg.Reconcile.Marker,
			LocalDNSPurge:   cfg.Reconcile.LocalDNSPurge,
			CNAMEPurge:      cfg.Reconcile.CNAMEPurge,
			GravityOnChange: gravityOnChange,
		}

		workers := worker.SyncWorkers(nil, resolved, deps)
		worker.ReconcileWorkers(workers)

		ticker := time.NewTicker(cfg.Reconcile.Interval.Duration)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if newCfg, newResolved, err := tryReloadConfig(cfgPath, &lastMtime); err != nil {
					slog.Error("config reload failed", "error", err)
					srv.RecordConfigReload(false)
				} else if newCfg != nil {
					slog.Info("config reloaded successfully")
					srv.RecordConfigReload(true)
					setupLogger(newCfg.LogLevel, newCfg.LogFormat)
					if newCfg.Reconcile.Interval.Duration != cfg.Reconcile.Interval.Duration {
						ticker.Reset(newCfg.Reconcile.Interval.Duration)
						deps.Interval = newCfg.Reconcile.Interval.Duration
						slog.Info("reconcile interval updated", "interval", newCfg.Reconcile.Interval.Duration)
					}
					cfg = newCfg
					resolved = newResolved
					deps.Marker = cfg.Reconcile.Marker
					deps.LocalDNSPurge = cfg.Reconcile.LocalDNSPurge
					deps.CNAMEPurge = cfg.Reconcile.CNAMEPurge
					deps.GravityOnChange = cfg.Reconcile.GravityOnChange != nil && *cfg.Reconcile.GravityOnChange
					srv.SetConfigMetrics(cfg)
					newTargets := resolvedToTargets(resolved)
					pool.SetAPIConfig(buildAPIConfigs(newTargets))
					srv.SetScrapeTimeout(newTargets)
					coll.UpdateTargets(newTargets)
					workers = worker.SyncWorkers(workers, resolved, deps)
				}
				worker.ReconcileWorkers(workers)
			case <-ctx.Done():
				slog.Info("shutting down")
				pool.Close()
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = srv.Shutdown(shutdownCtx)
				return 0
			}
		}

	case config.ModeSync:
		syncer := forsetisync.NewSyncer(pool, cfg, srv)
		syncer.SyncAll()

		ticker := time.NewTicker(cfg.Sync.Interval.Duration)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if newCfg, _, err := tryReloadConfig(cfgPath, &lastMtime); err != nil {
					slog.Error("config reload failed", "error", err)
					srv.RecordConfigReload(false)
				} else if newCfg != nil {
					slog.Info("config reloaded successfully")
					srv.RecordConfigReload(true)
					setupLogger(newCfg.LogLevel, newCfg.LogFormat)
					if newCfg.Sync.Interval.Duration != cfg.Sync.Interval.Duration {
						ticker.Reset(newCfg.Sync.Interval.Duration)
						slog.Info("sync interval updated", "interval", newCfg.Sync.Interval.Duration)
					}
					cfg = newCfg
					syncer = forsetisync.NewSyncer(pool, cfg, srv)
				}
				syncer.SyncAll()
			case <-ctx.Done():
				slog.Info("shutting down")
				pool.Close()
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = srv.Shutdown(shutdownCtx)
				return 0
			}
		}
	}

	return 0
}

func buildAPIConfigs(targets []config.Target) map[string]config.APIConfig {
	m := make(map[string]config.APIConfig, len(targets))
	for _, t := range targets {
		m[t.Name] = t.API
	}
	return m
}

func resolvedToTargets(resolved []config.ResolvedTarget) []config.Target {
	targets := make([]config.Target, len(resolved))
	for i, rt := range resolved {
		targets[i] = rt.Target
	}
	return targets
}

func printDiffReport(r *reconcile.DiffReport) {
	fmt.Printf("\n--- %s ---\n", r.Target)
	printResourceLine("groups", r.Groups)
	printResourceLine("adlists", r.Adlists)
	printResourceLine("deny", r.Deny)
	printResourceLine("allow", r.Allow)
	printResourceLine("local_dns", r.LocalDNS)
	printResourceLine("cname", r.CNAME)
	printResourceLine("clients", r.Clients)
	if r.NeedsGravity {
		fmt.Println("  gravity update required")
	}
}

func printResourceLine(name string, d reconcile.ResourceDiff) {
	if !d.HasChanges() && d.Unchanged == 0 {
		return
	}
	fmt.Printf("  %-10s  +%d  ~%d  -%d  =%d\n", name, len(d.Adds), len(d.Updates), len(d.Deletes), d.Unchanged)
}

func printSettingsDiff(target string, diff *reconcile.SettingsDiff) {
	fmt.Printf("  settings:\n")
	for _, c := range diff.Changes {
		fmt.Printf("    %-30s  %v → %v\n", c.Name, c.Current, c.Desired)
	}
}

func hasDiff(r *reconcile.DiffReport) bool {
	return r.Groups.HasChanges() || r.Adlists.HasChanges() ||
		r.Deny.HasChanges() || r.Allow.HasChanges() ||
		r.LocalDNS.HasChanges() || r.CNAME.HasChanges() || r.Clients.HasChanges()
}
