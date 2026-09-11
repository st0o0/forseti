package gravity

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/st0o0/forseti/internal/config"
	"github.com/st0o0/forseti/internal/session"
)

type mockRecorder struct {
	mu   sync.Mutex
	runs []recordedRun
}

type recordedRun struct {
	target  string
	trigger string
	dur     time.Duration
	err     error
}

func (r *mockRecorder) RecordGravityRun(target, trigger string, duration time.Duration, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runs = append(r.runs, recordedRun{target, trigger, duration, err})
}

func newGravityTestServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/auth" && r.Method == http.MethodPost:
			json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]string{"sid": "test-sid"},
			})
		case r.URL.Path == "/api/auth" && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/api/action/gravity":
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
}

func TestNewSchedulerParsesSchedules(t *testing.T) {
	targets := []config.Target{
		{Name: "alpha", Gravity: config.GravityConfig{Schedule: "0 3 * * 0"}},
		{Name: "beta", Gravity: config.GravityConfig{Schedule: "0 4 * * 0"}},
		{Name: "gamma"}, // no schedule
	}

	pool := session.NewPool()
	sched, err := NewScheduler(pool, targets, nil)
	if err != nil {
		t.Fatalf("NewScheduler error: %v", err)
	}

	if len(sched.entries) != 2 {
		t.Errorf("entries = %d, want 2 (gamma has no schedule)", len(sched.entries))
	}
}

func TestNewSchedulerRejectsInvalidCron(t *testing.T) {
	targets := []config.Target{
		{Name: "bad", Gravity: config.GravityConfig{Schedule: "not valid"}},
	}

	pool := session.NewPool()
	_, err := NewScheduler(pool, targets, nil)
	if err == nil {
		t.Error("expected error for invalid cron expression")
	}
}

func TestTriggerNowRecordsMetrics(t *testing.T) {
	srv := newGravityTestServer()
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	rec := &mockRecorder{}

	targets := []config.Target{
		{Name: "alpha", URL: srv.URL, Password: "pw", Gravity: config.GravityConfig{Schedule: "0 3 * * 0"}},
	}

	sched, err := NewScheduler(pool, targets, rec)
	if err != nil {
		t.Fatal(err)
	}

	if err := sched.TriggerNow("alpha", ReasonAdlistChange); err != nil {
		t.Fatalf("TriggerNow error: %v", err)
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.runs) != 1 {
		t.Fatalf("recorded runs = %d, want 1", len(rec.runs))
	}
	if rec.runs[0].trigger != string(ReasonAdlistChange) {
		t.Errorf("trigger = %q, want %q", rec.runs[0].trigger, ReasonAdlistChange)
	}
}

func TestTriggerNowUnknownTarget(t *testing.T) {
	pool := session.NewPool()
	sched, _ := NewScheduler(pool, nil, nil)

	err := sched.TriggerNow("nonexistent", ReasonScheduled)
	if err == nil {
		t.Error("expected error for unknown target")
	}
}
