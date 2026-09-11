package metrics

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/st0o0/forseti/internal/config"
	"github.com/st0o0/forseti/internal/pihole"
)

func TestNewServerRegistration(t *testing.T) {
	s := NewServer(0, "/metrics")
	if s == nil {
		t.Fatal("NewServer returned nil")
	}
	// Verify metrics are registered by gathering
	families, err := s.registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error: %v", err)
	}
	names := make(map[string]bool)
	for _, f := range families {
		names[f.GetName()] = true
	}
	for _, expected := range []string{
		"forseti_config_adlists",
		"forseti_config_deny_domains",
		"forseti_config_allow_domains",
	} {
		if !names[expected] {
			t.Errorf("metric %q not registered", expected)
		}
	}
}

func TestRecordReconcile(t *testing.T) {
	s := NewServer(0, "/metrics")

	s.RecordReconcile(ReconcileResult{
		Target:   "pihole-test",
		Duration: 2 * time.Second,
		Success:  true,
		Changes: map[string]map[string]int{
			"adlist": {"add": 3, "delete": 1},
		},
		Drift: map[string]int{
			"adlist": 0,
			"deny":   2,
		},
	})

	// Check runs counter
	val := getCounterValue(t, s.reconcileRuns, "pihole-test", "success")
	if val != 1 {
		t.Errorf("runs = %f, want 1", val)
	}

	// Check changes counter
	val = getCounterValue(t, s.reconcileChanges, "pihole-test", "adlist", "add")
	if val != 3 {
		t.Errorf("changes add = %f, want 3", val)
	}

	// Check drift gauge
	g, err := s.reconcileDrift.GetMetricWithLabelValues("pihole-test", "deny")
	if err != nil {
		t.Fatal(err)
	}
	m := &dto.Metric{}
	g.Write(m)
	if m.GetGauge().GetValue() != 2 {
		t.Errorf("drift deny = %f, want 2", m.GetGauge().GetValue())
	}
}

func TestUpdateStats(t *testing.T) {
	s := NewServer(0, "/metrics")

	s.UpdateStats("pihole-test", &pihole.Stats{
		QueriesTotal:      12345,
		BlockedTotal:      678,
		BlockedPercentage: 5.49,
		DomainsBlocked:    80000,
		Forwarded:         8000,
		Cached:            3000,
		UniqueDomains:     5678,
		Frequency:         1.5,
		ClientsActive:     15,
		ClientsTotal:      42,
		QueryTypes:        map[string]int{"A": 5000, "AAAA": 3000},
		QueryStatus:       map[string]int{"GRAVITY": 800, "FORWARDED": 8000},
		ReplyTypes:        map[string]int{"CNAME": 3000, "IP": 7000},
	})

	m := &dto.Metric{}

	g, _ := s.piholeQueries.GetMetricWithLabelValues("pihole-test")
	g.Write(m)
	if m.GetGauge().GetValue() != 12345 {
		t.Errorf("queries = %f, want 12345", m.GetGauge().GetValue())
	}

	g, _ = s.piholeDomainsBlocked.GetMetricWithLabelValues("pihole-test")
	g.Write(m)
	if m.GetGauge().GetValue() != 80000 {
		t.Errorf("domains_blocked = %f, want 80000", m.GetGauge().GetValue())
	}

	g, _ = s.piholeQueriesForwarded.GetMetricWithLabelValues("pihole-test")
	g.Write(m)
	if m.GetGauge().GetValue() != 8000 {
		t.Errorf("forwarded = %f, want 8000", m.GetGauge().GetValue())
	}

	g, _ = s.piholeQueriesCached.GetMetricWithLabelValues("pihole-test")
	g.Write(m)
	if m.GetGauge().GetValue() != 3000 {
		t.Errorf("cached = %f, want 3000", m.GetGauge().GetValue())
	}

	g, _ = s.piholeUniqueDomains.GetMetricWithLabelValues("pihole-test")
	g.Write(m)
	if m.GetGauge().GetValue() != 5678 {
		t.Errorf("unique_domains = %f, want 5678", m.GetGauge().GetValue())
	}

	g, _ = s.piholeRequestFrequency.GetMetricWithLabelValues("pihole-test")
	g.Write(m)
	if m.GetGauge().GetValue() != 1.5 {
		t.Errorf("frequency = %f, want 1.5", m.GetGauge().GetValue())
	}

	g, _ = s.piholeClientsActive.GetMetricWithLabelValues("pihole-test")
	g.Write(m)
	if m.GetGauge().GetValue() != 15 {
		t.Errorf("clients_active = %f, want 15", m.GetGauge().GetValue())
	}

	g, _ = s.piholeClientsTotal.GetMetricWithLabelValues("pihole-test")
	g.Write(m)
	if m.GetGauge().GetValue() != 42 {
		t.Errorf("clients_total = %f, want 42", m.GetGauge().GetValue())
	}

	g, _ = s.piholeQueryTypes.GetMetricWithLabelValues("pihole-test", "A")
	g.Write(m)
	if m.GetGauge().GetValue() != 5000 {
		t.Errorf("query_types A = %f, want 5000", m.GetGauge().GetValue())
	}

	g, _ = s.piholeQueryStatus.GetMetricWithLabelValues("pihole-test", "GRAVITY")
	g.Write(m)
	if m.GetGauge().GetValue() != 800 {
		t.Errorf("query_status GRAVITY = %f, want 800", m.GetGauge().GetValue())
	}

	g, _ = s.piholeReplyTypes.GetMetricWithLabelValues("pihole-test", "IP")
	g.Write(m)
	if m.GetGauge().GetValue() != 7000 {
		t.Errorf("reply_types IP = %f, want 7000", m.GetGauge().GetValue())
	}
}

