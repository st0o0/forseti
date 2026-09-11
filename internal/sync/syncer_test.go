package sync

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/st0o0/forseti/internal/config"
	"github.com/st0o0/forseti/internal/metrics"
	"github.com/st0o0/forseti/internal/pihole"
	"github.com/st0o0/forseti/internal/session"
)

func newSyncTestServer(groups []pihole.APIGroup, adlists []pihole.APIList, createCount *atomic.Int32) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/auth" && r.Method == http.MethodPost:
			json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]string{"sid": "test-sid"},
			})
		case r.URL.Path == "/api/auth" && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/api/groups" && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(map[string]any{"groups": groups})
		case r.URL.Path == "/api/groups" && r.Method == http.MethodPost:
			createCount.Add(1)
			json.NewEncoder(w).Encode(map[string]any{
				"group": map[string]any{"id": 99, "name": "synced"},
			})
		case r.URL.Path == "/api/lists" && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(map[string]any{"lists": adlists})
		case r.URL.Path == "/api/lists" && r.Method == http.MethodPost:
			createCount.Add(1)
			json.NewEncoder(w).Encode(map[string]any{"lists": []any{}})
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
}

func TestSyncGroupsAddsFromPrimary(t *testing.T) {
	primaryGroups := []pihole.APIGroup{
		{ID: 1, Name: "default"},
		{ID: 2, Name: "adblock"},
	}
	replicaGroups := []pihole.APIGroup{
		{ID: 1, Name: "default"},
	}

	var createCount atomic.Int32
	primarySrv := newSyncTestServer(primaryGroups, nil, &createCount)
	defer primarySrv.Close()
	replicaSrv := newSyncTestServer(replicaGroups, nil, &createCount)
	defer replicaSrv.Close()

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
			{Name: "replica", URL: replicaSrv.URL, Password: "pw", Role: "replica"},
		},
	}

	m := metrics.NewServer(0, "/metrics")
	syncer := NewSyncer(pool, cfg, m)
	syncer.SyncAll()

	if got := createCount.Load(); got != 1 {
		t.Errorf("create calls = %d, want 1 (adblock group missing on replica)", got)
	}
}

func TestSyncAdlistsAddsFromPrimary(t *testing.T) {
	primaryAdlists := []pihole.APIList{
		{ID: 1, Address: "https://example.com/list1.txt"},
		{ID: 2, Address: "https://example.com/list2.txt"},
	}
	replicaAdlists := []pihole.APIList{
		{ID: 1, Address: "https://example.com/list1.txt"},
	}

	var createCount atomic.Int32
	primarySrv := newSyncTestServer(nil, primaryAdlists, &createCount)
	defer primarySrv.Close()
	replicaSrv := newSyncTestServer(nil, replicaAdlists, &createCount)
	defer replicaSrv.Close()

	pool := session.NewPool()
	defer pool.Close()

	cfg := &config.Config{
		Mode: config.ModeSync,
		Sync: config.SyncConfig{
			Primary:   "primary",
			Resources: []string{"adlists"},
		},
		Targets: []config.Target{
			{Name: "primary", URL: primarySrv.URL, Password: "pw", Role: "primary"},
			{Name: "replica", URL: replicaSrv.URL, Password: "pw", Role: "replica"},
		},
	}

	m := metrics.NewServer(0, "/metrics")
	syncer := NewSyncer(pool, cfg, m)
	syncer.SyncAll()

	if got := createCount.Load(); got != 1 {
		t.Errorf("create calls = %d, want 1 (list2 missing on replica)", got)
	}
}

func TestSyncSkipsNonMarkedForDelete(t *testing.T) {
	primaryGroups := []pihole.APIGroup{
		{ID: 1, Name: "default"},
	}
	replicaGroups := []pihole.APIGroup{
		{ID: 1, Name: "default"},
		{ID: 2, Name: "manual-group", Comment: "added by admin"},
	}

	var createCount atomic.Int32
	primarySrv := newSyncTestServer(primaryGroups, nil, &createCount)
	defer primarySrv.Close()
	replicaSrv := newSyncTestServer(replicaGroups, nil, &createCount)
	defer replicaSrv.Close()

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
			{Name: "replica", URL: replicaSrv.URL, Password: "pw", Role: "replica"},
		},
	}

	m := metrics.NewServer(0, "/metrics")
	syncer := NewSyncer(pool, cfg, m)
	syncer.SyncAll()

	if got := createCount.Load(); got != 0 {
		t.Errorf("create calls = %d, want 0 (nothing to add, manual group should be left alone)", got)
	}
}
