package main

import (
	"context"
	"errors"
	"flag"
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
	"github.com/st0o0/forseti/internal/metrics"
	"github.com/st0o0/forseti/internal/pihole"
	"github.com/st0o0/forseti/internal/reconcile"
	"github.com/st0o0/forseti/internal/session"
	forsetisync "github.com/st0o0/forseti/internal/sync"
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
	fs.Parse(args)
	if *configPath == "" {
		fmt.Fprintf(os.Stderr, "error: --config is required\n")
		os.Exit(1)
	}
	return *configPath
}

func runPlan(args []string) int {
	cfgPath := parseConfigFlag(args, "plan")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	hasChanges := false
	for _, target := range cfg.Targets {
		client := pihole.NewClient(target.URL, target.Password)
		if err := client.Login(); err != nil {
			fmt.Fprintf(os.Stderr, "[%s] login error: %v\n", target.Name, err)
			continue
		}
		report, err := reconcile.Plan(cfg, target, client, cfg.Reconcile.Marker)
		client.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "[%s] error: %v\n", target.Name, err)
			continue
		}
		printDiffReport(report)
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
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	exitCode := 0
	for _, target := range cfg.Targets {
		client := pihole.NewClient(target.URL, target.Password)
		if err := client.Login(); err != nil {
			fmt.Fprintf(os.Stderr, "[%s] login error: %v\n", target.Name, err)
			exitCode = 1
			continue
		}
		report, err := reconcile.Apply(cfg, target, client, cfg.Reconcile.Marker)
		client.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "[%s] error: %v\n", target.Name, err)
			exitCode = 1
			continue
		}
		printDiffReport(&report.Diff)
		if len(report.Errors) > 0 {
			for _, e := range report.Errors {
				fmt.Fprintf(os.Stderr, "[%s] warning: %v\n", target.Name, e)
			}
			exitCode = 1
		}
	}
	return exitCode
}

func runHealthcheck(args []string) int {
	cfgPath := parseConfigFlag(args, "healthcheck")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	exitCode := 0
	for _, target := range cfg.Targets {
		client := pihole.NewClient(target.URL, target.Password)

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
	slog.SetDefault(slog.New(handler))
}

func tryReloadConfig(cfgPath string, lastMtime *time.Time) (*config.Config, error) {
	info, err := os.Stat(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("stat config: %w", err)
	}
	if !info.ModTime().After(*lastMtime) {
		return nil, nil
	}
	newCfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, err
	}
	*lastMtime = info.ModTime()
	return newCfg, nil
}

func runWatch(args []string) int {
	cfgPath := parseConfigFlag(args, "watch")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	setupLogger(cfg.LogLevel, cfg.LogFormat)

	info, err := os.Stat(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	lastMtime := info.ModTime()

	pool := session.NewPool()

	srv := metrics.NewServer(cfg.Metrics.Port, cfg.Metrics.Path)
	srv.SetBuildInfo(version, string(cfg.Mode))
	pool.SetCallbacks(session.PoolCallbacks{
		OnNewSession: func(_ string) { srv.IncSessionActive() },
		OnReauth:     func(target string) { srv.RecordSessionReauth(target) },
		OnClose:      func() { srv.ResetSessionActive() },
	})

	coll := collector.NewCollector(pool, cfg.Targets, cfg.Metrics.ScrapeInterval.Duration, srv)
	srv.SetCollectFunc(coll.Collect)

	gravSched, err := gravity.NewScheduler(pool, cfg.Targets, srv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
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
		reconcileAll(cfg, srv, pool, gravSched)

		ticker := time.NewTicker(cfg.Reconcile.Interval.Duration)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if newCfg, err := tryReloadConfig(cfgPath, &lastMtime); err != nil {
					slog.Error("config reload failed", "error", err)
					srv.RecordConfigReload(false)
				} else if newCfg != nil {
					slog.Info("config reloaded successfully")
					srv.RecordConfigReload(true)
					setupLogger(newCfg.LogLevel, newCfg.LogFormat)
					if newCfg.Reconcile.Interval.Duration != cfg.Reconcile.Interval.Duration {
						ticker.Reset(newCfg.Reconcile.Interval.Duration)
						slog.Info("reconcile interval updated", "interval", newCfg.Reconcile.Interval.Duration)
					}
					cfg = newCfg
					srv.SetConfigMetrics(cfg)
				}
				reconcileAll(cfg, srv, pool, gravSched)
			case <-ctx.Done():
				slog.Info("shutting down")
				pool.Close()
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				srv.Shutdown(shutdownCtx)
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
				if newCfg, err := tryReloadConfig(cfgPath, &lastMtime); err != nil {
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
				srv.Shutdown(shutdownCtx)
				return 0
			}
		}
	}

	return 0
}