func TestUpdateBlockingStatus(t *testing.T) {
	s := NewServer(0, "/metrics")

	s.UpdateBlockingStatus("pihole-test", true)
	g, _ := s.piholeStatus.GetMetricWithLabelValues("pihole-test")
	m := &dto.Metric{}
	g.Write(m)
	if m.GetGauge().GetValue() != 1 {
		t.Errorf("status = %f, want 1", m.GetGauge().GetValue())
	}

	s.UpdateBlockingStatus("pihole-test", false)
	g, _ = s.piholeStatus.GetMetricWithLabelValues("pihole-test")
	g.Write(m)
	if m.GetGauge().GetValue() != 0 {
		t.Errorf("status = %f, want 0", m.GetGauge().GetValue())
	}
}

func TestUpdateUpstreams(t *testing.T) {
	s := NewServer(0, "/metrics")

	s.UpdateUpstreams("pihole-test", []pihole.UpstreamStats{
		{IP: "1.1.1.1", Name: "one.one.one.one", Port: 53, Count: 5000, ResponseTime: 0.025, ResponseVariance: 0.003},
		{IP: "8.8.8.8", Name: "dns.google", Port: 53, Count: 3000, ResponseTime: 0.030, ResponseVariance: 0.005},
	})

	m := &dto.Metric{}

	g, _ := s.piholeUpstreamQueries.GetMetricWithLabelValues("pihole-test", "1.1.1.1", "one.one.one.one", "53")
	g.Write(m)
	if m.GetGauge().GetValue() != 5000 {
		t.Errorf("upstream queries 1.1.1.1 = %f, want 5000", m.GetGauge().GetValue())
	}

	g, _ = s.piholeUpstreamResponse.GetMetricWithLabelValues("pihole-test", "1.1.1.1", "one.one.one.one", "53")
	g.Write(m)
	if m.GetGauge().GetValue() != 0.025 {
		t.Errorf("upstream response 1.1.1.1 = %f, want 0.025", m.GetGauge().GetValue())
	}

	g, _ = s.piholeUpstreamVariance.GetMetricWithLabelValues("pihole-test", "8.8.8.8", "dns.google", "53")
	g.Write(m)
	if m.GetGauge().GetValue() != 0.005 {
		t.Errorf("upstream variance 8.8.8.8 = %f, want 0.005", m.GetGauge().GetValue())
	}
}

func TestSetConfigMetrics(t *testing.T) {
	s := NewServer(0, "/metrics")

	s.SetConfigMetrics(&config.Config{
		Adlists: make([]config.Adlist, 10),
		Deny:    make([]config.DenyEntry, 5),
		Allow:   make([]config.AllowEntry, 3),
	})

	m := &dto.Metric{}
	s.configAdlists.Write(m)
	if m.GetGauge().GetValue() != 10 {
		t.Errorf("adlists = %f, want 10", m.GetGauge().GetValue())
	}
	s.configDenyDomains.Write(m)
	if m.GetGauge().GetValue() != 5 {
		t.Errorf("deny = %f, want 5", m.GetGauge().GetValue())
	}
}

func TestRecordReconcileError(t *testing.T) {
	s := NewServer(0, "/metrics")

	s.RecordReconcile(ReconcileResult{
		Target:   "pihole-fail",
		Duration: 1 * time.Second,
		Success:  false,
	})

	val := getCounterValue(t, s.reconcileRuns, "pihole-fail", "error")
	if val != 1 {
		t.Errorf("error runs = %f, want 1", val)
	}

	m := &dto.Metric{}
	g, _ := s.targetReachable.GetMetricWithLabelValues("pihole-fail")
	g.Write(m)
	if m.GetGauge().GetValue() != 0 {
		t.Errorf("reachable = %f, want 0 on failure", m.GetGauge().GetValue())
	}
}

