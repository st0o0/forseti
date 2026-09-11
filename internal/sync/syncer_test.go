package sync

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/st0o0/forseti/internal/config"
	"github.com/st0o0/forseti/internal/metrics"
	"github.com/st0o0/forseti/internal/pihole"
	"github.com/st0o0/forseti/internal/session"
)

type testData struct {
	groups     []pihole.APIGroup
	adlists    []pihole.APIList
	deny       []pihole.APIDomain
	denyRegex  []pihole.APIDomain
	allow      []pihole.APIDomain
	allowRegex []pihole.APIDomain
	dns        []string // "ip domain" entries
	clients    []pihole.APIClient
}

type testCounters struct {
	creates atomic.Int32
	deletes atomic.Int32
}

func newFullTestServer(data testData, counters *testCounters) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/auth" && r.Method == http.MethodPost:
			json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]string{"sid": "test-sid"},
			})
		case r.URL.Path == "/api/auth" && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)

		// Groups
		case r.URL.Path == "/api/groups" && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(map[string]any{"groups": data.groups})
		case r.URL.Path == "/api/groups" && r.Method == http.MethodPost:
			counters.creates.Add(1)
			json.NewEncoder(w).Encode(map[string]any{
				"group": map[string]any{"id": 99, "name": "synced"},
			})
		case r.URL.Path == "/api/groups:batchDelete" && r.Method == http.MethodPost:
			counters.deletes.Add(1)

		// Adlists
		case r.URL.Path == "/api/lists" && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(map[string]any{"lists": data.adlists})
		case r.URL.Path == "/api/lists" && r.Method == http.MethodPost:
			counters.creates.Add(1)
			json.NewEncoder(w).Encode(map[string]any{"lists": []any{}})
		case r.URL.Path == "/api/lists:batchDelete" && r.Method == http.MethodPost:
			counters.deletes.Add(1)

		// Domains (deny & allow, exact & regex)
		case strings.HasPrefix(r.URL.Path, "/api/domains/") && r.Method == http.MethodGet:
			var domains []pihole.APIDomain
			switch {
			case strings.HasSuffix(r.URL.Path, "/deny/exact"):
				domains = data.deny
			case strings.HasSuffix(r.URL.Path, "/deny/regex"):
				domains = data.denyRegex
			case strings.HasSuffix(r.URL.Path, "/allow/exact"):
				domains = data.allow
			case strings.HasSuffix(r.URL.Path, "/allow/regex"):
				domains = data.allowRegex
			}
			json.NewEncoder(w).Encode(map[string]any{"domains": domains})
		case strings.HasPrefix(r.URL.Path, "/api/domains/") && r.Method == http.MethodPost:
			counters.creates.Add(1)
			json.NewEncoder(w).Encode(map[string]any{
				"domain": map[string]any{"id": 99},
			})
		case r.URL.Path == "/api/domains:batchDelete" && r.Method == http.MethodPost:
			counters.deletes.Add(1)

		// DNS records
		case r.URL.Path == "/api/config/dns/hosts" && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(map[string]any{
				"config": map[string]any{
					"dns": map[string]any{"hosts": data.dns},
				},
			})
		case strings.HasPrefix(r.URL.Path, "/api/config/dns/hosts/") && r.Method == http.MethodPut:
			counters.creates.Add(1)
		case strings.HasPrefix(r.URL.Path, "/api/config/dns/hosts/") && r.Method == http.MethodDelete:
			counters.deletes.Add(1)

		// Clients
		case r.URL.Path == "/api/clients" && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(map[string]any{"clients": data.clients})
		case r.URL.Path == "/api/clients" && r.Method == http.MethodPost:
			counters.creates.Add(1)
			json.NewEncoder(w).Encode(map[string]any{
				"client": map[string]any{"id": 99},
			})
		case r.URL.Path == "/api/clients:batchDelete" && r.Method == http.MethodPost:
			counters.deletes.Add(1)

		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
}

func newErrorServer(failPath string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/auth" && r.Method == http.MethodPost {
			json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]string{"sid": "test-sid"},
			})
			return
		}
		if r.URL.Path == "/api/auth" && r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if strings.Contains(r.URL.Path, failPath) || (failPath == "all" && r.URL.Path != "/api/auth") {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":{"key":"internal","message":"server error"}}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"groups": []any{}, "lists": []any{}, "domains": []any{},
			"clients": []any{},
			"config":  map[string]any{"dns": map[string]any{"hosts": []any{}}},
		})
	}))
}

func newUnreachableServer() *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	return srv
}

func makeSyncConfig(primaryURL, replicaURL string, resources []string) *config.Config {
	return &config.Config{
		Mode: config.ModeSync,
		Sync: config.SyncConfig{
			Primary:   "primary",
			Resources: resources,
		},
		Targets: []config.Target{
			{Name: "primary", URL: primaryURL, Password: "pw", Role: "primary"},
			{Name: "replica", URL: replicaURL, Password: "pw", Role: "replica"},
		},
	}
}

// === Group sync tests ===

