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

	target := config.Target{
		Name:     "slow",
		URL:      srv.URL,
		Password: "pw",
		API:      config.APIConfig{Timeout: config.Duration{Duration: 50 * time.Millisecond}},
	}
	m := metrics.NewServer(0, "/metrics", config.CollectorToggles{})

	coll := NewCollector(pool, []config.Target{target}, 30*time.Second, m, config.CollectorToggles{})

	start := time.Now()
	coll.Collect(context.Background())
	elapsed := time.Since(start)

	if elapsed > 2*time.Second {
		t.Errorf("Collect should respect per-target timeout, took %s", elapsed)
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

func TestCollectStaleOnBusy(t *testing.T) {
	var callCount atomic.Int32
	srv := newPiholeTestServer(&callCount)
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	target := config.Target{
		Name:     "busy",
		URL:      srv.URL,
		Password: "pw",
		API:      config.APIConfig{MaxConcurrent: 1},
	}
	pool.SetAPIConfig(map[string]config.APIConfig{
		"busy": target.API,
	})

	m := metrics.NewServer(0, "/metrics", config.CollectorToggles{})
	coll := NewCollector(pool, []config.Target{target}, 30*time.Second, m, config.CollectorToggles{})

	pool.Acquire("busy")

	coll.Collect(context.Background())

	staleVal := getCounterValue(t, m, "forseti_collector_cache_stale_total", map[string]string{"target": "busy"})
	if staleVal != 1 {
		t.Errorf("cache stale = %f, want 1", staleVal)
	}

	fetchVal := getCounterValue(t, m, "forseti_collector_fetches_total", map[string]string{"target": "busy", "status": "success"})
	if fetchVal != 0 {
		t.Errorf("fetch success = %f, want 0 (should not have fetched)", fetchVal)
	}

	pool.Release("busy")
}

func TestCollectAfterBusy(t *testing.T) {
	var callCount atomic.Int32
	srv := newPiholeTestServer(&callCount)
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	target := config.Target{
		Name:     "recover",
		URL:      srv.URL,
		Password: "pw",
		API:      config.APIConfig{MaxConcurrent: 1},
	}
	pool.SetAPIConfig(map[string]config.APIConfig{
		"recover": target.API,
	})

	m := metrics.NewServer(0, "/metrics", config.CollectorToggles{})
	coll := NewCollector(pool, []config.Target{target}, 30*time.Second, m, config.CollectorToggles{})

	pool.Acquire("recover")
	coll.Collect(context.Background())
	pool.Release("recover")

	coll.Collect(context.Background())

	fetchVal := getCounterValue(t, m, "forseti_collector_fetches_total", map[string]string{"target": "recover", "status": "success"})
	if fetchVal != 1 {
		t.Errorf("fetch success = %f, want 1 (should fetch after gate released)", fetchVal)
	}
}

func TestCollectMultiTargetOneBusy(t *testing.T) {
	var callCount atomic.Int32
	srv := newPiholeTestServer(&callCount)
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	fast := config.Target{Name: "fast", URL: srv.URL, Password: "pw", API: config.APIConfig{MaxConcurrent: 4}}
	slow := config.Target{Name: "slow", URL: srv.URL, Password: "pw", API: config.APIConfig{MaxConcurrent: 1}}
	pool.SetAPIConfig(map[string]config.APIConfig{
		"fast": fast.API,
		"slow": slow.API,
	})

	m := metrics.NewServer(0, "/metrics", config.CollectorToggles{})
	coll := NewCollector(pool, []config.Target{fast, slow}, 30*time.Second, m, config.CollectorToggles{})

	pool.Acquire("slow")

	coll.Collect(context.Background())

	fastFetch := getCounterValue(t, m, "forseti_collector_fetches_total", map[string]string{"target": "fast", "status": "success"})
	if fastFetch != 1 {
		t.Errorf("fast fetch = %f, want 1 (should succeed despite slow being busy)", fastFetch)
	}

	slowStale := getCounterValue(t, m, "forseti_collector_cache_stale_total", map[string]string{"target": "slow"})
	if slowStale != 1 {
		t.Errorf("slow stale = %f, want 1 (should serve stale while busy)", slowStale)
	}

	slowFetch := getCounterValue(t, m, "forseti_collector_fetches_total", map[string]string{"target": "slow", "status": "success"})
	if slowFetch != 0 {
		t.Errorf("slow fetch = %f, want 0 (should not fetch while busy)", slowFetch)
	}

	pool.Release("slow")
}

func TestCollectConcurrentReconcileAndScrape(t *testing.T) {
	var callCount atomic.Int32
	srv := newPiholeTestServer(&callCount)
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	target := config.Target{
		Name:     "contend",
		URL:      srv.URL,
		Password: "pw",
		API:      config.APIConfig{MaxConcurrent: 1},
	}
	pool.SetAPIConfig(map[string]config.APIConfig{
		"contend": target.API,
	})

	m := metrics.NewServer(0, "/metrics", config.CollectorToggles{})
	coll := NewCollector(pool, []config.Target{target}, 10*time.Millisecond, m, config.CollectorToggles{})

	coll.Collect(context.Background())
	time.Sleep(15 * time.Millisecond)

	pool.Acquire("contend")

	coll.Collect(context.Background())

	staleVal := getCounterValue(t, m, "forseti_collector_cache_stale_total", map[string]string{"target": "contend"})
	if staleVal != 1 {
		t.Errorf("stale = %f, want 1 (cache expired + gate held = stale serve)", staleVal)
	}

	pool.Release("contend")

	coll.Collect(context.Background())

	fetchVal := getCounterValue(t, m, "forseti_collector_fetches_total", map[string]string{"target": "contend", "status": "success"})
	if fetchVal != 2 {
		t.Errorf("fetch = %f, want 2 (initial + after release)", fetchVal)
	}
}

func TestCollectPerTargetTimeout(t *testing.T) {
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
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	shortTimeout := config.Target{
		Name:     "short",
		URL:      srv.URL,
		Password: "pw",
		API:      config.APIConfig{Timeout: config.Duration{Duration: 50 * time.Millisecond}},
	}
	longTimeout := config.Target{
		Name:     "long",
		URL:      srv.URL,
		Password: "pw",
		API:      config.APIConfig{Timeout: config.Duration{Duration: 5 * time.Second}},
	}
	pool.SetAPIConfig(map[string]config.APIConfig{
		"short": shortTimeout.API,
		"long":  longTimeout.API,
	})

	m := metrics.NewServer(0, "/metrics", config.CollectorToggles{})
	coll := NewCollector(pool, []config.Target{shortTimeout, longTimeout}, 30*time.Second, m, config.CollectorToggles{})

	coll.Collect(context.Background())

	shortErr := getCounterValue(t, m, "forseti_collector_fetches_total", map[string]string{"target": "short", "status": "error"})
	if shortErr == 0 {
		shortOk := getCounterValue(t, m, "forseti_collector_fetches_total", map[string]string{"target": "short", "status": "success"})
		if shortOk > 0 {
			t.Errorf("short target should timeout with 50ms budget against 200ms server, but got success=%f", shortOk)
		}
	}

	longOk := getCounterValue(t, m, "forseti_collector_fetches_total", map[string]string{"target": "long", "status": "success"})
	if longOk != 1 {
		t.Errorf("long fetch = %f, want 1 (5s budget for 200ms server should succeed)", longOk)
	}
}

func TestCollectGateHighConcurrency(t *testing.T) {
	var callCount atomic.Int32
	srv := newPiholeTestServer(&callCount)
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	target := config.Target{
		Name:     "high",
		URL:      srv.URL,
		Password: "pw",
		API:      config.APIConfig{MaxConcurrent: 4},
	}
	pool.SetAPIConfig(map[string]config.APIConfig{
		"high": target.API,
	})

	if !pool.TryAcquire("high") {
		t.Fatal("slot 1 should succeed")
	}
	if !pool.TryAcquire("high") {
		t.Fatal("slot 2 should succeed")
	}
	if !pool.TryAcquire("high") {
		t.Fatal("slot 3 should succeed")
	}

	m := metrics.NewServer(0, "/metrics", config.CollectorToggles{})
	coll := NewCollector(pool, []config.Target{target}, 30*time.Second, m, config.CollectorToggles{})

	coll.Collect(context.Background())

	fetchVal := getCounterValue(t, m, "forseti_collector_fetches_total", map[string]string{"target": "high", "status": "success"})
	if fetchVal != 1 {
		t.Errorf("fetch = %f, want 1 (3 of 4 slots used, 1 still available for collector)", fetchVal)
	}

	pool.Release("high")
	pool.Release("high")
	pool.Release("high")
}

func TestCollectGateFullHighConcurrency(t *testing.T) {
	var callCount atomic.Int32
	srv := newPiholeTestServer(&callCount)
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	target := config.Target{
		Name:     "full",
		URL:      srv.URL,
		Password: "pw",
		API:      config.APIConfig{MaxConcurrent: 2},
	}
	pool.SetAPIConfig(map[string]config.APIConfig{
		"full": target.API,
	})

	pool.Acquire("full")
	pool.Acquire("full")

	m := metrics.NewServer(0, "/metrics", config.CollectorToggles{})
	coll := NewCollector(pool, []config.Target{target}, 30*time.Second, m, config.CollectorToggles{})

	coll.Collect(context.Background())

	staleVal := getCounterValue(t, m, "forseti_collector_cache_stale_total", map[string]string{"target": "full"})
	if staleVal != 1 {
		t.Errorf("stale = %f, want 1 (all slots held)", staleVal)
	}

	pool.Release("full")
	pool.Release("full")
}

func TestCollectRepeatedStaleWhileBusy(t *testing.T) {
	var callCount atomic.Int32
	srv := newPiholeTestServer(&callCount)
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	target := config.Target{
		Name:     "held",
		URL:      srv.URL,
		Password: "pw",
		API:      config.APIConfig{MaxConcurrent: 1},
	}
	pool.SetAPIConfig(map[string]config.APIConfig{"held": target.API})

	m := metrics.NewServer(0, "/metrics", config.CollectorToggles{})
	coll := NewCollector(pool, []config.Target{target}, 5*time.Millisecond, m, config.CollectorToggles{})

	coll.Collect(context.Background())
	time.Sleep(10 * time.Millisecond)

	pool.Acquire("held")

	coll.Collect(context.Background())
	coll.Collect(context.Background())
	coll.Collect(context.Background())

	staleVal := getCounterValue(t, m, "forseti_collector_cache_stale_total", map[string]string{"target": "held"})
	if staleVal != 3 {
		t.Errorf("stale = %f, want 3 (three scrapes while gate held)", staleVal)
	}

	pool.Release("held")

	coll.Collect(context.Background())

	fetchVal := getCounterValue(t, m, "forseti_collector_fetches_total", map[string]string{"target": "held", "status": "success"})
	if fetchVal != 2 {
		t.Errorf("fetch = %f, want 2 (initial + recovery after release)", fetchVal)
	}
}

func TestCollectDefaultConfigNoAPIBlock(t *testing.T) {
	var callCount atomic.Int32
	srv := newPiholeTestServer(&callCount)
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	target := config.Target{
		Name:     "noapi",
		URL:      srv.URL,
		Password: "pw",
	}

	m := metrics.NewServer(0, "/metrics", config.CollectorToggles{})
	coll := NewCollector(pool, []config.Target{target}, 30*time.Second, m, config.CollectorToggles{})

	coll.Collect(context.Background())

	fetchVal := getCounterValue(t, m, "forseti_collector_fetches_total", map[string]string{"target": "noapi", "status": "success"})
	if fetchVal != 1 {
		t.Errorf("fetch = %f, want 1 (default config should work)", fetchVal)
	}
}

func TestCollectGateReleasedOnSessionError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	target := config.Target{
		Name:     "autherr",
		URL:      srv.URL,
		Password: "pw",
		API:      config.APIConfig{MaxConcurrent: 1},
	}
	pool.SetAPIConfig(map[string]config.APIConfig{"autherr": target.API})

	m := metrics.NewServer(0, "/metrics", config.CollectorToggles{})
	coll := NewCollector(pool, []config.Target{target}, 30*time.Second, m, config.CollectorToggles{})

	coll.Collect(context.Background())

	if !pool.TryAcquire("autherr") {
		t.Error("gate should be released after session error")
	}
	pool.Release("autherr")

	errVal := getCounterValue(t, m, "forseti_collector_fetches_total", map[string]string{"target": "autherr", "status": "error"})
	if errVal != 1 {
		t.Errorf("error fetch = %f, want 1", errVal)
	}
}

