package sync

import (
	"log/slog"
	"time"

	"github.com/st0o0/forseti/internal/config"
	"github.com/st0o0/forseti/internal/logging"
	"github.com/st0o0/forseti/internal/run"
	"github.com/st0o0/forseti/internal/targetstate"
	"github.com/st0o0/forseti/internal/worker"
)

const syncMarker = "[forseti-sync]"

// Syncer drives sync cycles: one primary snapshot run, then one run per
// replica, each through a worker.TargetWorker with its own health and backoff.
type Syncer struct {
	cfg      *config.Config
	deps     worker.Dependencies
	log      *slog.Logger
	primary  *worker.TargetWorker
	replicas map[string]*worker.TargetWorker
}

// NewSyncer builds the primary and replica workers for cfg. reach receives
// reachability observations; if nil, a tracker updating rec is created.
func NewSyncer(sessions worker.SessionManager, cfg *config.Config, rec worker.Recorder, reach worker.Reachability) *Syncer {
	log := logging.Component("sync")
	if reach == nil {
		var gauge targetstate.Reachability
		if g, ok := rec.(targetstate.Reachability); ok {
			gauge = g
		}
		reach = targetstate.New(gauge)
	}
	s := &Syncer{
		log: log,
		deps: worker.Dependencies{
			Sessions:     sessions,
			Recorder:     rec,
			Reachability: reach,
			Marker:       syncMarker,
			Log:          log,
		},
	}
	s.Update(cfg)
	return s
}

// Update applies a (reloaded) config, keeping worker state for targets that
// still exist.
func (s *Syncer) Update(cfg *config.Config) {
	s.cfg = cfg
	s.deps.Interval = cfg.Sync.Interval.Duration

	var replicas []config.ResolvedTarget
	for _, t := range cfg.Targets {
		rt := config.ResolvedTarget{Target: t}
		switch {
		case t.Name == cfg.Sync.Primary:
			if s.primary == nil || s.primary.Name() != t.Name {
				s.primary = worker.NewTargetWorker(rt, s.deps)
			} else {
				s.primary.UpdateConfig(rt)
			}
		case t.Role == "replica":
			replicas = append(replicas, rt)
		}
	}
	s.replicas = worker.SyncWorkers(s.replicas, replicas, s.deps)
}

func (s *Syncer) resources() map[string]bool {
	m := make(map[string]bool, len(s.cfg.Sync.Resources))
	for _, r := range s.cfg.Sync.Resources {
		m[r] = true
	}
	return m
}

// Cycle runs one sync cycle, logs "cycle complete" and returns the summary
// and the individual results (primary first). nextRun is reported in the log.
func (s *Syncer) Cycle(nextRun time.Time) (run.Cycle, []run.Result) {
	start := time.Now()
	var cycle run.Cycle
	resources := s.resources()

	snap := &SnapshotRunner{resources: resources}
	var primaryRes run.Result
	if s.primary == nil {
		s.log.Error("primary target not configured", "primary", s.cfg.Sync.Primary)
		return cycle, nil
	}
	if s.primary.ShouldSkip(start) {
		primaryRes = s.primary.Skip(run.StageBackoff)
	} else {
		primaryRes = s.primary.Run(snap)
	}
	cycle.Add(primaryRes)
	results := []run.Result{primaryRes}

	replicas := worker.Sorted(s.replicas)
	if primaryRes.Status != run.OK {
		for _, w := range replicas {
			res := w.Skip(run.StagePrimary)
			cycle.Add(res)
			results = append(results, res)
		}
	} else {
		results = append(results, worker.RunWorkers(&cycle, replicas, func(*worker.TargetWorker) worker.Runner {
			return &ReplicaRunner{
				primary:   snap.state,
				resources: resources,
				marker:    syncMarker,
				purgeDNS:  s.cfg.Sync.PurgeLocalDNS(),
				log:       s.log,
			}
		})...)
	}

	cycle.Duration = time.Since(start)
	worker.LogCycle(s.log, string(config.ModeSync), cycle, nextRun)
	return cycle, results
}

// SyncAll runs one cycle with the configured interval as next run.
func (s *Syncer) SyncAll() {
	s.Cycle(time.Now().Add(s.cfg.Sync.Interval.Duration))
}