func TestSyncGroupsAddsFromPrimary(t *testing.T) {
	primaryCounters := &testCounters{}
	primarySrv := newFullTestServer(testData{
		groups: []pihole.APIGroup{
			{ID: 1, Name: "default"},
			{ID: 2, Name: "adblock"},
		},
	}, primaryCounters)
	defer primarySrv.Close()

	replicaCounters := &testCounters{}
	replicaSrv := newFullTestServer(testData{
		groups: []pihole.APIGroup{
			{ID: 1, Name: "default"},
		},
	}, replicaCounters)
	defer replicaSrv.Close()

	pool := session.NewPool()
	defer pool.Close()

	cfg := makeSyncConfig(primarySrv.URL, replicaSrv.URL, []string{"groups"})
	m := metrics.NewServer(0, "/metrics")
	syncer := NewSyncer(pool, cfg, m)
	syncer.SyncAll()

	if got := replicaCounters.creates.Load(); got != 1 {
		t.Errorf("create calls = %d, want 1", got)
	}
}

func TestSyncGroupsDeletesMarked(t *testing.T) {
	primaryCounters := &testCounters{}
	primarySrv := newFullTestServer(testData{
		groups: []pihole.APIGroup{{ID: 1, Name: "default"}},
	}, primaryCounters)
	defer primarySrv.Close()

	replicaCounters := &testCounters{}
	replicaSrv := newFullTestServer(testData{
		groups: []pihole.APIGroup{
			{ID: 1, Name: "default"},
			{ID: 2, Name: "old-group", Comment: "[forseti-sync] managed"},
		},
	}, replicaCounters)
	defer replicaSrv.Close()

	pool := session.NewPool()
	defer pool.Close()

	cfg := makeSyncConfig(primarySrv.URL, replicaSrv.URL, []string{"groups"})
	m := metrics.NewServer(0, "/metrics")
	syncer := NewSyncer(pool, cfg, m)
	syncer.SyncAll()

	if got := replicaCounters.deletes.Load(); got != 1 {
		t.Errorf("delete calls = %d, want 1", got)
	}
}

func TestSyncSkipsNonMarkedForDelete(t *testing.T) {
	primaryCounters := &testCounters{}
	primarySrv := newFullTestServer(testData{
		groups: []pihole.APIGroup{{ID: 1, Name: "default"}},
	}, primaryCounters)
	defer primarySrv.Close()

	replicaCounters := &testCounters{}
	replicaSrv := newFullTestServer(testData{
		groups: []pihole.APIGroup{
			{ID: 1, Name: "default"},
			{ID: 2, Name: "manual-group", Comment: "added by admin"},
		},
	}, replicaCounters)
	defer replicaSrv.Close()

	pool := session.NewPool()
	defer pool.Close()

	cfg := makeSyncConfig(primarySrv.URL, replicaSrv.URL, []string{"groups"})
	m := metrics.NewServer(0, "/metrics")
	syncer := NewSyncer(pool, cfg, m)
	syncer.SyncAll()

	if got := replicaCounters.creates.Load(); got != 0 {
		t.Errorf("create calls = %d, want 0", got)
	}
	if got := replicaCounters.deletes.Load(); got != 0 {
		t.Errorf("delete calls = %d, want 0 (manual group should be left alone)", got)
	}
}

func newCreateFailServer(data testData) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/auth" && r.Method == http.MethodPost:
			json.NewEncoder(w).Encode(map[string]any{"session": map[string]string{"sid": "test-sid"}})
		case r.URL.Path == "/api/auth" && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet:
			switch {
			case r.URL.Path == "/api/groups":
				json.NewEncoder(w).Encode(map[string]any{"groups": data.groups})
			case r.URL.Path == "/api/lists":
				json.NewEncoder(w).Encode(map[string]any{"lists": data.adlists})
			case strings.HasPrefix(r.URL.Path, "/api/domains/"):
				var domains []pihole.APIDomain
				switch {
				case strings.HasSuffix(r.URL.Path, "/deny/exact"):
					domains = data.deny
				case strings.HasSuffix(r.URL.Path, "/deny/regex"):
					domains = data.denyRegex
				case strings.HasSuffix(r.URL.Path, "/allow/exact"):
					domains = data.allow
				case strings.HasSuffix(r.URL.Path, "/allow/regex"):
					domains = data.allowRegex
				}
				json.NewEncoder(w).Encode(map[string]any{"domains": domains})
			case r.URL.Path == "/api/config/dns/hosts":
				json.NewEncoder(w).Encode(map[string]any{"config": map[string]any{"dns": map[string]any{"hosts": data.dns}}})
			case r.URL.Path == "/api/clients":
				json.NewEncoder(w).Encode(map[string]any{"clients": data.clients})
			default:
				json.NewEncoder(w).Encode(map[string]any{})
			}
		default:
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":{"key":"internal","message":"create failed"}}`))
		}
	}))
}

func TestSyncGroupsCreateError(t *testing.T) {
	primaryCounters := &testCounters{}
	primarySrv := newFullTestServer(testData{
		groups: []pihole.APIGroup{{ID: 1, Name: "new-group"}},
	}, primaryCounters)
	defer primarySrv.Close()

	replicaSrv := newCreateFailServer(testData{groups: []pihole.APIGroup{}})
	defer replicaSrv.Close()

	pool := session.NewPool()
	defer pool.Close()

	cfg := makeSyncConfig(primarySrv.URL, replicaSrv.URL, []string{"groups"})
	m := metrics.NewServer(0, "/metrics")
	syncer := NewSyncer(pool, cfg, m)
	syncer.SyncAll()
}

