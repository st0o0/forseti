//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/st0o0/forseti/internal/config"
	"github.com/st0o0/forseti/internal/pihole"
	"github.com/st0o0/forseti/internal/reconcile"
	"github.com/st0o0/forseti/internal/session"
	"github.com/st0o0/forseti/internal/worker"
)

func piholeURL() string {
	if u := os.Getenv("PIHOLE_URL"); u != "" {
		return u
	}
	return "http://localhost:18080"
}

func piholePassword() string {
	if p := os.Getenv("PIHOLE_PASSWORD"); p != "" {
		return p
	}
	return "alpha-dev-password"
}

func skipIfNoPihole(t *testing.T) {
	t.Helper()
	resp, err := http.Get(piholeURL() + "/api/auth")
	if err != nil {
		t.Skipf("Pi-hole not available at %s: %v", piholeURL(), err)
	}
	resp.Body.Close()
}

func TestIntegration_FullReconcileCycle(t *testing.T) {
	skipIfNoPihole(t)

	client := pihole.NewClient(piholeURL(), piholePassword())
	if err := client.Login(); err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	defer client.Close()

	if err := client.CheckReadiness(); err != nil {
		t.Fatalf("CheckReadiness failed: %v", err)
	}

	rt := config.ResolvedTarget{
		Target: config.Target{Name: "integration", URL: piholeURL(), Password: piholePassword()},
		Deny:   []config.DenyEntry{{Domain: "integration-test.example.com"}},
	}

	report, err := reconcile.Apply(&rt, client, reconcile.ReconcileOptions{
		Marker: "[forseti-test]",
	})
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	for _, e := range report.Errors {
		t.Errorf("reconcile error: %v", e)
	}

	t.Logf("Reconciled: +%d ~%d -%d deny domains",
		len(report.Diff.Deny.Adds), len(report.Diff.Deny.Updates), len(report.Diff.Deny.Deletes))

	cleanupReport, err := reconcile.Apply(&config.ResolvedTarget{
		Target: rt.Target,
	}, client, reconcile.ReconcileOptions{
		Marker: "[forseti-test]",
	})
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}
	t.Logf("Cleanup: -%d deny domains", len(cleanupReport.Diff.Deny.Deletes))
}

func TestIntegration_ReadinessGate(t *testing.T) {
	skipIfNoPihole(t)

	client := pihole.NewClient(piholeURL(), piholePassword())
	if err := client.Login(); err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	defer client.Close()

	if err := client.CheckAlive(); err != nil {
		t.Fatalf("CheckAlive failed: %v", err)
	}

	if err := client.CheckReadiness(); err != nil {
		t.Fatalf("CheckReadiness failed: %v", err)
	}

	info, err := client.GetFTLInfo()
	if err != nil {
		t.Fatalf("GetFTLInfo failed: %v", err)
	}

	if info.PID == 0 {
		t.Error("PID should not be 0")
	}
	if info.Database.Gravity == 0 {
		t.Log("Warning: gravity count is 0 (fresh Pi-hole?)")
	}
	t.Logf("FTL: pid=%d uptime=%.0fs gravity=%d groups=%d",
		info.PID, info.Uptime, info.Database.Gravity, info.Database.Groups)
}

func TestIntegration_WorkerReconcile(t *testing.T) {
	skipIfNoPihole(t)

	pool := session.NewPool()
	defer pool.Close()

	rt := config.ResolvedTarget{
		Target: config.Target{Name: "integration", URL: piholeURL(), Password: piholePassword()},
		Deny:   []config.DenyEntry{{Domain: "worker-test.example.com"}},
	}

	rec := &noopRecorder{}
	grav := &noopGravity{}

	w := worker.NewTargetWorker(rt, worker.Dependencies{
		Sessions: pool,
		Settings: reconcile.Settings{},
		Content:  reconcile.Content{},
		Gravity:  grav,
		Recorder: rec,
		Interval: 5 * time.Minute,
		Marker:   "[forseti-test]",
	})

	if err := w.Reconcile(); err != nil {
		t.Fatalf("Worker.Reconcile failed: %v", err)
	}

	h := w.Health()
	if h.State != worker.Healthy {
		t.Errorf("health = %v, want Healthy", h.State)
	}
	if h.ConsecutiveFails != 0 {
		t.Errorf("consecutiveFails = %d, want 0", h.ConsecutiveFails)
	}

	rt.Deny = nil
	w.UpdateConfig(rt)
	if err := w.Reconcile(); err != nil {
		t.Fatalf("Cleanup reconcile failed: %v", err)
	}
}

func TestIntegration_SettingsChange(t *testing.T) {
	skipIfNoPihole(t)

	client := pihole.NewClient(piholeURL(), piholePassword())
	if err := client.Login(); err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	var configResp struct {
		Config map[string]any `json:"config"`
	}
	resp, err := http.Get(piholeURL() + "/admin/")
	if err != nil {
		t.Skipf("Admin interface not available: %v", err)
	}
	resp.Body.Close()

	req, _ := http.NewRequest("GET", piholeURL()+"/api/config", nil)
	req.Header.Set("X-FTL-SID", "invalid")
	_ = configResp
	_ = json.NewEncoder

	settings := config.Settings{
		DNS: config.DNSSettings{QueryLogging: boolPtr(true)},
	}
	diff, err := reconcile.DiffSettings(&settings, client)
	if err != nil {
		t.Fatalf("DiffSettings failed: %v", err)
	}
	t.Logf("Settings diff: %d changes", len(diff.Changes))

	client.Close()
}

type noopRecorder struct{}

func (noopRecorder) RecordReconcileResult(string, time.Duration, bool, map[string]map[string]int, map[string]int) {
}
func (noopRecorder) UpdateSettingsDrift(string, map[string]bool) {}
func (noopRecorder) MarkTargetUnreachable(string)               {}
func (noopRecorder) RecordTargetHealth(string, string)           {}

type noopGravity struct{}

func (noopGravity) TriggerNow(string, string) error { return nil }
func (noopGravity) TriggerAsync(string, string)     {}

func boolPtr(b bool) *bool { return &b }