func TestGather(t *testing.T) {
	s := NewServer(0, "/metrics")
	families, err := s.Gather()
	if err != nil {
		t.Fatalf("Gather() error: %v", err)
	}
	if len(families) == 0 {
		t.Error("Gather() returned no metric families")
	}
}

func TestSetCollectFunc(t *testing.T) {
	s := NewServer(0, "/metrics")
	called := false
	s.SetCollectFunc(func(ctx context.Context) {
		called = true
	})
	if s.collectFunc == nil {
		t.Error("collectFunc should be set")
	}
	s.collectFunc(context.Background())
	if !called {
		t.Error("collectFunc was not called")
	}
}

func TestStartAndShutdown(t *testing.T) {
	s := NewServer(0, "/metrics")

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.Start()
	}()

	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := s.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() error: %v", err)
	}

	err := <-errCh
	if err != nil && err.Error() != "http: Server closed" {
		t.Errorf("Start() unexpected error: %v", err)
	}
}

func TestScrapeCallsCollectFunc(t *testing.T) {
	s := NewServer(0, "/metrics")
	called := false
	s.SetCollectFunc(func(ctx context.Context) {
		called = true
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s.httpServer.Addr = ln.Addr().String()

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.httpServer.Serve(ln)
	}()

	resp, err := http.Get("http://" + ln.Addr().String() + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics error: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if !called {
		t.Error("collectFunc was not called during scrape")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	s.Shutdown(ctx)
	<-errCh
}

func TestScrapeWithoutCollectFunc(t *testing.T) {
	s := NewServer(0, "/metrics")

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s.httpServer.Addr = ln.Addr().String()

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.httpServer.Serve(ln)
	}()

	resp, err := http.Get("http://" + ln.Addr().String() + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics error: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	s.Shutdown(ctx)
	<-errCh
}

func TestUpdateStatsStaleKeysCleanup(t *testing.T) {
	s := NewServer(0, "/metrics")

	s.UpdateStats("t", &pihole.Stats{
		QueryTypes:  map[string]int{"A": 100, "AAAA": 200},
		QueryStatus: map[string]int{"GRAVITY": 10},
		ReplyTypes:  map[string]int{"CNAME": 50},
	})

	s.UpdateStats("t", &pihole.Stats{
		QueryTypes:  map[string]int{"A": 150},
		QueryStatus: map[string]int{},
		ReplyTypes:  map[string]int{"IP": 30},
	})

	families, _ := s.Gather()
	for _, f := range families {
		if f.GetName() == "pihole_dns_queries_by_type" {
			for _, metric := range f.GetMetric() {
				for _, lp := range metric.GetLabel() {
					if lp.GetName() == "query_type" && lp.GetValue() == "AAAA" {
						t.Error("stale query type AAAA should have been removed")
					}
				}
			}
		}
	}
}

func TestRecordGravityRun(t *testing.T) {
	s := NewServer(0, "/metrics")

	s.RecordGravityRun("pihole-test", "scheduled", 5*time.Second, nil)

	val := getCounterValue(t, s.gravityRuns, "pihole-test", "scheduled")
	if val != 1 {
		t.Errorf("gravity runs = %f, want 1", val)
	}

	m := &dto.Metric{}
	g, _ := s.gravityLastRun.GetMetricWithLabelValues("pihole-test")
	g.Write(m)
	if m.GetGauge().GetValue() == 0 {
		t.Error("gravity last run timestamp should be set")
	}
}

func TestRecordGravityRunWithError(t *testing.T) {
	s := NewServer(0, "/metrics")

	s.RecordGravityRun("pihole-test", "adlist_change", 1*time.Second, fmt.Errorf("failed"))

	val := getCounterValue(t, s.gravityErrors, "pihole-test")
	if val != 1 {
		t.Errorf("gravity errors = %f, want 1", val)
	}
}

func TestMarkTargetUnreachable(t *testing.T) {
	s := NewServer(0, "/metrics")

	s.MarkTargetUnreachable("pihole-down")

	m := &dto.Metric{}
	g, _ := s.targetReachable.GetMetricWithLabelValues("pihole-down")
	g.Write(m)
	if m.GetGauge().GetValue() != 0 {
		t.Errorf("reachable = %f, want 0", m.GetGauge().GetValue())
	}
}

func TestObserveCollectorDuration(t *testing.T) {
	s := NewServer(0, "/metrics")
	s.ObserveCollectorDuration(2 * time.Second)

	families, _ := s.Gather()
	found := false
	for _, f := range families {
		if f.GetName() == "forseti_collector_duration_seconds" {
			found = true
			if f.GetMetric()[0].GetHistogram().GetSampleCount() != 1 {
				t.Errorf("sample count = %d, want 1", f.GetMetric()[0].GetHistogram().GetSampleCount())
			}
		}
	}
	if !found {
		t.Error("forseti_collector_duration_seconds not found in gathered metrics")
	}
}

func TestRecordCollectorFetch(t *testing.T) {
	s := NewServer(0, "/metrics")
	s.RecordCollectorFetch("pihole-test", "success")
	s.RecordCollectorFetch("pihole-test", "error")

	val := getCounterValue(t, s.collectorFetches, "pihole-test", "success")
	if val != 1 {
		t.Errorf("fetch success = %f, want 1", val)
	}
	val = getCounterValue(t, s.collectorFetches, "pihole-test", "error")
	if val != 1 {
		t.Errorf("fetch error = %f, want 1", val)
	}
}

func TestRecordCollectorCacheHit(t *testing.T) {
	s := NewServer(0, "/metrics")
	s.RecordCollectorCacheHit("pihole-test")
	s.RecordCollectorCacheHit("pihole-test")

	val := getCounterValue(t, s.collectorCacheHits, "pihole-test")
	if val != 2 {
		t.Errorf("cache hits = %f, want 2", val)
	}
}

func TestRecordSessionReauth(t *testing.T) {
	s := NewServer(0, "/metrics")
	s.RecordSessionReauth("pihole-test")

	val := getCounterValue(t, s.sessionReauth, "pihole-test")
	if val != 1 {
		t.Errorf("reauth = %f, want 1", val)
	}
}

func TestSessionActiveGauge(t *testing.T) {
	s := NewServer(0, "/metrics")

	s.IncSessionActive()
	s.IncSessionActive()

	m := &dto.Metric{}
	s.sessionActive.Write(m)
	if m.GetGauge().GetValue() != 2 {
		t.Errorf("session active = %f, want 2", m.GetGauge().GetValue())
	}

	s.ResetSessionActive()
	s.sessionActive.Write(m)
	if m.GetGauge().GetValue() != 0 {
		t.Errorf("session active after reset = %f, want 0", m.GetGauge().GetValue())
	}
}

func TestSetBuildInfo(t *testing.T) {
	s := NewServer(0, "/metrics")
	s.SetBuildInfo("1.2.3", "config")

	m := &dto.Metric{}
	g, _ := s.buildInfo.GetMetricWithLabelValues("1.2.3", "config")
	g.Write(m)
	if m.GetGauge().GetValue() != 1 {
		t.Errorf("build info = %f, want 1", m.GetGauge().GetValue())
	}
}

func TestSplitUpstreamKey(t *testing.T) {
	tests := []struct {
		key  string
		want [3]string
	}{
		{"1.1.1.1|one.one|53", [3]string{"1.1.1.1", "one.one", "53"}},
		{"8.8.8.8|dns.google|443", [3]string{"8.8.8.8", "dns.google", "443"}},
		{"||", [3]string{"", "", ""}},
	}
	for _, tt := range tests {
		got := splitUpstreamKey(tt.key)
		if got != tt.want {
			t.Errorf("splitUpstreamKey(%q) = %v, want %v", tt.key, got, tt.want)
		}
	}
}

func TestUpdateUpstreamsStaleCleanup(t *testing.T) {
	s := NewServer(0, "/metrics")

	s.UpdateUpstreams("pihole-test", []pihole.UpstreamStats{
		{IP: "1.1.1.1", Name: "one", Port: 53, Count: 100, ResponseTime: 0.01, ResponseVariance: 0.001},
		{IP: "8.8.8.8", Name: "google", Port: 53, Count: 200, ResponseTime: 0.02, ResponseVariance: 0.002},
	})

	s.UpdateUpstreams("pihole-test", []pihole.UpstreamStats{
		{IP: "1.1.1.1", Name: "one", Port: 53, Count: 150, ResponseTime: 0.01, ResponseVariance: 0.001},
	})

	m := &dto.Metric{}
	g, _ := s.piholeUpstreamQueries.GetMetricWithLabelValues("pihole-test", "1.1.1.1", "one", "53")
	g.Write(m)
	if m.GetGauge().GetValue() != 150 {
		t.Errorf("remaining upstream = %f, want 150", m.GetGauge().GetValue())
	}

	families, _ := s.Gather()
	for _, f := range families {
		if f.GetName() != "pihole_upstream_queries" {
			continue
		}
		for _, metric := range f.GetMetric() {
			for _, lp := range metric.GetLabel() {
				if lp.GetName() == "upstream" && lp.GetValue() == "8.8.8.8" {
					t.Error("stale upstream 8.8.8.8 should have been cleaned up")
				}
			}
		}
	}
}

func getCounterValue(t *testing.T, cv *prometheus.CounterVec, labels ...string) float64 {
	t.Helper()
	c, err := cv.GetMetricWithLabelValues(labels...)
	if err != nil {
		t.Fatal(err)
	}
	m := &dto.Metric{}
	c.Write(m)
	return m.GetCounter().GetValue()
}
