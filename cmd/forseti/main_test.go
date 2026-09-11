package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/st0o0/forseti/internal/config"
	"github.com/st0o0/forseti/internal/gravity"
	"github.com/st0o0/forseti/internal/metrics"
	"github.com/st0o0/forseti/internal/reconcile"
	"github.com/st0o0/forseti/internal/session"
)

func TestHasDiff_NoChanges(t *testing.T) {
	r := &reconcile.DiffReport{Target: "test"}
	if hasDiff(r) {
		t.Error("hasDiff should return false for empty report")
	}
}

func TestHasDiff_WithGroupChanges(t *testing.T) {
	r := &reconcile.DiffReport{
		Target: "test",
		Groups: reconcile.ResourceDiff{
			Adds: []reconcile.DiffEntry{{Key: "g1"}},
		},
	}
	if !hasDiff(r) {
		t.Error("hasDiff should return true with group adds")
	}
}

func TestHasDiff_WithAdlistChanges(t *testing.T) {
	r := &reconcile.DiffReport{
		Target: "test",
		Adlists: reconcile.ResourceDiff{
			Deletes: []reconcile.DiffEntry{{Key: "a1"}},
		},
	}
	if !hasDiff(r) {
		t.Error("hasDiff should return true with adlist deletes")
	}
}

func TestHasDiff_WithDenyChanges(t *testing.T) {
	r := &reconcile.DiffReport{
		Target: "test",
		Deny: reconcile.ResourceDiff{
			Adds: []reconcile.DiffEntry{{Key: "d1"}},
		},
	}
	if !hasDiff(r) {
		t.Error("hasDiff should return true with deny adds")
	}
}

func TestHasDiff_WithAllowChanges(t *testing.T) {
	r := &reconcile.DiffReport{
		Target: "test",
		Allow: reconcile.ResourceDiff{
			Adds: []reconcile.DiffEntry{{Key: "a1"}},
		},
	}
	if !hasDiff(r) {
		t.Error("hasDiff should return true with allow adds")
	}
}

func TestHasDiff_WithLocalDNSChanges(t *testing.T) {
	r := &reconcile.DiffReport{
		Target: "test",
		LocalDNS: reconcile.ResourceDiff{
			Adds: []reconcile.DiffEntry{{Key: "dns1"}},
		},
	}
	if !hasDiff(r) {
		t.Error("hasDiff should return true with local DNS adds")
	}
}

func TestHasDiff_WithClientChanges(t *testing.T) {
	r := &reconcile.DiffReport{
		Target: "test",
		Clients: reconcile.ResourceDiff{
			Deletes: []reconcile.DiffEntry{{Key: "c1"}},
		},
	}
	if !hasDiff(r) {
		t.Error("hasDiff should return true with client deletes")
	}
}

func TestBuildChangesMap_Empty(t *testing.T) {
	r := &reconcile.DiffReport{Target: "test"}
	m := buildChangesMap(r)
	if len(m) != 0 {
		t.Errorf("buildChangesMap should be empty for no changes, got %d entries", len(m))
	}
}

func TestBuildChangesMap_WithChanges(t *testing.T) {
	r := &reconcile.DiffReport{
		Target: "test",
		Groups: reconcile.ResourceDiff{
			Adds:    []reconcile.DiffEntry{{Key: "g1"}, {Key: "g2"}},
			Deletes: []reconcile.DiffEntry{{Key: "g3"}},
		},
		Adlists: reconcile.ResourceDiff{
			Adds: []reconcile.DiffEntry{{Key: "a1"}},
		},
	}
	m := buildChangesMap(r)
	if m["group"]["add"] != 2 {
		t.Errorf("group add = %d, want 2", m["group"]["add"])
	}
	if m["group"]["delete"] != 1 {
		t.Errorf("group delete = %d, want 1", m["group"]["delete"])
	}
	if m["adlist"]["add"] != 1 {
		t.Errorf("adlist add = %d, want 1", m["adlist"]["add"])
	}
	if _, ok := m["deny"]; ok {
		t.Error("deny should not be in map with no changes")
	}
}

