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

func newPiholeTestServer(callCount *atomic.Int32) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/auth" && r.Method == http.MethodPost:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]string{"sid": "test-sid"},
			})
		case r.URL.Path == "/api/auth" && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/api/stats/summary":
			callCount.Add(1)
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
			callCount.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{"blocking": true})
		case r.URL.Path == "/api/stats/upstreams":
			callCount.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{"upstreams": []any{}})
		case r.URL.Path == "/api/dhcp/leases":
			callCount.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{"leases": []any{}})
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
}

func TestCollectCacheMiss(t *testing.T) {
	var callCount atomic.Int32
	srv := newPiholeTestServer(&callCount)
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	target := config.Target{Name: "test", URL: srv.URL, Password: "pw"}
	m := metrics.NewServer(0, "/metrics", config.CollectorToggles{})

	coll := NewCollector(pool, []config.Target{target}, 30*time.Second, m, config.CollectorToggles{})

	ctx := context.Background()
	coll.Collect(ctx)

	if got := callCount.Load(); got != 4 {
		t.Errorf("API calls on cache miss = %d, want 4 (stats + blocking + upstreams + dhcp)", got)
	}
}

func TestCollectCacheHit(t *testing.T) {
	var callCount atomic.Int32
	srv := newPiholeTestServer(&callCount)
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	target := config.Target{Name: "test", URL: srv.URL, Password: "pw"}
	m := metrics.NewServer(0, "/metrics", config.CollectorToggles{})

	coll := NewCollector(pool, []config.Target{target}, 30*time.Second, m, config.CollectorToggles{})

	ctx := context.Background()
	coll.Collect(ctx)

	before := callCount.Load()
	coll.Collect(ctx)
	after := callCount.Load()

	if after != before {
		t.Errorf("cache hit should not make API calls, got %d additional calls", after-before)
	}
}

func TestCollectCacheExpiry(t *testing.T) {
	var callCount atomic.Int32
	srv := newPiholeTestServer(&callCount)
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	target := config.Target{Name: "test", URL: srv.URL, Password: "pw"}
	m := metrics.NewServer(0, "/metrics", config.CollectorToggles{})

	coll := NewCollector(pool, []config.Target{target}, 10*time.Millisecond, m, config.CollectorToggles{})

	ctx := context.Background()
	coll.Collect(ctx)
	first := callCount.Load()

	time.Sleep(20 * time.Millisecond)
	coll.Collect(ctx)
	second := callCount.Load()

	if second <= first {
		t.Errorf("expired cache should trigger new API calls, first=%d second=%d", first, second)
	}
}

func TestCollectTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/auth" && r.Method == http.MethodPost {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]string{"sid": "test-sid"},
			})
			return
		}
		if r.URL.Path == "/api/auth" && r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		time.Sleep(5 * time.Second)
	}))
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	target := config.Target{Name: "slow", URL: srv.URL, Password: "pw"}
	m := metrics.NewServer(0, "/metrics", config.CollectorToggles{})

	coll := NewCollector(pool, []config.Target{target}, 30*time.Second, m, config.CollectorToggles{})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	coll.Collect(ctx)
	elapsed := time.Since(start)

	if elapsed > 2*time.Second {
		t.Errorf("Collect should respect context timeout, took %s", elapsed)
	}
}

func TestCollectMetricsFetchAndCacheHit(t *testing.T) {
	var callCount atomic.Int32
	srv := newPiholeTestServer(&callCount)
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	target := config.Target{Name: "test", URL: srv.URL, Password: "pw"}
	m := metrics.NewServer(0, "/metrics", config.CollectorToggles{})

	coll := NewCollector(pool, []config.Target{target}, 30*time.Second, m, config.CollectorToggles{})

	ctx := context.Background()
	coll.Collect(ctx)

	fetchVal := getCounterValue(t, m, "forseti_collector_fetches_total", map[string]string{"target": "test", "status": "success"})
	if fetchVal != 1 {
		t.Errorf("fetch success = %f, want 1", fetchVal)
	}

	coll.Collect(ctx)

	cacheVal := getCounterValue(t, m, "forseti_collector_cache_hits_total", map[string]string{"target": "test"})
	if cacheVal != 1 {
		t.Errorf("cache hits = %f, want 1", cacheVal)
	}
}

func TestCollectParallelTargets(t *testing.T) {
	var callCount atomic.Int32
	srv := newPiholeTestServer(&callCount)
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	targets := []config.Target{
		{Name: "alpha", URL: srv.URL, Password: "pw"},
		{Name: "beta", URL: srv.URL, Password: "pw"},
	}
	m := metrics.NewServer(0, "/metrics", config.CollectorToggles{})

	coll := NewCollector(pool, targets, 30*time.Second, m, config.CollectorToggles{})

	ctx := context.Background()
	coll.Collect(ctx)

	if got := callCount.Load(); got != 8 {
		t.Errorf("API calls for 2 targets = %d, want 8 (4 per target)", got)
	}
}

