package sync

import (
	"testing"
	"time"

	"github.com/st0o0/forseti/internal/config"
	"github.com/st0o0/forseti/internal/metrics"
	"github.com/st0o0/forseti/internal/pihole"
	"github.com/st0o0/forseti/internal/session"
)

// runsTotal returns forseti_reconcile_runs_total for the given target/status.
func runsTotal(t *testing.T, m *metrics.Server, target, status string) float64 {
	t.Helper()
	return labelledValue(t, m, "forseti_reconcile_runs_total", map[string]string{"target": target, "status": status})
}

func labelledValue(t *testing.T, m *metrics.Server, name string, labels map[string]string) float64 {
	t.Helper()
	families, err := m.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	for _, f := range families {
		if f.GetName() != name {
			continue
		}
	metricLoop:
		for _, metric := range f.GetMetric() {
			got := map[string]string{}
			for _, lp := range metric.GetLabel() {
				got[lp.GetName()] = lp.GetValue()
			}
			for k, v := range labels {
				if got[k] != v {
					continue metricLoop
				}
			}
			if c := metric.GetCounter(); c != nil {
				return c.GetValue()
			}
			return metric.GetGauge().GetValue()
		}
	}
	return 0
}

func resourceSet(rs ...string) map[string]bool {
	m := make(map[string]bool, len(rs))
	for _, r := range rs {
		m[r] = true
	}
	return m
}

func syncOnce(t *testing.T, pool *session.Pool, cfg *config.Config, m *metrics.Server, resources []string) SyncReport {
	t.Helper()
	cfg.Sync.Resources = resources
	s := NewSyncer(pool, cfg, m)
	set := resourceSet(resources...)
	primary, ok := s.loadPrimary(set)
	if !ok {
		t.Fatal("primary load failed")
	}
	return s.syncReplica(cfg.Targets[1], primary, set)
}

func TestReportSuccessWithoutChanges(t *testing.T) {
	data := testData{groups: []pihole.APIGroup{{ID: 1, Name: "default"}}}
	primarySrv := newFullTestServer(data, &testCounters{})
	defer primarySrv.Close()
	replicaSrv := newFullTestServer(data, &testCounters{})
	defer replicaSrv.Close()

	pool := session.NewPool()
	defer pool.Close()
	cfg := makeSyncConfig(primarySrv.URL, replicaSrv.URL, nil)
	m := metrics.NewServer(0, "/metrics", config.CollectorToggles{})

	rep := syncOnce(t, pool, cfg, m, []string{"groups"})

	if rep.Status != metrics.RunSuccess {
		t.Errorf("status = %q, want success", rep.Status)
	}
	if got := runsTotal(t, m, "replica", "success"); got != 1 {
		t.Errorf("runs{success} = %v, want 1", got)
	}
	if got := labelledValue(t, m, "forseti_target_reachable", map[string]string{"target": "replica"}); got != 1 {
		t.Errorf("reachable = %v, want 1", got)
	}
}

func TestReportPartialOnItemError(t *testing.T) {
	primarySrv := newFullTestServer(testData{
		adlists: []pihole.APIList{{ID: 1, Address: "https://a.example/list.txt", Enabled: true}},
	}, &testCounters{})
	defer primarySrv.Close()
	replicaSrv := newCreateFailServer(testData{})
	defer replicaSrv.Close()

	pool := session.NewPool()
	defer pool.Close()
	cfg := makeSyncConfig(primarySrv.URL, replicaSrv.URL, nil)
	m := metrics.NewServer(0, "/metrics", config.CollectorToggles{})

	rep := syncOnce(t, pool, cfg, m, []string{"adlists"})

	if rep.Status != metrics.RunPartial {
		t.Errorf("status = %q, want partial", rep.Status)
	}
	if len(rep.Errors) != 1 {
		t.Errorf("errors = %d, want 1", len(rep.Errors))
	}
	if got := runsTotal(t, m, "replica", "partial"); got != 1 {
		t.Errorf("runs{partial} = %v, want 1", got)
	}
	if got := runsTotal(t, m, "replica", "success"); got != 0 {
		t.Errorf("runs{success} = %v, want 0", got)
	}
	if got := labelledValue(t, m, "forseti_reconcile_changes_total", map[string]string{"target": "replica", "type": "adlists", "action": "add"}); got != 0 {
		t.Errorf("failed write counted as change: %v", got)
	}
}

func TestReportErrorOnReplicaLoadFailure(t *testing.T) {
	primarySrv := newFullTestServer(testData{groups: []pihole.APIGroup{{ID: 1, Name: "default"}}}, &testCounters{})
	defer primarySrv.Close()
	replicaSrv := newErrorServer("groups")
	defer replicaSrv.Close()

	pool := session.NewPool()
	defer pool.Close()
	invalidated := 0
	pool.SetCallbacks(session.PoolCallbacks{OnInvalidate: func(name string) {
		if name == "replica" {
			invalidated++
		}
	}})
	cfg := makeSyncConfig(primarySrv.URL, replicaSrv.URL, nil)
	m := metrics.NewServer(0, "/metrics", config.CollectorToggles{})

	rep := syncOnce(t, pool, cfg, m, []string{"groups"})

	if rep.Status != metrics.RunError {
		t.Errorf("status = %q, want error", rep.Status)
	}
	if got := runsTotal(t, m, "replica", "error"); got != 1 {
		t.Errorf("runs{error} = %v, want 1", got)
	}
	if invalidated != 1 {
		t.Errorf("replica session invalidations = %d, want 1", invalidated)
	}
}