func TestBuildChangesMap_AllResources(t *testing.T) {
	r := &reconcile.DiffReport{
		Target: "test",
		Deny:     reconcile.ResourceDiff{Adds: []reconcile.DiffEntry{{Key: "d1"}}},
		Allow:    reconcile.ResourceDiff{Adds: []reconcile.DiffEntry{{Key: "a1"}}},
		LocalDNS: reconcile.ResourceDiff{Adds: []reconcile.DiffEntry{{Key: "l1"}}},
		Clients:  reconcile.ResourceDiff{Adds: []reconcile.DiffEntry{{Key: "c1"}}},
	}
	m := buildChangesMap(r)
	for _, name := range []string{"deny", "allow", "dns", "client"} {
		if _, ok := m[name]; !ok {
			t.Errorf("expected %s in changes map", name)
		}
	}
}

func TestAddResourceChanges_NoChanges(t *testing.T) {
	m := make(map[string]map[string]int)
	addResourceChanges(m, "test", reconcile.ResourceDiff{})
	if len(m) != 0 {
		t.Error("addResourceChanges should not add entry for no changes")
	}
}

func TestAddResourceChanges_WithChanges(t *testing.T) {
	m := make(map[string]map[string]int)
	addResourceChanges(m, "test", reconcile.ResourceDiff{
		Adds:    []reconcile.DiffEntry{{Key: "a"}, {Key: "b"}},
		Deletes: []reconcile.DiffEntry{{Key: "c"}},
	})
	if m["test"]["add"] != 2 {
		t.Errorf("add = %d, want 2", m["test"]["add"])
	}
	if m["test"]["delete"] != 1 {
		t.Errorf("delete = %d, want 1", m["test"]["delete"])
	}
}

func TestBuildDriftMap(t *testing.T) {
	r := &reconcile.DiffReport{
		Target: "test",
		Groups:  reconcile.ResourceDiff{Adds: []reconcile.DiffEntry{{Key: "g1"}}, Deletes: []reconcile.DiffEntry{{Key: "g2"}, {Key: "g3"}}},
		Adlists: reconcile.ResourceDiff{Adds: []reconcile.DiffEntry{{Key: "a1"}}},
	}
	m := buildDriftMap(r)
	if m["group"] != 3 {
		t.Errorf("group drift = %d, want 3", m["group"])
	}
	if m["adlist"] != 1 {
		t.Errorf("adlist drift = %d, want 1", m["adlist"])
	}
	if m["deny"] != 0 {
		t.Errorf("deny drift = %d, want 0", m["deny"])
	}
	if m["allow"] != 0 {
		t.Errorf("allow drift = %d, want 0", m["allow"])
	}
	if m["dns"] != 0 {
		t.Errorf("dns drift = %d, want 0", m["dns"])
	}
	if m["client"] != 0 {
		t.Errorf("client drift = %d, want 0", m["client"])
	}
}

func TestPrintDiffReport(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	report := &reconcile.DiffReport{
		Target: "pihole-test",
		Groups: reconcile.ResourceDiff{
			Adds:      []reconcile.DiffEntry{{Key: "g1"}},
			Unchanged: 2,
		},
		Adlists:      reconcile.ResourceDiff{Unchanged: 5},
		NeedsGravity: true,
	}
	printDiffReport(report)

	w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	os.Stdout = old
	out := buf.String()

	if !strings.Contains(out, "--- pihole-test ---") {
		t.Errorf("output should contain target header, got: %s", out)
	}
	if !strings.Contains(out, "groups") {
		t.Errorf("output should contain groups line, got: %s", out)
	}
	if !strings.Contains(out, "adlists") {
		t.Errorf("output should contain adlists line, got: %s", out)
	}
	if !strings.Contains(out, "gravity update required") {
		t.Errorf("output should contain gravity note, got: %s", out)
	}
}

func TestPrintDiffReport_NoGravity(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	report := &reconcile.DiffReport{Target: "t"}
	printDiffReport(report)

	w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	os.Stdout = old
	out := buf.String()

	if strings.Contains(out, "gravity") {
		t.Errorf("output should not contain gravity note for no gravity, got: %s", out)
	}
}