func TestCollectThreeTargetsMixedState(t *testing.T) {
	var callCount atomic.Int32
	srv := newPiholeTestServer(&callCount)
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	healthy := config.Target{Name: "healthy", URL: srv.URL, Password: "pw", API: config.APIConfig{MaxConcurrent: 4}}
	busy := config.Target{Name: "busy3", URL: srv.URL, Password: "pw", API: config.APIConfig{MaxConcurrent: 1}}
	cached := config.Target{Name: "cached", URL: srv.URL, Password: "pw", API: config.APIConfig{MaxConcurrent: 4}}

	pool.SetAPIConfig(map[string]config.APIConfig{
		"healthy": healthy.API,
		"busy3":   busy.API,
		"cached":  cached.API,
	})

	m := metrics.NewServer(0, "/metrics", config.CollectorToggles{})
	coll := NewCollector(pool, []config.Target{healthy, busy, cached}, 30*time.Second, m, config.CollectorToggles{})

	coll.Collect(context.Background())

	pool.Acquire("busy3")

	coll.Collect(context.Background())

	healthyHit := getCounterValue(t, m, "forseti_collector_cache_hits_total", map[string]string{"target": "healthy"})
	if healthyHit != 1 {
		t.Errorf("healthy cache hit = %f, want 1 (within TTL)", healthyHit)
	}

	busyStale := getCounterValue(t, m, "forseti_collector_cache_stale_total", map[string]string{"target": "busy3"})
	if busyStale != 0 {
		t.Errorf("busy stale = %f, want 0 (within TTL, should be cache hit not stale)", busyStale)
	}

	cachedHit := getCounterValue(t, m, "forseti_collector_cache_hits_total", map[string]string{"target": "cached"})
	if cachedHit != 1 {
		t.Errorf("cached hit = %f, want 1", cachedHit)
	}

	pool.Release("busy3")
}

