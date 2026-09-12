package worker

import (
	"log/slog"
	"time"

	"github.com/st0o0/forseti/internal/config"
)

func SyncWorkers(workers map[string]*TargetWorker, resolved []config.ResolvedTarget, deps Dependencies) map[string]*TargetWorker {
	if workers == nil {
		workers = make(map[string]*TargetWorker)
	}

	seen := make(map[string]bool)
	for _, rt := range resolved {
		seen[rt.Name] = true
		if w, ok := workers[rt.Name]; ok {
			w.UpdateConfig(rt)
		} else {
			slog.Info("adding worker for target", "target", rt.Name)
			workers[rt.Name] = NewTargetWorker(rt, deps)
		}
	}

	for name := range workers {
		if !seen[name] {
			slog.Info("removing worker for target", "target", name)
			delete(workers, name)
		}
	}

	return workers
}

func ReconcileWorkers(workers map[string]*TargetWorker) {
	start := time.Now()
	now := start

	var changed, skipped, failed, total int
	for _, w := range workers {
		total++
		if w.ShouldSkip(now) {
			h := w.Health()
			slog.Debug("skipping target (backoff)", "target", w.Name(), "health", h.State, "until", h.BackoffUntil.Format(time.RFC3339))
			skipped++
			continue
		}
		if err := w.Reconcile(); err != nil {
			failed++
		} else {
			changed++
		}
	}

	slog.Info("cycle complete",
		"targets", total,
		"ok", changed,
		"failed", failed,
		"skipped", skipped,
		"duration", time.Since(start).Round(time.Millisecond),
	)
}