func TestPrintResourceLine_NoChangesNoUnchanged(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printResourceLine("groups", reconcile.ResourceDiff{})

	w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	os.Stdout = old

	if buf.Len() != 0 {
		t.Errorf("printResourceLine should produce no output for empty diff, got: %s", buf.String())
	}
}

func TestPrintResourceLine_WithUnchangedOnly(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printResourceLine("groups", reconcile.ResourceDiff{Unchanged: 3})

	w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	os.Stdout = old

	if !strings.Contains(buf.String(), "+0") {
		t.Errorf("expected +0 in output, got: %s", buf.String())
	}
}

func TestUsage(t *testing.T) {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	usage()

	w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	os.Stderr = old
	out := buf.String()

	if !strings.Contains(out, "forseti") {
		t.Errorf("usage should contain 'forseti', got: %s", out)
	}
	if !strings.Contains(out, "plan") {
		t.Errorf("usage should contain 'plan', got: %s", out)
	}
}

func newFullPiholeTestServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/auth" && r.Method == http.MethodPost:
			json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]string{"sid": "test-sid"},
			})
		case r.URL.Path == "/api/auth" && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/api/groups":
			json.NewEncoder(w).Encode(map[string]any{
				"groups": []any{
					map[string]any{"id": 0, "name": "Default", "comment": "", "enabled": true},
				},
			})
		case r.URL.Path == "/api/lists" && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(map[string]any{"lists": []any{}})
		case r.URL.Path == "/api/domains/deny/exact":
			json.NewEncoder(w).Encode(map[string]any{"domains": []any{}})
		case r.URL.Path == "/api/domains/allow/exact":
			json.NewEncoder(w).Encode(map[string]any{"domains": []any{}})
		case strings.HasPrefix(r.URL.Path, "/api/domains"):
			json.NewEncoder(w).Encode(map[string]any{"domains": []any{}})
		case r.URL.Path == "/api/config/dns/hosts":
			json.NewEncoder(w).Encode(map[string]any{"config": map[string]any{"dns": map[string]any{"hosts": []any{}}}})
		case r.URL.Path == "/api/config/dns/cnameRecords":
			json.NewEncoder(w).Encode(map[string]any{"config": map[string]any{"dns": map[string]any{"cnameRecords": []any{}}}})
		case r.URL.Path == "/api/clients":
			json.NewEncoder(w).Encode(map[string]any{"clients": []any{}})
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
}

func writeTempConfig(t *testing.T, serverURL string) string {
	t.Helper()
	cfg := fmt.Sprintf(`mode: config
metrics:
  port: 0
  path: /metrics
  scrape_interval: 30s
targets:
  - name: test
    url: %s
    password: test
reconcile:
  interval: 5m
  marker: "[forseti]"
`, serverURL)
	f, err := os.CreateTemp(t.TempDir(), "forseti-*.yml")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString(cfg)
	f.Close()
	return f.Name()
}

func TestRunPlan_Success(t *testing.T) {
	srv := newFullPiholeTestServer()
	defer srv.Close()

	cfgPath := writeTempConfig(t, srv.URL)
	code := runPlan([]string{"--config", cfgPath})
	if code != 0 {
		t.Errorf("runPlan returned %d, want 0", code)
	}
}

func TestRunPlan_InvalidConfig(t *testing.T) {
	old := os.Stderr
	_, w, _ := os.Pipe()
	os.Stderr = w

	code := runPlan([]string{"--config", "/nonexistent/config.yml"})

	w.Close()
	os.Stderr = old
	if code != 1 {
		t.Errorf("runPlan with bad config returned %d, want 1", code)
	}
}

func TestRunPlan_LoginError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	cfgPath := writeTempConfig(t, srv.URL)
	code := runPlan([]string{"--config", cfgPath})
	if code != 0 {
		t.Errorf("runPlan with login error returned %d, want 0 (continues past login errors)", code)
	}
}