func TestSyncGroupsDeleteError(t *testing.T) {
	primaryCounters := &testCounters{}
	primarySrv := newFullTestServer(testData{
		groups: []pihole.APIGroup{},
	}, primaryCounters)
	defer primarySrv.Close()

	replicaSrv := newErrorServer("groups:batchDelete")
	defer replicaSrv.Close()

	pool := session.NewPool()
	defer pool.Close()

	cfg := makeSyncConfig(primarySrv.URL, replicaSrv.URL, []string{"groups"})
	m := metrics.NewServer(0, "/metrics")

	replica, _ := pool.Get(cfg.Targets[1])
	s := &Syncer{pool: pool, cfg: cfg, metrics: m, marker: "[forseti-sync]"}
	s.syncGroups(replica, []pihole.APIGroup{}, []pihole.APIGroup{
		{ID: 1, Name: "stale", Comment: "[forseti-sync]"},
	})
}

// === Adlist sync tests ===

func TestSyncAdlistsAddsFromPrimary(t *testing.T) {
	primaryCounters := &testCounters{}
	primarySrv := newFullTestServer(testData{
		adlists: []pihole.APIList{
			{ID: 1, Address: "https://example.com/list1.txt"},
			{ID: 2, Address: "https://example.com/list2.txt"},
		},
	}, primaryCounters)
	defer primarySrv.Close()

	replicaCounters := &testCounters{}
	replicaSrv := newFullTestServer(testData{
		adlists: []pihole.APIList{
			{ID: 1, Address: "https://example.com/list1.txt"},
		},
	}, replicaCounters)
	defer replicaSrv.Close()

	pool := session.NewPool()
	defer pool.Close()

	cfg := makeSyncConfig(primarySrv.URL, replicaSrv.URL, []string{"adlists"})
	m := metrics.NewServer(0, "/metrics")
	syncer := NewSyncer(pool, cfg, m)
	syncer.SyncAll()

	if got := replicaCounters.creates.Load(); got != 1 {
		t.Errorf("create calls = %d, want 1", got)
	}
}

func TestSyncAdlistsDeletesMarked(t *testing.T) {
	primaryCounters := &testCounters{}
	primarySrv := newFullTestServer(testData{
		adlists: []pihole.APIList{},
	}, primaryCounters)
	defer primarySrv.Close()

	replicaCounters := &testCounters{}
	replicaSrv := newFullTestServer(testData{
		adlists: []pihole.APIList{
			{ID: 1, Address: "https://stale.com/list.txt", Comment: "[forseti-sync]"},
		},
	}, replicaCounters)
	defer replicaSrv.Close()

	pool := session.NewPool()
	defer pool.Close()

	cfg := makeSyncConfig(primarySrv.URL, replicaSrv.URL, []string{"adlists"})
	m := metrics.NewServer(0, "/metrics")
	syncer := NewSyncer(pool, cfg, m)
	syncer.SyncAll()

	if got := replicaCounters.deletes.Load(); got != 1 {
		t.Errorf("delete calls = %d, want 1", got)
	}
}

func TestSyncAdlistsCreateError(t *testing.T) {
	primaryCounters := &testCounters{}
	primarySrv := newFullTestServer(testData{
		adlists: []pihole.APIList{{ID: 1, Address: "https://new.com/list.txt"}},
	}, primaryCounters)
	defer primarySrv.Close()

	replicaSrv := newCreateFailServer(testData{adlists: []pihole.APIList{}})
	defer replicaSrv.Close()

	pool := session.NewPool()
	defer pool.Close()

	cfg := makeSyncConfig(primarySrv.URL, replicaSrv.URL, []string{"adlists"})
	m := metrics.NewServer(0, "/metrics")
	syncer := NewSyncer(pool, cfg, m)
	syncer.SyncAll()
}

func TestSyncAdlistsDeleteError(t *testing.T) {
	primaryCounters := &testCounters{}
	primarySrv := newFullTestServer(testData{
		adlists: []pihole.APIList{},
	}, primaryCounters)
	defer primarySrv.Close()

	pool := session.NewPool()
	defer pool.Close()

	replicaSrv := newErrorServer("lists:batchDelete")
	defer replicaSrv.Close()

	cfg := makeSyncConfig(primarySrv.URL, replicaSrv.URL, []string{"adlists"})
	m := metrics.NewServer(0, "/metrics")

	replica, _ := pool.Get(cfg.Targets[1])
	s := &Syncer{pool: pool, cfg: cfg, metrics: m, marker: "[forseti-sync]"}
	s.syncAdlists(replica, []pihole.APIList{}, []pihole.APIList{
		{ID: 1, Address: "https://stale.com/list.txt", Comment: "[forseti-sync]"},
	})
}

// === Domain sync tests (deny & allow) ===

func TestSyncDomainsAdds(t *testing.T) {
	primaryCounters := &testCounters{}
	primarySrv := newFullTestServer(testData{
		deny: []pihole.APIDomain{
			{ID: 1, Domain: "ads.example.com", Enabled: true},
			{ID: 2, Domain: "tracker.example.com", Enabled: true},
		},
	}, primaryCounters)
	defer primarySrv.Close()

	replicaCounters := &testCounters{}
	replicaSrv := newFullTestServer(testData{
		deny: []pihole.APIDomain{
			{ID: 1, Domain: "ads.example.com"},
		},
	}, replicaCounters)
	defer replicaSrv.Close()

	pool := session.NewPool()
	defer pool.Close()

	cfg := makeSyncConfig(primarySrv.URL, replicaSrv.URL, []string{"deny"})
	m := metrics.NewServer(0, "/metrics")
	syncer := NewSyncer(pool, cfg, m)
	syncer.SyncAll()

	if got := replicaCounters.creates.Load(); got != 1 {
		t.Errorf("create calls = %d, want 1", got)
	}
}

