package sync

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/st0o0/forseti/internal/config"
	"github.com/st0o0/forseti/internal/pihole"
	"github.com/st0o0/forseti/internal/run"
	"github.com/st0o0/forseti/internal/session"
)

type expansionCounters struct {
	allowlistCreates atomic.Int32
	allowlistDeletes atomic.Int32
	cnameCreates     atomic.Int32
	cnameDeletes     atomic.Int32
}

func newExpansionTestServer(allowlists []pihole.APIList, cnames []string, counters *expansionCounters) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/auth" && r.Method == http.MethodPost:
			_ = json.NewEncoder(w).Encode(map[string]any{"session": map[string]string{"sid": "test-sid"}})
		case r.URL.Path == "/api/auth" && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/api/info/ftl":
			_, _ = w.Write([]byte(ftlReady))

		// Allowlists
		case r.URL.Path == "/api/lists" && r.Method == http.MethodGet && r.URL.Query().Get("type") == "allow":
			_ = json.NewEncoder(w).Encode(map[string]any{"lists": allowlists})
		case r.URL.Path == "/api/lists" && r.Method == http.MethodPost && r.URL.Query().Get("type") == "allow":
			counters.allowlistCreates.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{"lists": []any{}})
		case r.URL.Path == "/api/lists:batchDelete" && r.Method == http.MethodPost && r.URL.Query().Get("type") == "allow":
			counters.allowlistDeletes.Add(1)

		// CNAME records
		case r.URL.Path == "/api/config/dns/cnameRecords" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"config": map[string]any{
					"dns": map[string]any{"cnameRecords": cnames},
				},
			})
		case strings.HasPrefix(r.URL.Path, "/api/config/dns/cnameRecords/") && r.Method == http.MethodPut:
			counters.cnameCreates.Add(1)
		case strings.HasPrefix(r.URL.Path, "/api/config/dns/cnameRecords/") && r.Method == http.MethodDelete:
			counters.cnameDeletes.Add(1)

		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
}

// === containsInvalidChars tests ===