func TestRunPlan_ReconcileError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/auth" && r.Method == http.MethodPost:
			json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]string{"sid": "s"},
			})
		case r.URL.Path == "/api/auth" && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/api/groups":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	cfgPath := writeTempConfig(t, srv.URL)
	code := runPlan([]string{"--config", cfgPath})
	if code != 0 {
		t.Errorf("runPlan with reconcile error returned %d, want 0 (continues past errors)", code)
	}
}

func TestRunApply_Success(t *testing.T) {
	srv := newFullPiholeTestServer()
	defer srv.Close()

	cfgPath := writeTempConfig(t, srv.URL)
	code := runApply([]string{"--config", cfgPath})
	if code != 0 {
		t.Errorf("runApply returned %d, want 0", code)
	}
}

func TestRunApply_InvalidConfig(t *testing.T) {
	old := os.Stderr
	_, w, _ := os.Pipe()
	os.Stderr = w

	code := runApply([]string{"--config", "/nonexistent/config.yml"})

	w.Close()
	os.Stderr = old
	if code != 1 {
		t.Errorf("runApply with bad config returned %d, want 1", code)
	}
}

func TestRunApply_LoginError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	cfgPath := writeTempConfig(t, srv.URL)
	code := runApply([]string{"--config", cfgPath})
	if code != 1 {
		t.Errorf("runApply with login error returned %d, want 1", code)
	}
}

func TestRunApply_ReconcileError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/auth" && r.Method == http.MethodPost:
			json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]string{"sid": "s"},
			})
		case r.URL.Path == "/api/auth" && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/api/groups":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	cfgPath := writeTempConfig(t, srv.URL)
	code := runApply([]string{"--config", cfgPath})
	if code != 1 {
		t.Errorf("runApply with reconcile error returned %d, want 1", code)
	}
}

func TestRunApply_WithWarnings(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/auth" && r.Method == http.MethodPost:
			json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]string{"sid": "s"},
			})
		case r.URL.Path == "/api/auth" && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/api/groups":
			json.NewEncoder(w).Encode(map[string]any{
				"groups": []any{
					map[string]any{"id": 0, "name": "Default", "comment": "", "enabled": true},
				},
			})
		case r.URL.Path == "/api/lists" && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(map[string]any{"lists": []any{}})
		case r.URL.Path == "/api/lists" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusInternalServerError)
		case strings.HasPrefix(r.URL.Path, "/api/domains"):
			json.NewEncoder(w).Encode(map[string]any{"domains": []any{}})
		case r.URL.Path == "/api/config/dns/hosts":
			json.NewEncoder(w).Encode(map[string]any{"config": map[string]any{"dns": map[string]any{"hosts": []any{}}}})
		case r.URL.Path == "/api/config/dns/cnameRecords":
			json.NewEncoder(w).Encode(map[string]any{"config": map[string]any{"dns": map[string]any{"cnameRecords": []any{}}}})
		case r.URL.Path == "/api/clients":
			json.NewEncoder(w).Encode(map[string]any{"clients": []any{}})
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	cfg := fmt.Sprintf(`mode: config
metrics:
  port: 0
  path: /metrics
  scrape_interval: 30s
targets:
  - name: test
    url: %s
    password: test
reconcile:
  interval: 5m
  marker: "[forseti]"
adlists:
  - url: https://example.com/list.txt
    comment: "[forseti] test"
`, srv.URL)
	f, err := os.CreateTemp(t.TempDir(), "forseti-*.yml")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString(cfg)
	f.Close()

	code := runApply([]string{"--config", f.Name()})
	if code != 1 {
		t.Errorf("runApply with warnings returned %d, want 1", code)
	}
}

func TestReconcileAll_SessionError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	cfgPath := writeTempConfig(t, srv.URL)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	pool := session.NewPool()
	defer pool.Close()

	m := metrics.NewServer(0, "/metrics")
	gravSched, _ := gravity.NewScheduler(pool, nil, m)

	reconcileAll(cfg, m, pool, gravSched)
}