func TestCollectPartialBlockingError(t *testing.T) {
	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/auth" && r.Method == http.MethodPost:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]string{"sid": "test-sid"},
			})
		case r.URL.Path == "/api/auth" && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/api/stats/summary":
			callCount.Add(1)
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
			w.WriteHeader(http.StatusInternalServerError)
		case r.URL.Path == "/api/stats/upstreams":
			_ = json.NewEncoder(w).Encode(map[string]any{"upstreams": []any{}})
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	target := config.Target{Name: "partial", URL: srv.URL, Password: "pw"}
	m := metrics.NewServer(0, "/metrics", config.CollectorToggles{})

	coll := NewCollector(pool, []config.Target{target}, 30*time.Second, m, config.CollectorToggles{})

	ctx := context.Background()
	coll.Collect(ctx)

	fetchVal := getCounterValue(t, m, "forseti_collector_fetches_total", map[string]string{"target": "partial", "status": "success"})
	if fetchVal != 1 {
		t.Errorf("fetch success = %f, want 1 (stats OK despite blocking error)", fetchVal)
	}
}

func TestCollectPartialUpstreamsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			_ = json.NewEncoder(w).Encode(map[string]any{"blocking": true})
		case r.URL.Path == "/api/stats/upstreams":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	target := config.Target{Name: "partial-ups", URL: srv.URL, Password: "pw"}
	m := metrics.NewServer(0, "/metrics", config.CollectorToggles{})

	coll := NewCollector(pool, []config.Target{target}, 30*time.Second, m, config.CollectorToggles{})

	ctx := context.Background()
	coll.Collect(ctx)

	fetchVal := getCounterValue(t, m, "forseti_collector_fetches_total", map[string]string{"target": "partial-ups", "status": "success"})
	if fetchVal != 1 {
		t.Errorf("fetch success = %f, want 1 (stats OK despite upstreams error)", fetchVal)
	}
}

func TestCollectSessionError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	target := config.Target{Name: "unreachable", URL: srv.URL, Password: "pw"}
	m := metrics.NewServer(0, "/metrics", config.CollectorToggles{})

	coll := NewCollector(pool, []config.Target{target}, 30*time.Second, m, config.CollectorToggles{})

	ctx := context.Background()
	coll.Collect(ctx)

	fetchVal := getCounterValue(t, m, "forseti_collector_fetches_total", map[string]string{"target": "unreachable", "status": "error"})
	if fetchVal != 1 {
		t.Errorf("fetch error = %f, want 1 (session error)", fetchVal)
	}
}

func TestCollectStatsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/auth" && r.Method == http.MethodPost:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]string{"sid": "test-sid"},
			})
		case r.URL.Path == "/api/auth" && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/api/stats/summary":
			w.WriteHeader(http.StatusInternalServerError)
		case r.URL.Path == "/api/dns/blocking":
			_ = json.NewEncoder(w).Encode(map[string]any{"blocking": true})
		case r.URL.Path == "/api/stats/upstreams":
			_ = json.NewEncoder(w).Encode(map[string]any{"upstreams": []any{}})
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	target := config.Target{Name: "statserr", URL: srv.URL, Password: "pw"}
	m := metrics.NewServer(0, "/metrics", config.CollectorToggles{})

	coll := NewCollector(pool, []config.Target{target}, 30*time.Second, m, config.CollectorToggles{})

	ctx := context.Background()
	coll.Collect(ctx)

	fetchVal := getCounterValue(t, m, "forseti_collector_fetches_total", map[string]string{"target": "statserr", "status": "error"})
	if fetchVal != 1 {
		t.Errorf("fetch error = %f, want 1 (stats error)", fetchVal)
	}
}

func TestCollectAllCachedNoFetch(t *testing.T) {
	var callCount atomic.Int32
	srv := newPiholeTestServer(&callCount)
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	targets := []config.Target{
		{Name: "a", URL: srv.URL, Password: "pw"},
		{Name: "b", URL: srv.URL, Password: "pw"},
	}
	m := metrics.NewServer(0, "/metrics", config.CollectorToggles{})

	coll := NewCollector(pool, targets, 30*time.Second, m, config.CollectorToggles{})

	ctx := context.Background()
	coll.Collect(ctx)

	before := callCount.Load()
	coll.Collect(ctx)
	after := callCount.Load()

	if after != before {
		t.Errorf("all cached should not make API calls, got %d additional", after-before)
	}

	for _, name := range []string{"a", "b"} {
		cacheVal := getCounterValue(t, m, "forseti_collector_cache_hits_total", map[string]string{"target": name})
		if cacheVal != 1 {
			t.Errorf("cache hits for %s = %f, want 1", name, cacheVal)
		}
	}
}

func getCounterValue(t *testing.T, m *metrics.Server, name string, labels map[string]string) float64 {
	t.Helper()
	families, err := m.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range families {
		if f.GetName() != name {
			continue
		}
		for _, metric := range f.GetMetric() {
			match := true
			for k, v := range labels {
				found := false
				for _, lp := range metric.GetLabel() {
					if lp.GetName() == k && lp.GetValue() == v {
						found = true
						break
					}
				}
				if !found {
					match = false
					break
				}
			}
			if match {
				return metric.GetCounter().GetValue()
			}
		}
	}
	return 0
}
