package collector

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/st0o0/forseti/internal/config"
	"github.com/st0o0/forseti/internal/metrics"
	"github.com/st0o0/forseti/internal/session"
)

func newNewDomainsTestServer(t *testing.T, queriesCallCount *atomic.Int32) *httptest.Server {
	t.Helper()
	now := time.Now()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/auth" && r.Method == http.MethodPost:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]string{"sid": "test-sid"},
			})
		case r.URL.Path == "/api/auth" && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/api/queries":
			queriesCallCount.Add(1)
			domain := r.URL.Query().Get("domain")
			switch domain {
			case "":
				// Stage 1: candidates seen in the last 24h window.
				_ = json.NewEncoder(w).Encode(map[string]any{
					"queries": []map[string]any{
						{"time": float64(now.Add(-1 * time.Hour).Unix()), "domain": "new.example.com", "status": "GRAVITY"},
						{"time": float64(now.Add(-2 * time.Hour).Unix()), "domain": "new.example.com", "status": "GRAVITY"},
						{"time": float64(now.Add(-3 * time.Hour).Unix()), "domain": "old.example.com", "status": "DENYLIST"},
					},
				})
			case "new.example.com":
				// Stage 2: no prior history -> genuinely new.
				_ = json.NewEncoder(w).Encode(map[string]any{"queries": []map[string]any{}})
			case "old.example.com":
				// Stage 2: prior history exists -> not new.
				_ = json.NewEncoder(w).Encode(map[string]any{
					"queries": []map[string]any{
						{"time": float64(now.Add(-48 * time.Hour).Unix()), "domain": "old.example.com", "status": "DENYLIST"},
					},
				})
			default:
				_ = json.NewEncoder(w).Encode(map[string]any{"queries": []map[string]any{}})
			}
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
}

func TestNewDomainsCollectorDetectsNewDomain(t *testing.T) {
	var queriesCallCount atomic.Int32
	srv := newNewDomainsTestServer(t, &queriesCallCount)
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	target := config.Target{Name: "test", URL: srv.URL, Password: "pw"}
	m := metrics.NewServer(0, "/metrics", config.CollectorToggles{})

	coll := NewNewDomainsCollector(pool, []config.Target{target}, time.Minute, m, config.CollectorToggles{})
	coll.CollectOnce(context.Background())

	families, err := m.Gather()
	if err != nil {
		t.Fatalf("Gather() error: %v", err)
	}
	value, found := findGaugeValue(families, "forseti_new_blocked_domains_24h", "test")
	if !found {
		t.Fatal("forseti_new_blocked_domains_24h{target=\"test\"} not found")
	}
	if value != 1 {
		t.Errorf("new blocked domains = %f, want 1 (only new.example.com, old.example.com has prior history)", value)
	}

	if queriesCallCount.Load() != 3 {
		t.Errorf("queries calls = %d, want 3 (1 window fetch + 2 existence checks)", queriesCallCount.Load())
	}
}

func TestNewDomainsCollectorToggleDisabledSkipsAPI(t *testing.T) {
	var queriesCallCount atomic.Int32
	srv := newNewDomainsTestServer(t, &queriesCallCount)
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	target := config.Target{Name: "test", URL: srv.URL, Password: "pw"}
	disabled := false
	toggles := config.CollectorToggles{NewDomains: &disabled}
	m := metrics.NewServer(0, "/metrics", toggles)

	coll := NewNewDomainsCollector(pool, []config.Target{target}, time.Minute, m, toggles)
	coll.CollectOnce(context.Background())

	if queriesCallCount.Load() != 0 {
		t.Errorf("queries calls = %d, want 0 when toggle disabled", queriesCallCount.Load())
	}
}

func TestNewDomainsCollectorPerTargetIndependence(t *testing.T) {
	var callsA, callsB atomic.Int32
	srvA := newNewDomainsTestServer(t, &callsA)
	defer srvA.Close()
	srvB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/auth" && r.Method == http.MethodPost:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]string{"sid": "test-sid-b"},
			})
		case r.URL.Path == "/api/auth" && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/api/queries":
			callsB.Add(1)
			// Target B sees no blocked queries at all in the last 24h.
			_ = json.NewEncoder(w).Encode(map[string]any{"queries": []map[string]any{}})
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srvB.Close()

	pool := session.NewPool()
	defer pool.Close()

	targets := []config.Target{
		{Name: "target-a", URL: srvA.URL, Password: "pw"},
		{Name: "target-b", URL: srvB.URL, Password: "pw"},
	}
	m := metrics.NewServer(0, "/metrics", config.CollectorToggles{})

	coll := NewNewDomainsCollector(pool, targets, time.Minute, m, config.CollectorToggles{})
	coll.CollectOnce(context.Background())

	families, err := m.Gather()
	if err != nil {
		t.Fatalf("Gather() error: %v", err)
	}

	valueA, foundA := findGaugeValue(families, "forseti_new_blocked_domains_24h", "target-a")
	if !foundA || valueA != 1 {
		t.Errorf("target-a new domains = %f (found=%v), want 1", valueA, foundA)
	}

	valueB, foundB := findGaugeValue(families, "forseti_new_blocked_domains_24h", "target-b")
	if !foundB || valueB != 0 {
		t.Errorf("target-b new domains = %f (found=%v), want 0", valueB, foundB)
	}
}

func findGaugeValue(families []*dto.MetricFamily, name string, target string) (float64, bool) {
	for _, f := range families {
		if f.GetName() != name {
			continue
		}
		for _, metric := range f.GetMetric() {
			for _, label := range metric.GetLabel() {
				if label.GetName() == "target" && label.GetValue() == target {
					return metric.GetGauge().GetValue(), true
				}
			}
		}
	}
	return 0, false
}
