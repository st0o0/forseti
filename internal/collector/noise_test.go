package collector

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/st0o0/forseti/internal/config"
	"github.com/st0o0/forseti/internal/logging/logtest"
	"github.com/st0o0/forseti/internal/metrics"
	"github.com/st0o0/forseti/internal/session"
)

func TestCollectorDownTargetLoggedOnceAndNegativelyCached(t *testing.T) {
	rec := logtest.New(t)
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()
	target := config.Target{Name: "down", URL: srv.URL, Password: "pw"}
	m := metrics.NewServer(0, "/metrics", config.CollectorToggles{})
	coll := NewCollector(pool, []config.Target{target}, time.Hour, m, config.CollectorToggles{})

	for i := 0; i < 5; i++ {
		coll.Collect(context.Background())
	}

	if got := hits.Load(); got != 1 {
		t.Errorf("requests to down target = %d, want 1 (negative cache until TTL)", got)
	}
	if n := len(rec.Find("target unreachable")); n != 1 {
		t.Errorf("target unreachable events = %d, want 1", n)
	}
	if v := getCounterValue(t, m, "forseti_collector_cache_hits_total", map[string]string{"target": "down"}); v != 0 {
		t.Errorf("negative cache counted as cache hit: %v", v)
	}
}

func TestNewDomainLoggedOncePerWindow(t *testing.T) {
	rec := logtest.New(t)
	var calls atomic.Int32
	srv := newNewDomainsTestServer(t, &calls)
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()
	target := config.Target{Name: "pi", URL: srv.URL, Password: "pw"}
	m := metrics.NewServer(0, "/metrics", config.CollectorToggles{})
	coll := NewNewDomainsCollector(pool, []config.Target{target}, time.Minute, m, config.CollectorToggles{})

	coll.CollectOnce(context.Background())
	coll.CollectOnce(context.Background())

	if n := len(rec.Find("new_blocked_domain")); n != 1 {
		t.Errorf("new_blocked_domain events = %d, want 1", n)
	}
	families, _ := m.Gather()
	if v, _ := findGaugeValue(families, "forseti_new_blocked_domains_24h", "pi"); v != 1 {
		t.Errorf("gauge = %v, want 1 on the second run too", v)
	}
}
