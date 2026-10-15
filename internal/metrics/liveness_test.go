package metrics

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/st0o0/forseti/internal/config"
	"github.com/st0o0/forseti/internal/run"
)

func TestHealthzOKAndStalled(t *testing.T) {
	s := NewServer(0, "/metrics", config.CollectorToggles{})
	s.SetLivenessInterval(time.Minute)
	s.Heartbeat()

	rr := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rr.Code != http.StatusOK || rr.Body.String() != "{\"status\":\"ok\"}\n" {
		t.Errorf("healthz = %d %q", rr.Code, rr.Body.String())
	}

	s.liveness.mu.Lock()
	s.liveness.last = time.Now().Add(-10 * time.Minute)
	s.liveness.mu.Unlock()

	rr = httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("stalled healthz = %d, want 503", rr.Code)
	}
}

func TestStallThreshold(t *testing.T) {
	if got := stallThreshold(10 * time.Second); got != 2*time.Minute+10*time.Second {
		t.Errorf("short interval threshold = %v", got)
	}
	if got := stallThreshold(5 * time.Minute); got != 15*time.Minute {
		t.Errorf("long interval threshold = %v", got)
	}
}

func TestRecordRunMapsStatusAndSkipsSkipped(t *testing.T) {
	s := NewServer(0, "/metrics", config.CollectorToggles{})

	ok := run.Result{Target: "pi", Status: run.OK}
	ok.Record("clients", "add", 2, nil)
	ok.Record("adlists", "add", 1, errors.New("x"))
	s.RecordRun(ok)
	s.RecordRun(run.Result{Target: "pi", Status: run.Failed})
	s.RecordRun(run.Result{Target: "pi", Status: run.Skipped})

	if v := getCounterValue(t, s.reconcileRuns, "pi", "success"); v != 1 {
		t.Errorf("success = %v", v)
	}
	if v := getCounterValue(t, s.reconcileRuns, "pi", "error"); v != 1 {
		t.Errorf("error = %v", v)
	}
	if v := getCounterValue(t, s.reconcileChanges, "pi", "clients", "add"); v != 2 {
		t.Errorf("clients add = %v", v)
	}
	if v := getCounterValue(t, s.reconcileChanges, "pi", "adlists", "add"); v != 0 {
		t.Errorf("failed adlist counted: %v", v)
	}
}
