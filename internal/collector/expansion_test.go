package collector

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/st0o0/forseti/internal/config"
	"github.com/st0o0/forseti/internal/metrics"
	"github.com/st0o0/forseti/internal/session"
)

func newExpansionTestServer(calls *atomic.Int32) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/auth" && r.Method == http.MethodPost:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]string{"sid": "test-sid"},
			})
		case r.URL.Path == "/api/auth" && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/api/stats/summary":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"queries": map[string]any{
					"total": 1000, "blocked": 100, "percent_blocked": 10.0,
					"forwarded": 800, "cached": 100, "unique_domains": 500,
					"frequency": 1.5,
				},
				"clients": map[string]any{"active": 10, "total": 20},
				"gravity": map[string]any{"domains_being_blocked": 50000, "last_update": 1234567890},
			})
		case r.URL.Path == "/api/dns/blocking":
			_ = json.NewEncoder(w).Encode(map[string]any{"blocking": "enabled"})
		case r.URL.Path == "/api/stats/upstreams":
			_ = json.NewEncoder(w).Encode(map[string]any{"upstreams": []any{}})
		case r.URL.Path == "/api/dhcp/leases":
			_ = json.NewEncoder(w).Encode(map[string]any{"leases": []any{}})
		case r.URL.Path == "/api/stats/top_domains":
			calls.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"top_domains":    []any{map[string]any{"domain": "example.com", "count": 100}},
				"blocked_domains": []any{map[string]any{"domain": "ads.example.com", "count": 50}},
			})
		case r.URL.Path == "/api/stats/top_clients":
			calls.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"top_clients": []any{map[string]any{"name": "client1", "ip": "192.168.1.2", "count": 200}},
			})
		case r.URL.Path == "/api/info/system":
			calls.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"system": map[string]any{
					"cpu": 15.5,
					"memory": map[string]any{
						"ram": map[string]any{"percent_used": 45.2},
					},
					"sensors": map[string]any{"cpu_temp": 52.3},
					"load":    []any{0.5, 0.8, 1.2},
				},
			})
		case r.URL.Path == "/api/history":
			calls.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"history": []any{
					map[string]any{"timestamp": 1000, "total": 500, "blocked": 50},
					map[string]any{"timestamp": 1600, "total": 600, "blocked": 60},
				},
			})
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
}

func TestCollectTopDomains(t *testing.T) {
	var calls atomic.Int32
	srv := newExpansionTestServer(&calls)
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	target := config.Target{Name: "test", URL: srv.URL, Password: "pw"}
	toggles := config.CollectorToggles{TopDomains: boolPtr(true)}
	m := metrics.NewServer(0, "/metrics", toggles)

	coll := NewCollector(pool, []config.Target{target}, 30*time.Second, m, toggles)
	coll.Collect(context.Background())

	if got := calls.Load(); got == 0 {
		t.Error("top_domains endpoint was not called")
	}
}

func TestCollectTopClients(t *testing.T) {
	var calls atomic.Int32
	srv := newExpansionTestServer(&calls)
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	target := config.Target{Name: "test", URL: srv.URL, Password: "pw"}
	toggles := config.CollectorToggles{TopClients: boolPtr(true)}
	m := metrics.NewServer(0, "/metrics", toggles)

	coll := NewCollector(pool, []config.Target{target}, 30*time.Second, m, toggles)
	coll.Collect(context.Background())

	if got := calls.Load(); got == 0 {
		t.Error("top_clients endpoint was not called")
	}
}

func TestCollectSystemInfo(t *testing.T) {
	var calls atomic.Int32
	srv := newExpansionTestServer(&calls)
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	target := config.Target{Name: "test", URL: srv.URL, Password: "pw"}
	toggles := config.CollectorToggles{System: boolPtr(true)}
	m := metrics.NewServer(0, "/metrics", toggles)

	coll := NewCollector(pool, []config.Target{target}, 30*time.Second, m, toggles)
	coll.Collect(context.Background())

	if got := calls.Load(); got == 0 {
		t.Error("system info endpoint was not called")
	}
}

func TestCollectNewTogglesDisabledByDefault(t *testing.T) {
	var calls atomic.Int32
	srv := newExpansionTestServer(&calls)
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	target := config.Target{Name: "test", URL: srv.URL, Password: "pw"}
	toggles := config.CollectorToggles{}
	m := metrics.NewServer(0, "/metrics", toggles)

	coll := NewCollector(pool, []config.Target{target}, 30*time.Second, m, toggles)
	coll.Collect(context.Background())

	if got := calls.Load(); got != 0 {
		t.Errorf("new endpoints called %d times with toggles disabled, want 0", got)
	}
}

func TestHistoryCollectorCollectsOnce(t *testing.T) {
	var calls atomic.Int32
	srv := newExpansionTestServer(&calls)
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	target := config.Target{Name: "test", URL: srv.URL, Password: "pw"}
	toggles := config.CollectorToggles{History: boolPtr(true)}
	m := metrics.NewServer(0, "/metrics", toggles)

	hc := NewHistoryCollector(pool, []config.Target{target}, 5*time.Minute, m, toggles)
	hc.CollectOnce(context.Background())

	if got := calls.Load(); got == 0 {
		t.Error("history endpoint was not called")
	}
}

func TestHistoryCollectorDisabled(t *testing.T) {
	var calls atomic.Int32
	srv := newExpansionTestServer(&calls)
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	target := config.Target{Name: "test", URL: srv.URL, Password: "pw"}
	toggles := config.CollectorToggles{}
	m := metrics.NewServer(0, "/metrics", toggles)

	hc := NewHistoryCollector(pool, []config.Target{target}, 5*time.Minute, m, toggles)
	hc.CollectOnce(context.Background())

	if got := calls.Load(); got != 0 {
		t.Errorf("history endpoint called %d times with toggle disabled, want 0", got)
	}
}

func TestSetTopStatsCount(t *testing.T) {
	pool := session.NewPool()
	defer pool.Close()

	m := metrics.NewServer(0, "/metrics", config.CollectorToggles{})
	coll := NewCollector(pool, nil, 30*time.Second, m, config.CollectorToggles{})

	if coll.topStatsCount != config.DefaultTopStatsCount {
		t.Errorf("default topStatsCount = %d, want %d", coll.topStatsCount, config.DefaultTopStatsCount)
	}

	coll.SetTopStatsCount(25)
	if coll.topStatsCount != 25 {
		t.Errorf("topStatsCount after SetTopStatsCount(25) = %d, want 25", coll.topStatsCount)
	}

	coll.SetTopStatsCount(0)
	if coll.topStatsCount != 25 {
		t.Errorf("topStatsCount after SetTopStatsCount(0) = %d, want 25 (unchanged)", coll.topStatsCount)
	}
}

func boolPtr(b bool) *bool { return &b }