func TestContainsInvalidChars(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"normal name", "my-group", false},
		{"empty string", "", false},
		{"with spaces", "my group", false},
		{"carriage return", "bad\rname", true},
		{"line feed", "bad\nname", true},
		{"null byte", "bad\x00name", true},
		{"CRLF pair", "bad\r\nname", true},
		{"only CR", "\r", true},
		{"only LF", "\n", true},
		{"only null", "\x00", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := containsInvalidChars(tt.in); got != tt.want {
				t.Errorf("containsInvalidChars(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// === syncGroups group name validation tests ===

func TestSyncGroupsSkipsInvalidNames(t *testing.T) {
	counters := &testCounters{}
	srv := newFullTestServer(testData{groups: []pihole.APIGroup{}}, counters)
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	target := config.Target{Name: "replica", URL: srv.URL, Password: "pw"}
	client, err := pool.Get(target)
	if err != nil {
		t.Fatal(err)
	}

	r := &ReplicaRunner{marker: "[forseti-sync]", log: slog.Default()}
	res := &run.Result{}

	primary := []pihole.APIGroup{
		{ID: 1, Name: "valid-group"},
		{ID: 2, Name: "bad\ngroup"},
		{ID: 3, Name: "bad\rgroup"},
		{ID: 4, Name: "bad\x00group"},
	}
	replica := []pihole.APIGroup{}

	r.syncGroups(client, res, primary, replica)

	if got := counters.creates.Load(); got != 1 {
		t.Errorf("creates = %d, want 1 (only valid-group should be synced)", got)
	}
}

// === syncAllowlists tests ===

func TestSyncAllowlistsAddsFromPrimary(t *testing.T) {
	counters := &expansionCounters{}
	srv := newExpansionTestServer(nil, nil, counters)
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	target := config.Target{Name: "replica", URL: srv.URL, Password: "pw"}
	client, err := pool.Get(target)
	if err != nil {
		t.Fatal(err)
	}

	r := &ReplicaRunner{marker: "[forseti-sync]", log: slog.Default()}
	res := &run.Result{}

	primaryIDToName := map[int]string{1: "default", 2: "kids"}
	replicaNameToID := map[string]int{"default": 10, "kids": 20}

	primary := []pihole.APIList{
		{ID: 1, Address: "https://allow.example.com/list.txt", Groups: []int{1, 2}},
		{ID: 2, Address: "https://allow.example.com/list2.txt", Groups: []int{1}},
	}
	replica := []pihole.APIList{
		{ID: 10, Address: "https://allow.example.com/list.txt"},
	}

	r.syncAllowlists(client, res, primary, replica, primaryIDToName, replicaNameToID)

	if got := counters.allowlistCreates.Load(); got != 1 {
		t.Errorf("creates = %d, want 1 (only list2 should be added)", got)
	}
}

func TestSyncAllowlistsDeletesMarked(t *testing.T) {
	counters := &expansionCounters{}
	srv := newExpansionTestServer(nil, nil, counters)
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	target := config.Target{Name: "replica", URL: srv.URL, Password: "pw"}
	client, err := pool.Get(target)
	if err != nil {
		t.Fatal(err)
	}

	r := &ReplicaRunner{marker: "[forseti-sync]", log: slog.Default()}
	res := &run.Result{}

	primary := []pihole.APIList{}
	replica := []pihole.APIList{
		{ID: 1, Address: "https://orphan.com/list.txt", Comment: "[forseti-sync] managed"},
	}

	r.syncAllowlists(client, res, primary, replica, nil, nil)

	if got := counters.allowlistDeletes.Load(); got != 1 {
		t.Errorf("deletes = %d, want 1", got)
	}
}

func TestSyncAllowlistsPreservesManual(t *testing.T) {
	counters := &expansionCounters{}
	srv := newExpansionTestServer(nil, nil, counters)
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	target := config.Target{Name: "replica", URL: srv.URL, Password: "pw"}
	client, err := pool.Get(target)
	if err != nil {
		t.Fatal(err)
	}

	r := &ReplicaRunner{marker: "[forseti-sync]", log: slog.Default()}
	res := &run.Result{}

	primary := []pihole.APIList{}
	replica := []pihole.APIList{
		{ID: 1, Address: "https://manual.com/list.txt", Comment: "added by admin"},
	}

	r.syncAllowlists(client, res, primary, replica, nil, nil)

	if got := counters.allowlistCreates.Load(); got != 0 {
		t.Errorf("creates = %d, want 0", got)
	}
	if got := counters.allowlistDeletes.Load(); got != 0 {
		t.Errorf("deletes = %d, want 0 (manual entry should be preserved)", got)
	}
}

// === syncCNAMEs tests ===

func TestSyncCNAMEsAddsFromPrimary(t *testing.T) {
	counters := &expansionCounters{}
	srv := newExpansionTestServer(nil, nil, counters)
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	target := config.Target{Name: "replica", URL: srv.URL, Password: "pw"}
	client, err := pool.Get(target)
	if err != nil {
		t.Fatal(err)
	}

	r := &ReplicaRunner{marker: "[forseti-sync]", purgeDNS: false, log: slog.Default()}
	res := &run.Result{}

	primary := []pihole.APICNAMERecord{
		{Domain: "app.local", Target: "server.local"},
		{Domain: "api.local", Target: "server.local"},
	}
	replica := []pihole.APICNAMERecord{
		{Domain: "app.local", Target: "server.local"},
	}

	r.syncCNAMEs(client, res, primary, replica)

	if got := counters.cnameCreates.Load(); got != 1 {
		t.Errorf("creates = %d, want 1 (only api.local should be added)", got)
	}
}

func TestSyncCNAMEsDeletesWhenPurgeEnabled(t *testing.T) {
	counters := &expansionCounters{}
	srv := newExpansionTestServer(nil, nil, counters)
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	target := config.Target{Name: "replica", URL: srv.URL, Password: "pw"}
	client, err := pool.Get(target)
	if err != nil {
		t.Fatal(err)
	}

	r := &ReplicaRunner{marker: "[forseti-sync]", purgeDNS: true, log: slog.Default()}
	res := &run.Result{}

	primary := []pihole.APICNAMERecord{}
	replica := []pihole.APICNAMERecord{
		{Domain: "old.local", Target: "server.local"},
	}

	r.syncCNAMEs(client, res, primary, replica)

	if got := counters.cnameDeletes.Load(); got != 1 {
		t.Errorf("deletes = %d, want 1", got)
	}
}

func TestSyncCNAMEsKeepsOrphansWhenPurgeDisabled(t *testing.T) {
	counters := &expansionCounters{}
	srv := newExpansionTestServer(nil, nil, counters)
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	target := config.Target{Name: "replica", URL: srv.URL, Password: "pw"}
	client, err := pool.Get(target)
	if err != nil {
		t.Fatal(err)
	}

	r := &ReplicaRunner{marker: "[forseti-sync]", purgeDNS: false, log: slog.Default()}
	res := &run.Result{}

	primary := []pihole.APICNAMERecord{}
	replica := []pihole.APICNAMERecord{
		{Domain: "old.local", Target: "server.local"},
	}

	r.syncCNAMEs(client, res, primary, replica)

	if got := counters.cnameDeletes.Load(); got != 0 {
		t.Errorf("deletes = %d, want 0 (purge disabled)", got)
	}
}

// === state.load tests for new resources ===

func TestLoadAllowlistsError(t *testing.T) {
	srv := newErrorServer("lists")
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	target := config.Target{Name: "test", URL: srv.URL, Password: "pw"}
	client, _ := pool.Get(target)

	var st state
	err := st.load(client, map[string]bool{"allowlists": true})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "list allowlists") {
		t.Errorf("error = %q, want to contain 'list allowlists'", err.Error())
	}
}

func TestLoadCNAMEsError(t *testing.T) {
	srv := newErrorServer("config/dns/cnameRecords")
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	target := config.Target{Name: "test", URL: srv.URL, Password: "pw"}
	client, _ := pool.Get(target)

	var st state
	err := st.load(client, map[string]bool{"cnames": true})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "list cnames") {
		t.Errorf("error = %q, want to contain 'list cnames'", err.Error())
	}
}

func TestLoadAllowlistsAndCNAMEs(t *testing.T) {
	counters := &expansionCounters{}
	srv := newExpansionTestServer(
		[]pihole.APIList{{ID: 1, Address: "https://allow.com/list.txt"}},
		[]string{"app.local,server.local"},
		counters,
	)
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	target := config.Target{Name: "test", URL: srv.URL, Password: "pw", API: config.APIConfig{Timeout: config.Duration{Duration: 5 * time.Second}}}
	client, err := pool.Get(target)
	if err != nil {
		t.Fatal(err)
	}

	var st state
	if err := st.load(client, map[string]bool{"allowlists": true, "cnames": true}); err != nil {
		t.Fatalf("load error: %v", err)
	}

	if len(st.allowlists) != 1 {
		t.Errorf("allowlists = %d, want 1", len(st.allowlists))
	}
	if len(st.cnames) != 1 {
		t.Errorf("cnames = %d, want 1", len(st.cnames))
	}
}
