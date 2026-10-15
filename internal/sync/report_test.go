package sync

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/st0o0/forseti/internal/config"
	"github.com/st0o0/forseti/internal/logging/logtest"
	"github.com/st0o0/forseti/internal/metrics"
	"github.com/st0o0/forseti/internal/pihole"
	"github.com/st0o0/forseti/internal/run"
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

func syncOnce(t *testing.T, pool *session.Pool, cfg *config.Config, m *metrics.Server, resources []string) run.Result {
	t.Helper()
	cfg.Sync.Resources = resources
	_, results := NewSyncer(pool, cfg, m, nil).Cycle(time.Now())
	return replicaResult(t, results)
}

func replicaResult(t *testing.T, results []run.Result) run.Result {
	t.Helper()
	for _, r := range results {
		if r.Target == "replica" {
			return r
		}
	}
	t.Fatalf("no replica result in %+v", results)
	return run.Result{}
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

	if rep.Status != run.OK {
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

	if rep.Status != run.Partial {
		t.Errorf("status = %q, want partial", rep.Status)
	}
	if rep.ErrorCount != 1 {
		t.Errorf("errors = %d, want 1", rep.ErrorCount)
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

	if rep.Status != run.Failed {
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

	if rep.Status != run.Failed {
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

	if rep.ChangeMap()["clients"]["add"] != 2 {
		t.Errorf("clients add = %d, want 2", rep.ChangeMap()["clients"]["add"])
	}
	if rep.ChangeMap()["deny"]["delete"] != 1 {
		t.Errorf("deny delete = %d, want 1", rep.ChangeMap()["deny"]["delete"])
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
	s := NewSyncer(pool, cfg, m, nil)

	pool.Acquire("replica")
	done := make(chan struct{})
	go func() {
		s.Cycle(time.Now())
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

func TestPrimaryFailureSkipsReplicas(t *testing.T) {
	rec := logtest.New(t)
	primarySrv := newUnreachableServer()
	defer primarySrv.Close()
	replicaCounters := &testCounters{}
	replicaSrv := newFullTestServer(testData{}, replicaCounters)
	defer replicaSrv.Close()

	pool := session.NewPool()
	defer pool.Close()
	cfg := makeSyncConfig(primarySrv.URL, replicaSrv.URL, []string{"groups"})
	cfg.Targets = append(cfg.Targets, config.Target{Name: "replica-2", URL: replicaSrv.URL, Password: "pw", Role: "replica"})
	m := metrics.NewServer(0, "/metrics", config.CollectorToggles{})

	cycle, results := NewSyncer(pool, cfg, m, nil).Cycle(time.Now())

	if cycle.Failed != 1 || cycle.Skipped != 2 {
		t.Errorf("cycle = %+v, want failed=1 skipped=2", cycle)
	}
	for _, r := range results[1:] {
		if r.Status != run.Skipped || r.Stage != run.StagePrimary {
			t.Errorf("replica %s: status=%s stage=%s", r.Target, r.Status, r.Stage)
		}
	}
	if got := runsTotal(t, m, "replica", "error") + runsTotal(t, m, "replica", "success"); got != 0 {
		t.Errorf("skipped replica recorded in metrics: %v", got)
	}
	if got := runsTotal(t, m, "primary", "error"); got != 1 {
		t.Errorf("primary runs{error} = %v, want 1", got)
	}
	ev := rec.Find("cycle complete")
	if len(ev) != 1 || ev[0].Attrs["skipped"] != int64(2) || ev[0].Attrs["component"] != "sync" {
		t.Errorf("cycle complete = %+v", ev)
	}
}

func TestPrimaryStateLoadedOncePerCycle(t *testing.T) {
	var groupReads atomic.Int32
	primarySrv := newFullTestServer(testData{}, &testCounters{})
	defer primarySrv.Close()
	counting := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/groups" && r.Method == http.MethodGet {
			groupReads.Add(1)
		}
		primarySrv.Config.Handler.ServeHTTP(w, r)
	}))
	defer counting.Close()
	replicaSrv := newFullTestServer(testData{}, &testCounters{})
	defer replicaSrv.Close()

	pool := session.NewPool()
	defer pool.Close()
	cfg := makeSyncConfig(counting.URL, replicaSrv.URL, []string{"groups"})
	for i := 2; i <= 3; i++ {
		cfg.Targets = append(cfg.Targets, config.Target{Name: fmt.Sprintf("replica-%d", i), URL: replicaSrv.URL, Password: "pw", Role: "replica"})
	}
	m := metrics.NewServer(0, "/metrics", config.CollectorToggles{})

	NewSyncer(pool, cfg, m, nil).Cycle(time.Now())

	if got := groupReads.Load(); got != 1 {
		t.Errorf("primary group reads = %d, want 1", got)
	}
}

func TestReplicaBackoffAfterFailures(t *testing.T) {
	primarySrv := newFullTestServer(testData{}, &testCounters{})
	defer primarySrv.Close()
	replicaSrv := newUnreachableServer()
	defer replicaSrv.Close()

	pool := session.NewPool()
	defer pool.Close()
	cfg := makeSyncConfig(primarySrv.URL, replicaSrv.URL, []string{"groups"})
	cfg.Sync.Interval.Duration = time.Hour
	m := metrics.NewServer(0, "/metrics", config.CollectorToggles{})
	s := NewSyncer(pool, cfg, m, nil)

	_, first := s.Cycle(time.Now())
	if r := replicaResult(t, first); r.Status != run.Failed {
		t.Fatalf("first run status = %s, want failed", r.Status)
	}
	_, second := s.Cycle(time.Now())
	if r := replicaResult(t, second); r.Status != run.Skipped || r.Stage != run.StageBackoff {
		t.Errorf("second run = %s/%s, want skipped/backoff", r.Status, r.Stage)
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
	if rep.Status != run.OK {
		t.Errorf("status = %q, want success", rep.Status)
	}
}
