package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/st0o0/forseti/internal/config"
	"github.com/st0o0/forseti/internal/logging/logtest"
)

func TestHealthcheckProbe(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer ok.Close()
	stalled := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer stalled.Close()

	if code := runHealthcheck([]string{"--addr", strings.TrimPrefix(ok.URL, "http://")}); code != 0 {
		t.Errorf("healthy daemon: exit %d, want 0", code)
	}
	if code := runHealthcheck([]string{"--addr", strings.TrimPrefix(stalled.URL, "http://")}); code != 1 {
		t.Errorf("stalled daemon: exit %d, want 1", code)
	}
	if code := runHealthcheck([]string{"--addr", "127.0.0.1:1"}); code != 1 {
		t.Errorf("no daemon: exit %d, want 1", code)
	}
}

func TestMetricsPort(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		p := filepath.Join(dir, name)
		_ = os.WriteFile(p, []byte(content), 0o644)
		return p
	}
	t.Setenv("FORSETI_TEST_PORT", "9200")

	cases := map[string]int{
		"":                                defaultMetricsPort,
		filepath.Join(dir, "missing.yml"): defaultMetricsPort,
		write("custom.yml", "metrics:\n  port: 8080\n"):                  8080,
		write("env.yml", "metrics:\n  port: ${FORSETI_TEST_PORT}\n"):     9200,
		write("broken.yml", "metrics: [broken\ntargets: {"):              defaultMetricsPort,
		write("invalid.yml", "mode: nonsense\nmetrics:\n  port: 7000\n"): 7000,
	}
	for path, want := range cases {
		if got := metricsPort(path); got != want {
			t.Errorf("metricsPort(%q) = %d, want %d", path, got, want)
		}
	}
}

func TestLogConfigLoadedSync(t *testing.T) {
	rec := logtest.New(t)
	cfg := &config.Config{
		Mode:     config.ModeSync,
		LogLevel: "info",
		Sync:     config.SyncConfig{Primary: "p", Resources: []string{"groups", "adlists"}, Interval: config.Duration{Duration: 5 * time.Minute}},
		Targets: []config.Target{
			{Name: "p", Role: "primary"},
			{Name: "r1", Role: "replica"},
			{Name: "r2", Role: "replica"},
		},
	}
	logConfigLoaded(rec.Logger(), cfg)

	ev := rec.Find("config loaded")
	if len(ev) != 1 {
		t.Fatalf("config loaded events = %d", len(ev))
	}
	a := ev[0].Attrs
	if a["primary"] != "p" || a["interval"] != 5*time.Minute {
		t.Errorf("attrs = %v", a)
	}
	if r, ok := a["replicas"].([]string); !ok || strings.Join(r, ",") != "r1,r2" {
		t.Errorf("replicas = %v", a["replicas"])
	}
}