func TestCollectBusyTargetCacheExpiredServeStale(t *testing.T) {
	var callCount atomic.Int32
	srv := newPiholeTestServer(&callCount)
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	target := config.Target{
		Name:     "staleexp",
		URL:      srv.URL,
		Password: "pw",
		API:      config.APIConfig{MaxConcurrent: 1},
	}
	pool.SetAPIConfig(map[string]config.APIConfig{"staleexp": target.API})

	m := metrics.NewServer(0, "/metrics", config.CollectorToggles{})
	coll := NewCollector(pool, []config.Target{target}, 5*time.Millisecond, m, config.CollectorToggles{})

	coll.Collect(context.Background())

	fetch1 := getCounterValue(t, m, "forseti_collector_fetches_total", map[string]string{"target": "staleexp", "status": "success"})
	if fetch1 != 1 {
		t.Fatalf("initial fetch = %f, want 1", fetch1)
	}

	time.Sleep(10 * time.Millisecond)
	pool.Acquire("staleexp")

	coll.Collect(context.Background())

	staleVal := getCounterValue(t, m, "forseti_collector_cache_stale_total", map[string]string{"target": "staleexp"})
	if staleVal != 1 {
		t.Errorf("stale = %f, want 1 (cache expired + gate held = stale)", staleVal)
	}

	fetch2 := getCounterValue(t, m, "forseti_collector_fetches_total", map[string]string{"target": "staleexp", "status": "success"})
	if fetch2 != 1 {
		t.Errorf("fetch after stale = %f, want 1 (no new fetch while busy)", fetch2)
	}

	pool.Release("staleexp")
}

