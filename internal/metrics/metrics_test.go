package metrics

import (
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
