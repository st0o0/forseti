package metrics

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
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

func gaugeValue(t *testing.T, g interface {
	GetMetricWithLabelValues(...string) (prometheus.Gauge, error)
}, labels ...string) float64 {
	t.Helper()
	m, err := g.GetMetricWithLabelValues(labels...)
	if err != nil {
		t.Fatal(err)
	}
	var out dto.Metric
	_ = m.Write(&out)
	return out.GetGauge().GetValue()
}

func TestFreshnessAndFailureMetrics(t *testing.T) {
	s := NewServer(0, "/metrics", config.CollectorToggles{})
	start := time.Unix(1_700_000_000, 0)

	partial := run.Result{Target: "pi", Status: run.Partial, Started: start, Duration: 2 * time.Second}
	partial.Record("clients", "add", 2, errors.New("x"))
	partial.AddError(errors.New("re-fetch groups"))
	s.RecordRun(partial)

	if v := gaugeValue(t, s.reconcileLastSuccess, "pi"); v != float64(start.Unix()+2) {
		t.Errorf("last success = %v", v)
	}
	if v := getCounterValue(t, s.reconcileItemErrors, "pi", "clients"); v != 2 {
		t.Errorf("item errors clients = %v, want 2", v)
	}
	if v := getCounterValue(t, s.reconcileItemErrors, "pi", "other"); v != 1 {
		t.Errorf("item errors other = %v, want 1", v)
	}

	later := start.Add(time.Minute)
	s.RecordRun(run.Result{Target: "pi", Status: run.Failed, Started: later})
	if v := gaugeValue(t, s.reconcileLastRun, "pi"); v != float64(later.Unix()) {
		t.Errorf("last run = %v", v)
	}
	if v := gaugeValue(t, s.reconcileLastSuccess, "pi"); v != float64(start.Unix()+2) {
		t.Errorf("failed run moved last success to %v", v)
	}

	s.RecordTargetState("pi", run.TargetState{Health: "degraded", ConsecutiveFailures: 2})
	if v := gaugeValue(t, s.consecutiveFailures, "pi"); v != 2 {
		t.Errorf("consecutive failures = %v", v)
	}
	s.RecordTargetState("pi", run.TargetState{Health: "healthy"})
	if v := gaugeValue(t, s.consecutiveFailures, "pi"); v != 0 {
		t.Errorf("consecutive failures after success = %v", v)
	}
}

func TestHandleMountsRoute(t *testing.T) {
	s := NewServer(0, "/metrics", config.CollectorToggles{})
	s.Handle("/status", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	rr := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/status", nil))
	if rr.Code != http.StatusTeapot {
		t.Errorf("code = %d", rr.Code)
	}
}