func TestCollectSlowServerDifferentTimeouts(t *testing.T) {
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
		time.Sleep(150 * time.Millisecond)
		_, _ = w.Write([]byte(`{"queries":{},"clients":{}}`))
	}))
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	fast := config.Target{
		Name:     "fast-to",
		URL:      srv.URL,
		Password: "pw",
		API:      config.APIConfig{Timeout: config.Duration{Duration: 50 * time.Millisecond}},
	}
	medium := config.Target{
		Name:     "medium-to",
		URL:      srv.URL,
		Password: "pw",
		API:      config.APIConfig{Timeout: config.Duration{Duration: 300 * time.Millisecond}},
	}
	slow := config.Target{
		Name:     "slow-to",
		URL:      srv.URL,
		Password: "pw",
		API:      config.APIConfig{Timeout: config.Duration{Duration: 5 * time.Second}},
	}

	pool.SetAPIConfig(map[string]config.APIConfig{
		"fast-to":   fast.API,
		"medium-to": medium.API,
		"slow-to":   slow.API,
	})

	m := metrics.NewServer(0, "/metrics", config.CollectorToggles{})
	coll := NewCollector(pool, []config.Target{fast, medium, slow}, 30*time.Second, m, config.CollectorToggles{})

	coll.Collect(context.Background())

	fastErr := getCounterValue(t, m, "forseti_collector_fetches_total", map[string]string{"target": "fast-to", "status": "error"})
	fastOk := getCounterValue(t, m, "forseti_collector_fetches_total", map[string]string{"target": "fast-to", "status": "success"})
	if fastErr == 0 && fastOk > 0 {
		t.Errorf("fast-to should timeout (50ms budget, 150ms server), got success=%f error=%f", fastOk, fastErr)
	}

	mediumOk := getCounterValue(t, m, "forseti_collector_fetches_total", map[string]string{"target": "medium-to", "status": "success"})
	if mediumOk != 1 {
		t.Errorf("medium-to fetch = %f, want 1 (300ms budget for 150ms server)", mediumOk)
	}

	slowOk := getCounterValue(t, m, "forseti_collector_fetches_total", map[string]string{"target": "slow-to", "status": "success"})
	if slowOk != 1 {
		t.Errorf("slow-to fetch = %f, want 1 (5s budget for 150ms server)", slowOk)
	}
}