func TestReconcileAll_ReconcileError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/auth" && r.Method == http.MethodPost:
			json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]string{"sid": "s"},
			})
		case r.URL.Path == "/api/auth" && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/api/groups":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	cfgPath := writeTempConfig(t, srv.URL)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	pool := session.NewPool()
	defer pool.Close()

	m := metrics.NewServer(0, "/metrics")
	gravSched, _ := gravity.NewScheduler(pool, cfg.Targets, m)

	reconcileAll(cfg, m, pool, gravSched)
}

func TestReconcileAll_Success(t *testing.T) {
	srv := newFullPiholeTestServer()
	defer srv.Close()

	cfgPath := writeTempConfig(t, srv.URL)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	pool := session.NewPool()
	defer pool.Close()

	m := metrics.NewServer(0, "/metrics")
	gravSched, _ := gravity.NewScheduler(pool, cfg.Targets, m)

	reconcileAll(cfg, m, pool, gravSched)
}

func TestReconcileAll_WithGravityTrigger(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/auth" && r.Method == http.MethodPost:
			json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]string{"sid": "s"},
			})
		case r.URL.Path == "/api/auth" && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/api/groups":
			json.NewEncoder(w).Encode(map[string]any{
				"groups": []any{
					map[string]any{"id": 0, "name": "Default", "comment": "", "enabled": true},
				},
			})
		case r.URL.Path == "/api/lists" && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(map[string]any{"lists": []any{}})
		case r.URL.Path == "/api/lists" && r.Method == http.MethodPost:
			json.NewEncoder(w).Encode(map[string]any{"list": map[string]any{"id": 1, "address": "https://example.com/list.txt"}})
		case strings.HasPrefix(r.URL.Path, "/api/domains"):
			json.NewEncoder(w).Encode(map[string]any{"domains": []any{}})
		case r.URL.Path == "/api/config/dns/hosts":
			json.NewEncoder(w).Encode(map[string]any{"config": map[string]any{"dns": map[string]any{"hosts": []any{}}}})
		case r.URL.Path == "/api/config/dns/cnameRecords":
			json.NewEncoder(w).Encode(map[string]any{"config": map[string]any{"dns": map[string]any{"cnameRecords": []any{}}}})
		case r.URL.Path == "/api/clients":
			json.NewEncoder(w).Encode(map[string]any{"clients": []any{}})
		case r.URL.Path == "/api/action/gravity":
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	gravOnChange := true
	cfg := fmt.Sprintf(`mode: config
metrics:
  port: 0
  path: /metrics
  scrape_interval: 30s
targets:
  - name: test
    url: %s
    password: test
reconcile:
  interval: 5m
  marker: "[forseti]"
  gravity_on_change: true
adlists:
  - url: https://example.com/list.txt
    comment: "[forseti] test"
`, srv.URL)
	f, err := os.CreateTemp(t.TempDir(), "forseti-*.yml")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString(cfg)
	f.Close()

	loadedCfg, err := config.Load(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	loadedCfg.Reconcile.GravityOnChange = &gravOnChange

	pool := session.NewPool()
	defer pool.Close()

	m := metrics.NewServer(0, "/metrics")
	gravSched, _ := gravity.NewScheduler(pool, loadedCfg.Targets, m)

	reconcileAll(loadedCfg, m, pool, gravSched)
}