func TestReportErrorOnReplicaSessionFailure(t *testing.T) {
	primarySrv := newFullTestServer(testData{}, &testCounters{})
	defer primarySrv.Close()
	replicaSrv := newUnreachableServer()
	defer replicaSrv.Close()

	pool := session.NewPool()
	defer pool.Close()
	cfg := makeSyncConfig(primarySrv.URL, replicaSrv.URL, nil)
	m := metrics.NewServer(0, "/metrics", config.CollectorToggles{})

	rep := syncOnce(t, pool, cfg, m, []string{"groups"})

	if rep.Status != metrics.RunError {
		t.Errorf("status = %q, want error", rep.Status)
	}
	if got := runsTotal(t, m, "replica", "error"); got != 1 {
		t.Errorf("runs{error} = %v, want 1", got)
	}
	if got := labelledValue(t, m, "forseti_target_reachable", map[string]string{"target": "replica"}); got != 0 {
		t.Errorf("reachable = %v, want 0", got)
	}
}

func TestReportChangesPerResource(t *testing.T) {
	primarySrv := newFullTestServer(testData{
		clients: []pihole.APIClient{{ID: 1, Client: "10.0.0.1"}, {ID: 2, Client: "10.0.0.2"}},
	}, &testCounters{})
	defer primarySrv.Close()
	replicaSrv := newFullTestServer(testData{
		deny: []pihole.APIDomain{{ID: 7, Domain: "old.example", Comment: "[forseti-sync]"}},
	}, &testCounters{})
	defer replicaSrv.Close()

	pool := session.NewPool()
	defer pool.Close()
	cfg := makeSyncConfig(primarySrv.URL, replicaSrv.URL, nil)
	m := metrics.NewServer(0, "/metrics", config.CollectorToggles{})

	rep := syncOnce(t, pool, cfg, m, []string{"deny", "clients"})

	if rep.Changes["clients"]["add"] != 2 {
		t.Errorf("clients add = %d, want 2", rep.Changes["clients"]["add"])
	}
	if rep.Changes["deny"]["delete"] != 1 {
		t.Errorf("deny delete = %d, want 1", rep.Changes["deny"]["delete"])
	}
	if got := labelledValue(t, m, "forseti_reconcile_changes_total", map[string]string{"target": "replica", "type": "clients", "action": "add"}); got != 2 {
		t.Errorf("changes_total{clients,add} = %v, want 2", got)
	}
	if got := labelledValue(t, m, "forseti_reconcile_changes_total", map[string]string{"target": "replica", "type": "deny", "action": "delete"}); got != 1 {
		t.Errorf("changes_total{deny,delete} = %v, want 1", got)
	}
}

func TestReplicaSyncWaitsForGate(t *testing.T) {
	primarySrv := newFullTestServer(testData{}, &testCounters{})
	defer primarySrv.Close()
	replicaSrv := newFullTestServer(testData{}, &testCounters{})
	defer replicaSrv.Close()

	pool := session.NewPool()
	defer pool.Close()
	pool.SetAPIConfig(map[string]config.APIConfig{"replica": {MaxConcurrent: 1}})
	cfg := makeSyncConfig(primarySrv.URL, replicaSrv.URL, []string{"groups"})
	m := metrics.NewServer(0, "/metrics", config.CollectorToggles{})
	s := NewSyncer(pool, cfg, m)
	set := resourceSet("groups")
	primary, ok := s.loadPrimary(set)
	if !ok {
		t.Fatal("primary load failed")
	}

	pool.Acquire("replica")
	done := make(chan struct{})
	go func() {
		s.syncReplica(cfg.Targets[1], primary, set)
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("sync ran while the replica gate was held")
	case <-time.After(100 * time.Millisecond):
	}

	pool.Release("replica")
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("sync did not proceed after gate release")
	}
}

func TestDNSPurgeDisabledKeepsReplicaRecords(t *testing.T) {
	primarySrv := newFullTestServer(testData{dns: []string{"10.0.0.1 router.lan"}}, &testCounters{})
	defer primarySrv.Close()
	replicaCounters := &testCounters{}
	replicaSrv := newFullTestServer(testData{dns: []string{"10.0.0.9 manual.lan"}}, replicaCounters)
	defer replicaSrv.Close()

	pool := session.NewPool()
	defer pool.Close()
	cfg := makeSyncConfig(primarySrv.URL, replicaSrv.URL, nil)
	purge := false
	cfg.Sync.LocalDNSPurge = &purge
	m := metrics.NewServer(0, "/metrics", config.CollectorToggles{})

	rep := syncOnce(t, pool, cfg, m, []string{"local_dns"})

	if got := replicaCounters.deletes.Load(); got != 0 {
		t.Errorf("DNS deletes = %d, want 0 with local_dns_purge=false", got)
	}
	if got := replicaCounters.creates.Load(); got != 1 {
		t.Errorf("DNS adds = %d, want 1", got)
	}
	if rep.Status != metrics.RunSuccess {
		t.Errorf("status = %q, want success", rep.Status)
	}
}