func TestSyncDomainsDeletesMarked(t *testing.T) {
	primaryCounters := &testCounters{}
	primarySrv := newFullTestServer(testData{
		deny: []pihole.APIDomain{},
	}, primaryCounters)
	defer primarySrv.Close()

	replicaCounters := &testCounters{}
	replicaSrv := newFullTestServer(testData{
		deny: []pihole.APIDomain{
			{ID: 1, Domain: "stale.example.com", Comment: "[forseti-sync] managed"},
		},
	}, replicaCounters)
	defer replicaSrv.Close()

	pool := session.NewPool()
	defer pool.Close()

	cfg := makeSyncConfig(primarySrv.URL, replicaSrv.URL, []string{"deny"})
	m := metrics.NewServer(0, "/metrics")
	syncer := NewSyncer(pool, cfg, m)
	syncer.SyncAll()

	if got := replicaCounters.deletes.Load(); got != 1 {
		t.Errorf("delete calls = %d, want 1", got)
	}
}

func TestSyncDomainsSkipsNonMarked(t *testing.T) {
	primaryCounters := &testCounters{}
	primarySrv := newFullTestServer(testData{
		deny: []pihole.APIDomain{},
	}, primaryCounters)
	defer primarySrv.Close()

	replicaCounters := &testCounters{}
	replicaSrv := newFullTestServer(testData{
		deny: []pihole.APIDomain{
			{ID: 1, Domain: "manual.example.com", Comment: "user added"},
		},
	}, replicaCounters)
	defer replicaSrv.Close()

	pool := session.NewPool()
	defer pool.Close()

	cfg := makeSyncConfig(primarySrv.URL, replicaSrv.URL, []string{"deny"})
	m := metrics.NewServer(0, "/metrics")
	syncer := NewSyncer(pool, cfg, m)
	syncer.SyncAll()

	if got := replicaCounters.deletes.Load(); got != 0 {
		t.Errorf("delete calls = %d, want 0", got)
	}
}

func TestSyncAllowDomains(t *testing.T) {
	primaryCounters := &testCounters{}
	primarySrv := newFullTestServer(testData{
		allow: []pihole.APIDomain{
			{ID: 1, Domain: "safe.example.com", Enabled: true},
		},
	}, primaryCounters)
	defer primarySrv.Close()

	replicaCounters := &testCounters{}
	replicaSrv := newFullTestServer(testData{
		allow: []pihole.APIDomain{},
	}, replicaCounters)
	defer replicaSrv.Close()

	pool := session.NewPool()
	defer pool.Close()

	cfg := makeSyncConfig(primarySrv.URL, replicaSrv.URL, []string{"allow"})
	m := metrics.NewServer(0, "/metrics")
	syncer := NewSyncer(pool, cfg, m)
	syncer.SyncAll()

	if got := replicaCounters.creates.Load(); got != 1 {
		t.Errorf("create calls = %d, want 1", got)
	}
}

func TestSyncDomainsCreateError(t *testing.T) {
	primaryCounters := &testCounters{}
	primarySrv := newFullTestServer(testData{
		deny: []pihole.APIDomain{{ID: 1, Domain: "new.example.com", Enabled: true}},
	}, primaryCounters)
	defer primarySrv.Close()

	replicaSrv := newCreateFailServer(testData{deny: []pihole.APIDomain{}})
	defer replicaSrv.Close()

	pool := session.NewPool()
	defer pool.Close()

	cfg := makeSyncConfig(primarySrv.URL, replicaSrv.URL, []string{"deny"})
	m := metrics.NewServer(0, "/metrics")
	syncer := NewSyncer(pool, cfg, m)
	syncer.SyncAll()
}

func TestSyncDomainsDeleteError(t *testing.T) {
	pool := session.NewPool()
	defer pool.Close()

	replicaSrv := newErrorServer("domains:batchDelete")
	defer replicaSrv.Close()

	primaryCounters := &testCounters{}
	primarySrv := newFullTestServer(testData{}, primaryCounters)
	defer primarySrv.Close()

	cfg := makeSyncConfig(primarySrv.URL, replicaSrv.URL, []string{"deny"})
	m := metrics.NewServer(0, "/metrics")

	replica, _ := pool.Get(cfg.Targets[1])
	s := &Syncer{pool: pool, cfg: cfg, metrics: m, marker: "[forseti-sync]"}
	s.syncDomains(replica, "deny", "exact", []pihole.APIDomain{}, []pihole.APIDomain{
		{ID: 1, Domain: "stale.example.com", Comment: "[forseti-sync]"},
	})
}

// === DNS sync tests ===