func TestReconcileAll_GravityTriggerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/auth" && r.Method == http.MethodPost:
			json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]string{"sid": "s"},
			})
		case r.URL.Path == "/api/auth" && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/api/groups":
			json.NewEncoder(w).Encode(map[string]any{
				"groups": []any{
					map[string]any{"id": 0, "name": "Default", "comment": "", "enabled": true},
				},
			})
		case r.URL.Path == "/api/lists" && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(map[string]any{"lists": []any{}})
		case r.URL.Path == "/api/lists" && r.Method == http.MethodPost:
			json.NewEncoder(w).Encode(map[string]any{"list": map[string]any{"id": 1, "address": "https://example.com/list.txt"}})
		case strings.HasPrefix(r.URL.Path, "/api/domains"):
			json.NewEncoder(w).Encode(map[string]any{"domains": []any{}})
		case r.URL.Path == "/api/config/dns/hosts":
			json.NewEncoder(w).Encode(map[string]any{"config": map[string]any{"dns": map[string]any{"hosts": []any{}}}})
		case r.URL.Path == "/api/config/dns/cnameRecords":
			json.NewEncoder(w).Encode(map[string]any{"config": map[string]any{"dns": map[string]any{"cnameRecords": []any{}}}})
		case r.URL.Path == "/api/clients":
			json.NewEncoder(w).Encode(map[string]any{"clients": []any{}})
		case r.URL.Path == "/api/action/gravity":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	gravOnChange := true
	cfgStr := fmt.Sprintf(`mode: config
metrics:
  port: 0
  path: /metrics
  scrape_interval: 30s
targets:
  - name: test
    url: %s
    password: test
reconcile:
  interval: 5m
  marker: "[forseti]"
  gravity_on_change: true
adlists:
  - url: https://example.com/list.txt
    comment: "[forseti] test"
`, srv.URL)
	f, err := os.CreateTemp(t.TempDir(), "forseti-*.yml")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(cfgStr)
	f.Close()

	loadedCfg, err := config.Load(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	loadedCfg.Reconcile.GravityOnChange = &gravOnChange

	pool := session.NewPool()
	defer pool.Close()

	m := metrics.NewServer(0, "/metrics")
	gravSched, _ := gravity.NewScheduler(pool, loadedCfg.Targets, m)

	reconcileAll(loadedCfg, m, pool, gravSched)
}

func TestRunPlan_WithChanges(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/auth" && r.Method == http.MethodPost:
			json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]string{"sid": "s"},
			})
		case r.URL.Path == "/api/auth" && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/api/groups":
			json.NewEncoder(w).Encode(map[string]any{
				"groups": []any{
					map[string]any{"id": 0, "name": "Default", "comment": "", "enabled": true},
				},
			})
		case r.URL.Path == "/api/lists" && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(map[string]any{"lists": []any{}})
		case strings.HasPrefix(r.URL.Path, "/api/domains"):
			json.NewEncoder(w).Encode(map[string]any{"domains": []any{}})
		case r.URL.Path == "/api/config/dns/hosts":
			json.NewEncoder(w).Encode(map[string]any{"config": map[string]any{"dns": map[string]any{"hosts": []any{}}}})
		case r.URL.Path == "/api/config/dns/cnameRecords":
			json.NewEncoder(w).Encode(map[string]any{"config": map[string]any{"dns": map[string]any{"cnameRecords": []any{}}}})
		case r.URL.Path == "/api/clients":
			json.NewEncoder(w).Encode(map[string]any{"clients": []any{}})
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	cfg := fmt.Sprintf(`mode: config
metrics:
  port: 0
  path: /metrics
  scrape_interval: 30s
targets:
  - name: test
    url: %s
    password: test
reconcile:
  interval: 5m
  marker: "[forseti]"
adlists:
  - url: https://example.com/list.txt
    comment: "[forseti] test"
`, srv.URL)
	f, err := os.CreateTemp(t.TempDir(), "forseti-*.yml")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString(cfg)
	f.Close()

	code := runPlan([]string{"--config", f.Name()})
	if code != 0 {
		t.Errorf("runPlan with changes returned %d, want 0", code)
	}
}

func TestRunWatch_InvalidConfig(t *testing.T) {
	old := os.Stderr
	_, w, _ := os.Pipe()
	os.Stderr = w

	code := runWatch([]string{"--config", "/nonexistent/config.yml"})

	w.Close()
	os.Stderr = old
	if code != 1 {
		t.Errorf("runWatch with bad config returned %d, want 1", code)
	}
}

func TestRunWatch_InvalidGravitySchedule(t *testing.T) {
	cfg := `mode: config
metrics:
  port: 0
  path: /metrics
  scrape_interval: 30s
targets:
  - name: test
    url: http://localhost:1
    password: test
    gravity:
      schedule: "invalid cron"
reconcile:
  interval: 5m
  marker: "[forseti]"
`
	f, err := os.CreateTemp(t.TempDir(), "forseti-*.yml")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString(cfg)
	f.Close()

	old := os.Stderr
	_, w, _ := os.Pipe()
	os.Stderr = w

	code := runWatch([]string{"--config", f.Name()})

	w.Close()
	os.Stderr = old
	if code != 1 {
		t.Errorf("runWatch with bad gravity schedule returned %d, want 1", code)
	}
}