func reconcileAll(cfg *config.Config, srv *metrics.Server, pool *session.Pool, gravSched *gravity.Scheduler) {
	for _, target := range cfg.Targets {
		start := time.Now()
		client, err := pool.Get(target)
		if err != nil {
			slog.Error("session error", "target", target.Name, "error", err)
			srv.MarkTargetUnreachable(target.Name)
			continue
		}

		report, err := reconcile.Apply(cfg, target, client, cfg.Reconcile.Marker)
		duration := time.Since(start)

		if err != nil {
			slog.Error("reconcile error", "target", target.Name, "error", err)
			srv.RecordReconcile(metrics.ReconcileResult{
				Target:   target.Name,
				Duration: duration,
				Success:  false,
			})
			srv.MarkTargetUnreachable(target.Name)
			continue
		}

		changes := buildChangesMap(&report.Diff)
		drift := buildDriftMap(&report.Diff)

		srv.RecordReconcile(metrics.ReconcileResult{
			Target:   target.Name,
			Duration: duration,
			Success:  len(report.Errors) == 0,
			Changes:  changes,
			Drift:    drift,
		})

		for _, e := range report.Errors {
			slog.Warn("reconcile warning", "target", target.Name, "error", e)
		}

		if report.Diff.NeedsGravity && cfg.Reconcile.GravityOnChange != nil && *cfg.Reconcile.GravityOnChange {
			if err := gravSched.TriggerNow(target.Name, gravity.ReasonAdlistChange); err != nil {
				slog.Error("gravity trigger error", "target", target.Name, "error", err)
			}
		}

		if hasDiff(&report.Diff) {
			slog.Info("reconciled", "target", target.Name, "duration", duration.Round(time.Millisecond))
		}
	}
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

func hasDiff(r *reconcile.DiffReport) bool {
	return r.Groups.HasChanges() || r.Adlists.HasChanges() ||
		r.Deny.HasChanges() || r.Allow.HasChanges() ||
		r.LocalDNS.HasChanges() || r.CNAME.HasChanges() || r.Clients.HasChanges()
}

func buildChangesMap(r *reconcile.DiffReport) map[string]map[string]int {
	m := make(map[string]map[string]int)
	addResourceChanges(m, "group", r.Groups)
	addResourceChanges(m, "adlist", r.Adlists)
	addResourceChanges(m, "deny", r.Deny)
	addResourceChanges(m, "allow", r.Allow)
	addResourceChanges(m, "dns", r.LocalDNS)
	addResourceChanges(m, "cname", r.CNAME)
	addResourceChanges(m, "client", r.Clients)
	return m
}

func addResourceChanges(m map[string]map[string]int, name string, d reconcile.ResourceDiff) {
	if !d.HasChanges() {
		return
	}
	m[name] = map[string]int{
		"add":    len(d.Adds),
		"update": len(d.Updates),
		"delete": len(d.Deletes),
	}
}

func buildDriftMap(r *reconcile.DiffReport) map[string]int {
	m := make(map[string]int)
	m["group"] = len(r.Groups.Adds) + len(r.Groups.Deletes) + len(r.Groups.Updates)
	m["adlist"] = len(r.Adlists.Adds) + len(r.Adlists.Deletes) + len(r.Adlists.Updates)
	m["deny"] = len(r.Deny.Adds) + len(r.Deny.Deletes) + len(r.Deny.Updates)
	m["allow"] = len(r.Allow.Adds) + len(r.Allow.Deletes) + len(r.Allow.Updates)
	m["dns"] = len(r.LocalDNS.Adds) + len(r.LocalDNS.Deletes) + len(r.LocalDNS.Updates)
	m["cname"] = len(r.CNAME.Adds) + len(r.CNAME.Deletes) + len(r.CNAME.Updates)
	m["client"] = len(r.Clients.Adds) + len(r.Clients.Deletes) + len(r.Clients.Updates)
	return m
}