func TestSyncDNSAdds(t *testing.T) {
	primaryCounters := &testCounters{}
	primarySrv := newFullTestServer(testData{
		dns: []string{"192.168.1.1 router.local", "192.168.1.2 nas.local"},
	}, primaryCounters)
	defer primarySrv.Close()

	replicaCounters := &testCounters{}
	replicaSrv := newFullTestServer(testData{
		dns: []string{"192.168.1.1 router.local"},
	}, replicaCounters)
	defer replicaSrv.Close()

	pool := session.NewPool()
	defer pool.Close()

	cfg := makeSyncConfig(primarySrv.URL, replicaSrv.URL, []string{"local_dns"})
	m := metrics.NewServer(0, "/metrics")
	syncer := NewSyncer(pool, cfg, m)
	syncer.SyncAll()

	if got := replicaCounters.creates.Load(); got != 1 {
		t.Errorf("DNS add calls = %d, want 1", got)
	}
}

func TestSyncDNSDeletes(t *testing.T) {
	primaryCounters := &testCounters{}
	primarySrv := newFullTestServer(testData{
		dns: []string{},
	}, primaryCounters)
	defer primarySrv.Close()

	replicaCounters := &testCounters{}
	replicaSrv := newFullTestServer(testData{
		dns: []string{"192.168.1.99 old.local"},
	}, replicaCounters)
	defer replicaSrv.Close()

	pool := session.NewPool()
	defer pool.Close()

	cfg := makeSyncConfig(primarySrv.URL, replicaSrv.URL, []string{"local_dns"})
	m := metrics.NewServer(0, "/metrics")
	syncer := NewSyncer(pool, cfg, m)
	syncer.SyncAll()

	if got := replicaCounters.deletes.Load(); got != 1 {
		t.Errorf("DNS delete calls = %d, want 1", got)
	}
}

func TestSyncDNSAddError(t *testing.T) {
	primaryCounters := &testCounters{}
	primarySrv := newFullTestServer(testData{
		dns: []string{"192.168.1.5 new.local"},
	}, primaryCounters)
	defer primarySrv.Close()

	replicaSrv := newCreateFailServer(testData{dns: []string{}})
	defer replicaSrv.Close()

	pool := session.NewPool()
	defer pool.Close()

	cfg := makeSyncConfig(primarySrv.URL, replicaSrv.URL, []string{"local_dns"})
	m := metrics.NewServer(0, "/metrics")
	syncer := NewSyncer(pool, cfg, m)
	syncer.SyncAll()
}

func TestSyncDNSDeleteError(t *testing.T) {
	pool := session.NewPool()
	defer pool.Close()

	replicaSrv := newErrorServer("config/dns/hosts")
	defer replicaSrv.Close()

	primaryCounters := &testCounters{}
	primarySrv := newFullTestServer(testData{}, primaryCounters)
	defer primarySrv.Close()

	cfg := makeSyncConfig(primarySrv.URL, replicaSrv.URL, []string{"local_dns"})
	m := metrics.NewServer(0, "/metrics")

	replica, _ := pool.Get(cfg.Targets[1])
	s := &Syncer{pool: pool, cfg: cfg, metrics: m, marker: "[forseti-sync]"}
	s.syncDNS(replica, []pihole.APIDNSRecord{}, []pihole.APIDNSRecord{
		{IP: "192.168.1.99", Domain: "old.local"},
	})
}

// === Client sync tests ===

func TestSyncClientsAdds(t *testing.T) {
	primaryCounters := &testCounters{}
	primarySrv := newFullTestServer(testData{
		clients: []pihole.APIClient{
			{ID: 1, Client: "192.168.1.100", Comment: "laptop"},
			{ID: 2, Client: "192.168.1.101", Comment: "phone"},
		},
	}, primaryCounters)
	defer primarySrv.Close()

	replicaCounters := &testCounters{}
	replicaSrv := newFullTestServer(testData{
		clients: []pihole.APIClient{
			{ID: 1, Client: "192.168.1.100", Comment: "laptop"},
		},
	}, replicaCounters)
	defer replicaSrv.Close()

	pool := session.NewPool()
	defer pool.Close()

	cfg := makeSyncConfig(primarySrv.URL, replicaSrv.URL, []string{"clients"})
	m := metrics.NewServer(0, "/metrics")
	syncer := NewSyncer(pool, cfg, m)
	syncer.SyncAll()

	if got := replicaCounters.creates.Load(); got != 1 {
		t.Errorf("create calls = %d, want 1", got)
	}
}

func TestSyncClientsDeletesMarked(t *testing.T) {
	primaryCounters := &testCounters{}
	primarySrv := newFullTestServer(testData{
		clients: []pihole.APIClient{},
	}, primaryCounters)
	defer primarySrv.Close()

	replicaCounters := &testCounters{}
	replicaSrv := newFullTestServer(testData{
		clients: []pihole.APIClient{
			{ID: 1, Client: "192.168.1.200", Comment: "[forseti-sync] managed"},
		},
	}, replicaCounters)
	defer replicaSrv.Close()

	pool := session.NewPool()
	defer pool.Close()

	cfg := makeSyncConfig(primarySrv.URL, replicaSrv.URL, []string{"clients"})
	m := metrics.NewServer(0, "/metrics")
	syncer := NewSyncer(pool, cfg, m)
	syncer.SyncAll()

	if got := replicaCounters.deletes.Load(); got != 1 {
		t.Errorf("delete calls = %d, want 1", got)
	}
}

