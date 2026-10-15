package worker

import (
	"log/slog"
	"sort"
	"time"

	"github.com/st0o0/forseti/internal/config"
	"github.com/st0o0/forseti/internal/logging"
	"github.com/st0o0/forseti/internal/run"
)

func SyncWorkers(workers map[string]*TargetWorker, resolved []config.ResolvedTarget, deps Dependencies) map[string]*TargetWorker {
	if workers == nil {
		workers = make(map[string]*TargetWorker)
	}
	log := deps.Log
	if log == nil {
		log = logging.Component("worker")
	}

	seen := make(map[string]bool)
	for _, rt := range resolved {
		seen[rt.Name] = true
		if w, ok := workers[rt.Name]; ok {
			w.UpdateConfig(rt)
			w.UpdateDependencies(deps)
		} else {
			log.Debug("worker added", "target", rt.Name)
			workers[rt.Name] = NewTargetWorker(rt, deps)
		}
	}

	for name := range workers {
		if !seen[name] {
			log.Debug("worker removed", "target", name)
			delete(workers, name)
		}
	}

	return workers
}

// Sorted returns the workers ordered by target name.
func Sorted(workers map[string]*TargetWorker) []*TargetWorker {
	out := make([]*TargetWorker, 0, len(workers))
	for _, w := range workers {
		out = append(out, w)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out
}

// RunWorkers runs each worker (or skips it while in backoff) and accumulates
// the cycle summary.
func RunWorkers(cycle *run.Cycle, workers []*TargetWorker, runnerFor func(*TargetWorker) Runner) []run.Result {
	results := make([]run.Result, 0, len(workers))
	now := time.Now()
	for _, w := range workers {
		var res run.Result
		if w.ShouldSkip(now) {
			res = w.Skip(run.StageBackoff)
		} else {
			res = w.Run(runnerFor(w))
		}
		cycle.Add(res)
		results = append(results, res)
	}
	return results
}

// LogCycle logs the cycle summary ("cycle complete") with the next run time.
func LogCycle(log *slog.Logger, mode string, c run.Cycle, nextRun time.Time) {
	log.Info("cycle complete",
		"mode", mode,
		"targets", c.Targets,
		"ok", c.OK,
		"partial", c.Partial,
		"failed", c.Failed,
		"skipped", c.Skipped,
		"changes", c.Changes,
		"duration", c.Duration.Round(time.Millisecond),
		"next_run", nextRun.Format(time.RFC3339),
	)
}

// ReconcileWorkers runs one config-mode cycle over all workers and logs the
// summary. nextRun is the time of the following cycle.
func ReconcileWorkers(workers map[string]*TargetWorker, nextRun time.Time) run.Cycle {
	start := time.Now()
	var cycle run.Cycle
	RunWorkers(&cycle, Sorted(workers), func(w *TargetWorker) Runner { return &ConfigRunner{w: w} })
	cycle.Duration = time.Since(start)
	LogCycle(logging.Component("worker"), string(config.ModeConfig), cycle, nextRun)
	return cycle
}