func TestRunWatch_ConfigModeStartsAndStops(t *testing.T) {
	srv := newFullPiholeTestServer()
	defer srv.Close()

	cfgPath := writeTempConfig(t, srv.URL)

	cmd := exec.Command(os.Args[0], "-test.run=TestRunWatch_SubprocessConfig", "-test.timeout=10s")
	cmd.Env = append(os.Environ(), "FORSETI_WATCH_CONFIG="+cfgPath)
	err := cmd.Start()
	if err != nil {
		t.Fatalf("failed to start subprocess: %v", err)
	}

	time.Sleep(500 * time.Millisecond)
	_ = cmd.Process.Kill()
}

func TestRunWatch_SubprocessConfig(t *testing.T) {
	cfgPath := os.Getenv("FORSETI_WATCH_CONFIG")
	if cfgPath == "" {
		t.Skip("only runs as subprocess")
	}
	runWatch([]string{"--config", cfgPath})
}

func TestRunWatch_SyncModeStartsAndStops(t *testing.T) {
	srv := newFullPiholeTestServer()
	defer srv.Close()

	cfg := fmt.Sprintf(`mode: sync
metrics:
  port: 0
  path: /metrics
  scrape_interval: 30s
targets:
  - name: primary
    url: %s
    password: test
    role: primary
  - name: replica
    url: %s
    password: test
    role: replica
sync:
  interval: 5m
  primary: primary
  resources:
    - adlists
`, srv.URL, srv.URL)
	f, err := os.CreateTemp(t.TempDir(), "forseti-*.yml")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString(cfg)
	f.Close()

	cmd := exec.Command(os.Args[0], "-test.run=TestRunWatch_SubprocessSync", "-test.timeout=10s")
	cmd.Env = append(os.Environ(), "FORSETI_WATCH_CONFIG="+f.Name())
	err = cmd.Start()
	if err != nil {
		t.Fatalf("failed to start subprocess: %v", err)
	}

	time.Sleep(500 * time.Millisecond)
	_ = cmd.Process.Kill()
}

func TestRunWatch_SubprocessSync(t *testing.T) {
	cfgPath := os.Getenv("FORSETI_WATCH_CONFIG")
	if cfgPath == "" {
		t.Skip("only runs as subprocess")
	}
	runWatch([]string{"--config", cfgPath})
}

func TestTryReloadConfig_Unchanged(t *testing.T) {
	srv := newFullPiholeTestServer()
	defer srv.Close()

	cfgPath := writeTempConfig(t, srv.URL)
	info, err := os.Stat(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	lastMtime := info.ModTime()

	newCfg, err := tryReloadConfig(cfgPath, &lastMtime)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if newCfg != nil {
		t.Error("expected nil config for unchanged file")
	}
}

func TestTryReloadConfig_Changed(t *testing.T) {
	srv := newFullPiholeTestServer()
	defer srv.Close()

	cfgPath := writeTempConfig(t, srv.URL)
	lastMtime := time.Time{}

	newCfg, err := tryReloadConfig(cfgPath, &lastMtime)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if newCfg == nil {
		t.Fatal("expected non-nil config for changed file")
	}
	if len(newCfg.Targets) != 1 {
		t.Errorf("expected 1 target, got %d", len(newCfg.Targets))
	}
}

func TestTryReloadConfig_InvalidFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := dir + "/bad.yml"
	_ = os.WriteFile(cfgPath, []byte("invalid: [broken"), 0o644)
	lastMtime := time.Time{}

	_, err := tryReloadConfig(cfgPath, &lastMtime)
	if err == nil {
		t.Error("expected error for invalid config file")
	}
}

func TestTryReloadConfig_MissingFile(t *testing.T) {
	lastMtime := time.Time{}
	_, err := tryReloadConfig("/nonexistent/config.yml", &lastMtime)
	if err == nil {
		t.Error("expected error for missing file")
	}
}