func TestSyncClientsSkipsNonMarked(t *testing.T) {
	primaryCounters := &testCounters{}
	primarySrv := newFullTestServer(testData{
		clients: []pihole.APIClient{},
	}, primaryCounters)
	defer primarySrv.Close()

	replicaCounters := &testCounters{}
	replicaSrv := newFullTestServer(testData{
		clients: []pihole.APIClient{
			{ID: 1, Client: "192.168.1.200", Comment: "manual"},
		},
	}, replicaCounters)
	defer replicaSrv.Close()

	pool := session.NewPool()
	defer pool.Close()

	cfg := makeSyncConfig(primarySrv.URL, replicaSrv.URL, []string{"clients"})
	m := metrics.NewServer(0, "/metrics")
	syncer := NewSyncer(pool, cfg, m)
	syncer.SyncAll()

	if got := replicaCounters.deletes.Load(); got != 0 {
		t.Errorf("delete calls = %d, want 0", got)
	}
}

func TestSyncClientsCreateError(t *testing.T) {
	primaryCounters := &testCounters{}
	primarySrv := newFullTestServer(testData{
		clients: []pihole.APIClient{{ID: 1, Client: "192.168.1.5"}},
	}, primaryCounters)
	defer primarySrv.Close()

	replicaSrv := newCreateFailServer(testData{clients: []pihole.APIClient{}})
	defer replicaSrv.Close()

	pool := session.NewPool()
	defer pool.Close()

	cfg := makeSyncConfig(primarySrv.URL, replicaSrv.URL, []string{"clients"})
	m := metrics.NewServer(0, "/metrics")
	syncer := NewSyncer(pool, cfg, m)
	syncer.SyncAll()
}

func TestSyncClientsDeleteError(t *testing.T) {
	pool := session.NewPool()
	defer pool.Close()

	replicaSrv := newErrorServer("clients:batchDelete")
	defer replicaSrv.Close()

	primaryCounters := &testCounters{}
	primarySrv := newFullTestServer(testData{}, primaryCounters)
	defer primarySrv.Close()

	cfg := makeSyncConfig(primarySrv.URL, replicaSrv.URL, []string{"clients"})
	m := metrics.NewServer(0, "/metrics")

	replica, _ := pool.Get(cfg.Targets[1])
	s := &Syncer{pool: pool, cfg: cfg, metrics: m, marker: "[forseti-sync]"}
	s.syncClients(replica, []pihole.APIClient{}, []pihole.APIClient{
		{ID: 1, Client: "192.168.1.200", Comment: "[forseti-sync]"},
	})
}

// === SyncAll / getPrimary / load error paths ===

func TestSyncAllNoChanges(t *testing.T) {
	primaryCounters := &testCounters{}
	primarySrv := newFullTestServer(testData{
		groups: []pihole.APIGroup{{ID: 1, Name: "default"}},
	}, primaryCounters)
	defer primarySrv.Close()

	replicaCounters := &testCounters{}
	replicaSrv := newFullTestServer(testData{
		groups: []pihole.APIGroup{{ID: 1, Name: "default"}},
	}, replicaCounters)
	defer replicaSrv.Close()

	pool := session.NewPool()
	defer pool.Close()

	cfg := makeSyncConfig(primarySrv.URL, replicaSrv.URL, []string{"groups"})
	m := metrics.NewServer(0, "/metrics")
	syncer := NewSyncer(pool, cfg, m)
	syncer.SyncAll()

	if got := replicaCounters.creates.Load(); got != 0 {
		t.Errorf("creates = %d, want 0", got)
	}
	if got := replicaCounters.deletes.Load(); got != 0 {
		t.Errorf("deletes = %d, want 0", got)
	}
}

func TestSyncAllMultipleResources(t *testing.T) {
	primaryCounters := &testCounters{}
	primarySrv := newFullTestServer(testData{
		groups:  []pihole.APIGroup{{ID: 1, Name: "default"}, {ID: 2, Name: "extra"}},
		adlists: []pihole.APIList{{ID: 1, Address: "https://list.com/a.txt"}},
		deny:    []pihole.APIDomain{{ID: 1, Domain: "bad.com", Enabled: true}},
		allow:   []pihole.APIDomain{{ID: 1, Domain: "good.com", Enabled: true}},
		dns:     []string{"192.168.1.1 router.local"},
		clients: []pihole.APIClient{{ID: 1, Client: "192.168.1.100"}},
	}, primaryCounters)
	defer primarySrv.Close()

	replicaCounters := &testCounters{}
	replicaSrv := newFullTestServer(testData{}, replicaCounters)
	defer replicaSrv.Close()

	pool := session.NewPool()
	defer pool.Close()

	cfg := makeSyncConfig(primarySrv.URL, replicaSrv.URL,
		[]string{"groups", "adlists", "deny", "allow", "local_dns", "clients"})
	m := metrics.NewServer(0, "/metrics")
	syncer := NewSyncer(pool, cfg, m)
	syncer.SyncAll()

	if got := replicaCounters.creates.Load(); got != 7 {
		t.Errorf("total creates = %d, want 7 (2 groups + 1 adlist + 1 deny + 1 allow + 1 dns + 1 client)", got)
	}
}

func TestGetPrimaryNotFound(t *testing.T) {
	pool := session.NewPool()
	defer pool.Close()

	cfg := &config.Config{
		Mode: config.ModeSync,
		Sync: config.SyncConfig{
			Primary:   "nonexistent",
			Resources: []string{"groups"},
		},
		Targets: []config.Target{
			{Name: "other", URL: "http://localhost:1", Password: "pw", Role: "replica"},
		},
	}

	m := metrics.NewServer(0, "/metrics")
	syncer := NewSyncer(pool, cfg, m)
	syncer.SyncAll()
}

