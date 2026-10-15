package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	_ "time/tzdata" // honour TZ in the scratch image

	"github.com/st0o0/forseti/internal/config"
	"github.com/st0o0/forseti/internal/logging"
	"github.com/st0o0/forseti/internal/pihole"
	"github.com/st0o0/forseti/internal/reconcile"
)

var version = "dev"

func main() {
	// Silence the stdlib logger; everything goes through slog.
	log.SetOutput(io.Discard)

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
	fmt.Fprintln(os.Stderr, "  forseti healthcheck [--config <path> | --addr <host:port>]   Liveness probe against /healthz")
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

// loadConfig sets up a bootstrap text logger on out, loads the config and
// reconfigures logging with the configured format and level. It logs config
// warnings with the final logger.
func loadConfig(path string, out io.Writer) (*config.Config, []config.ResolvedTarget, error) {
	logging.Setup("text", slog.LevelInfo, out)
	cfg, resolved, err := config.Load(path)
	if err != nil {
		logging.Component("main").Error("loading config failed", "error", err)
		return nil, nil, err
	}
	level, _ := logging.ParseLevel(cfg.LogLevel)
	logging.Setup(cfg.LogFormat, level, out)
	logConfigWarnings(resolved)
	return cfg, resolved, nil
}

func logConfigWarnings(resolved []config.ResolvedTarget) {
	log := logging.Component("config")
	for _, rt := range resolved {
		for _, w := range rt.Warnings {
			log.Warn("config warning", "target", rt.Name, "reason", w)
		}
	}
}

// One-shot commands log to stderr so stdout carries only their output.
func runPlan(args []string) int {
	cfgPath := parseConfigFlag(args, "plan")
	cfg, resolved, err := loadConfig(cfgPath, os.Stderr)
	if err != nil {
		return 1
	}
	log := logging.Component("main")

	hasChanges := false
	for _, rt := range resolved {
		client := pihole.NewClient(rt.URL, rt.Password, rt.API.TimeoutOrDefault())
		if err := client.Login(); err != nil {
			log.Error("login failed", "target", rt.Name, "error", err)
			continue
		}
		if err := client.CheckReadiness(); err != nil {
			client.Close()
			log.Error("target not ready", "target", rt.Name, "error", err)
			continue
		}
		report, err := reconcile.Plan(&rt, client, reconcile.ReconcileOptions{
			Marker:        cfg.Reconcile.Marker,
			LocalDNSPurge: cfg.Reconcile.LocalDNSPurge,
			CNAMEPurge:    cfg.Reconcile.CNAMEPurge,
		})
		if err != nil {
			client.Close()
			log.Error("plan failed", "target", rt.Name, "error", err)
			continue
		}

		settingsDiff, err := reconcile.DiffSettings(&rt.Settings, client)
		client.Close()
		if err != nil {
			log.Error("settings diff failed", "target", rt.Name, "error", err)
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
	cfg, resolved, err := loadConfig(cfgPath, os.Stderr)
	if err != nil {
		return 1
	}
	log := logging.Component("main")

	exitCode := 0
	for _, rt := range resolved {
		client := pihole.NewClient(rt.URL, rt.Password, rt.API.TimeoutOrDefault())
		if err := client.Login(); err != nil {
			log.Error("login failed", "target", rt.Name, "error", err)
			exitCode = 1
			continue
		}
		if err := client.CheckReadiness(); err != nil {
			client.Close()
			log.Error("target not ready", "target", rt.Name, "error", err)
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
			log.Error("apply failed", "target", rt.Name, "error", err)
			exitCode = 1
			continue
		}
		printDiffReport(&report.Diff)
		if len(report.Errors) > 0 {
			for _, e := range report.Errors {
				log.Warn("write failed", "target", rt.Name, "error", e)
			}
			exitCode = 1
		}
	}
	return exitCode
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
		fmt.Printf("    %-30s  %v → %v\n", c.Name, c.DisplayCurrent(), c.DisplayDesired())
	}
}

func hasDiff(r *reconcile.DiffReport) bool {
	return r.Groups.HasChanges() || r.Adlists.HasChanges() ||
		r.Deny.HasChanges() || r.Allow.HasChanges() ||
		r.LocalDNS.HasChanges() || r.CNAME.HasChanges() || r.Clients.HasChanges()
}