func TestSyncAllPrimaryUnreachable(t *testing.T) {
	srv := newUnreachableServer()
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	cfg := &config.Config{
		Mode: config.ModeSync,
		Sync: config.SyncConfig{
			Primary:   "primary",
			Resources: []string{"groups"},
		},
		Targets: []config.Target{
			{Name: "primary", URL: srv.URL, Password: "pw", Role: "primary"},
		},
	}

	m := metrics.NewServer(0, "/metrics")
	syncer := NewSyncer(pool, cfg, m)
	syncer.SyncAll()
}

func TestSyncAllPrimaryLoadError(t *testing.T) {
	primarySrv := newErrorServer("groups")
	defer primarySrv.Close()

	replicaCounters := &testCounters{}
	replicaSrv := newFullTestServer(testData{}, replicaCounters)
	defer replicaSrv.Close()

	pool := session.NewPool()
	defer pool.Close()

	cfg := makeSyncConfig(primarySrv.URL, replicaSrv.URL, []string{"groups"})
	m := metrics.NewServer(0, "/metrics")
	syncer := NewSyncer(pool, cfg, m)
	syncer.SyncAll()
}

func TestSyncReplicaSessionError(t *testing.T) {
	primaryCounters := &testCounters{}
	primarySrv := newFullTestServer(testData{
		groups: []pihole.APIGroup{{ID: 1, Name: "default"}},
	}, primaryCounters)
	defer primarySrv.Close()

	replicaSrv := newUnreachableServer()
	defer replicaSrv.Close()

	pool := session.NewPool()
	defer pool.Close()

	cfg := makeSyncConfig(primarySrv.URL, replicaSrv.URL, []string{"groups"})
	m := metrics.NewServer(0, "/metrics")
	syncer := NewSyncer(pool, cfg, m)
	syncer.SyncAll()
}

func TestSyncReplicaLoadError(t *testing.T) {
	primaryCounters := &testCounters{}
	primarySrv := newFullTestServer(testData{
		groups: []pihole.APIGroup{{ID: 1, Name: "default"}},
	}, primaryCounters)
	defer primarySrv.Close()

	replicaSrv := newErrorServer("groups")
	defer replicaSrv.Close()

	pool := session.NewPool()
	defer pool.Close()

	cfg := makeSyncConfig(primarySrv.URL, replicaSrv.URL, []string{"groups"})
	m := metrics.NewServer(0, "/metrics")
	syncer := NewSyncer(pool, cfg, m)
	syncer.SyncAll()
}

// === load() error paths for each resource ===

func TestLoadGroupsError(t *testing.T) {
	srv := newErrorServer("groups")
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	target := config.Target{Name: "test", URL: srv.URL, Password: "pw"}
	client, _ := pool.Get(target)

	var st state
	err := st.load(client, map[string]bool{"groups": true})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "list groups") {
		t.Errorf("error = %q, want to contain 'list groups'", err.Error())
	}
}

func TestLoadAdlistsError(t *testing.T) {
	srv := newErrorServer("lists")
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	target := config.Target{Name: "test", URL: srv.URL, Password: "pw"}
	client, _ := pool.Get(target)

	var st state
	err := st.load(client, map[string]bool{"adlists": true})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "list adlists") {
		t.Errorf("error = %q, want to contain 'list adlists'", err.Error())
	}
}

func TestLoadDenyError(t *testing.T) {
	srv := newErrorServer("domains/deny")
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	target := config.Target{Name: "test", URL: srv.URL, Password: "pw"}
	client, _ := pool.Get(target)

	var st state
	err := st.load(client, map[string]bool{"deny": true})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "list deny") {
		t.Errorf("error = %q, want to contain 'list deny'", err.Error())
	}
}

func TestLoadAllowError(t *testing.T) {
	srv := newErrorServer("domains/allow")
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	target := config.Target{Name: "test", URL: srv.URL, Password: "pw"}
	client, _ := pool.Get(target)

	var st state
	err := st.load(client, map[string]bool{"allow": true})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "list allow") {
		t.Errorf("error = %q, want to contain 'list allow'", err.Error())
	}
}

func TestLoadDNSError(t *testing.T) {
	srv := newErrorServer("config/dns/hosts")
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	target := config.Target{Name: "test", URL: srv.URL, Password: "pw"}
	client, _ := pool.Get(target)

	var st state
	err := st.load(client, map[string]bool{"local_dns": true})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "list dns") {
		t.Errorf("error = %q, want to contain 'list dns'", err.Error())
	}
}

func TestLoadClientsError(t *testing.T) {
	srv := newErrorServer("clients")
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	target := config.Target{Name: "test", URL: srv.URL, Password: "pw"}
	client, _ := pool.Get(target)

	var st state
	err := st.load(client, map[string]bool{"clients": true})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "list clients") {
		t.Errorf("error = %q, want to contain 'list clients'", err.Error())
	}
}

func TestLoadAllResources(t *testing.T) {
	counters := &testCounters{}
	srv := newFullTestServer(testData{
		groups:  []pihole.APIGroup{{ID: 1, Name: "default"}},
		adlists: []pihole.APIList{{ID: 1, Address: "https://example.com/list.txt"}},
		deny:    []pihole.APIDomain{{ID: 1, Domain: "bad.com"}},
		allow:   []pihole.APIDomain{{ID: 1, Domain: "good.com"}},
		dns:     []string{"192.168.1.1 router.local"},
		clients: []pihole.APIClient{{ID: 1, Client: "192.168.1.100"}},
	}, counters)
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	target := config.Target{Name: "test", URL: srv.URL, Password: "pw"}
	client, err := pool.Get(target)
	if err != nil {
		t.Fatal(err)
	}

	var st state
	err = st.load(client, map[string]bool{
		"groups": true, "adlists": true, "deny": true,
		"allow": true, "local_dns": true, "clients": true,
	})
	if err != nil {
		t.Fatalf("load error: %v", err)
	}

	if len(st.groups) != 1 {
		t.Errorf("groups = %d, want 1", len(st.groups))
	}
	if len(st.adlists) != 1 {
		t.Errorf("adlists = %d, want 1", len(st.adlists))
	}
	if len(st.deny) != 1 {
		t.Errorf("deny = %d, want 1", len(st.deny))
	}
	if len(st.allow) != 1 {
		t.Errorf("allow = %d, want 1", len(st.allow))
	}
	if len(st.dns) != 1 {
		t.Errorf("dns = %d, want 1", len(st.dns))
	}
	if len(st.clients) != 1 {
		t.Errorf("clients = %d, want 1", len(st.clients))
	}
}

func TestSyncAllSkipsNonReplica(t *testing.T) {
	primaryCounters := &testCounters{}
	primarySrv := newFullTestServer(testData{
		groups: []pihole.APIGroup{{ID: 1, Name: "default"}, {ID: 2, Name: "extra"}},
	}, primaryCounters)
	defer primarySrv.Close()

	pool := session.NewPool()
	defer pool.Close()

	cfg := &config.Config{
		Mode: config.ModeSync,
		Sync: config.SyncConfig{
			Primary:   "primary",
			Resources: []string{"groups"},
		},
		Targets: []config.Target{
			{Name: "primary", URL: primarySrv.URL, Password: "pw", Role: "primary"},
		},
	}

	m := metrics.NewServer(0, "/metrics")
	syncer := NewSyncer(pool, cfg, m)
	syncer.SyncAll()
}

func TestSyncGroupsCaseInsensitive(t *testing.T) {
	primaryCounters := &testCounters{}
	primarySrv := newFullTestServer(testData{
		groups: []pihole.APIGroup{{ID: 1, Name: "MyGroup"}},
	}, primaryCounters)
	defer primarySrv.Close()

	replicaCounters := &testCounters{}
	replicaSrv := newFullTestServer(testData{
		groups: []pihole.APIGroup{{ID: 1, Name: "mygroup"}},
	}, replicaCounters)
	defer replicaSrv.Close()

	pool := session.NewPool()
	defer pool.Close()

	cfg := makeSyncConfig(primarySrv.URL, replicaSrv.URL, []string{"groups"})
	m := metrics.NewServer(0, "/metrics")
	syncer := NewSyncer(pool, cfg, m)
	syncer.SyncAll()

	if got := replicaCounters.creates.Load(); got != 0 {
		t.Errorf("creates = %d, want 0 (case-insensitive match)", got)
	}
}

func TestSyncRegexDomainPropagation(t *testing.T) {
	primaryCounters := &testCounters{}
	primarySrv := newFullTestServer(testData{
		denyRegex: []pihole.APIDomain{
			{ID: 1, Domain: "(^|\\.)ads\\.", Kind: "regex", Comment: "primary regex"},
		},
	}, primaryCounters)
	defer primarySrv.Close()

	replicaCounters := &testCounters{}
	replicaSrv := newFullTestServer(testData{}, replicaCounters)
	defer replicaSrv.Close()

	pool := session.NewPool()
	defer pool.Close()

	cfg := makeSyncConfig(primarySrv.URL, replicaSrv.URL, []string{"deny"})
	m := metrics.NewServer(0, "/metrics")
	syncer := NewSyncer(pool, cfg, m)
	syncer.SyncAll()

	if got := replicaCounters.creates.Load(); got != 1 {
		t.Errorf("creates = %d, want 1 (regex domain should be synced)", got)
	}
}

func TestSyncRegexAndExactIndependent(t *testing.T) {
	primaryCounters := &testCounters{}
	primarySrv := newFullTestServer(testData{
		deny: []pihole.APIDomain{
			{ID: 1, Domain: "ads.com", Kind: "exact"},
		},
		denyRegex: []pihole.APIDomain{
			{ID: 2, Domain: "(^|\\.)ads\\.", Kind: "regex"},
		},
	}, primaryCounters)
	defer primarySrv.Close()

	replicaCounters := &testCounters{}
	replicaSrv := newFullTestServer(testData{
		deny: []pihole.APIDomain{
			{ID: 10, Domain: "ads.com", Kind: "exact"},
		},
	}, replicaCounters)
	defer replicaSrv.Close()

	pool := session.NewPool()
	defer pool.Close()

	cfg := makeSyncConfig(primarySrv.URL, replicaSrv.URL, []string{"deny"})
	m := metrics.NewServer(0, "/metrics")
	syncer := NewSyncer(pool, cfg, m)
	syncer.SyncAll()

	if got := replicaCounters.creates.Load(); got != 1 {
		t.Errorf("creates = %d, want 1 (only regex should be created, exact already exists)", got)
	}
}
