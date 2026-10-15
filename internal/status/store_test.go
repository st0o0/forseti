package status

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/st0o0/forseti/internal/run"
)

func result(target string, status run.Status, at time.Time) run.Result {
	return run.Result{Target: target, Status: status, Started: at, Duration: 150 * time.Millisecond}
}

func TestRingBounds(t *testing.T) {
	r := newRing[int](3)
	for i := 1; i <= 5; i++ {
		r.push(i)
	}
	if got := r.items(); len(got) != 3 || got[0] != 3 || got[2] != 5 {
		t.Errorf("items = %v, want [3 4 5]", got)
	}
	if v, _ := r.last(); v != 5 {
		t.Errorf("last = %d", v)
	}
}

func TestHistoryBoundedAndOrdered(t *testing.T) {
	s := New("1.0.0", "sync", 10)
	s.SetTargets([]Target{{Name: "pi", Role: "replica"}})
	base := time.Now().Add(-time.Hour)
	for i := 0; i < 25; i++ {
		s.RunFinished(result("pi", run.OK, base.Add(time.Duration(i)*time.Minute)), run.TargetState{Health: "healthy"})
	}

	entries, ok := s.History("pi", 24*time.Hour, 0)
	if !ok || len(entries) != 10 {
		t.Fatalf("entries = %d, want 10", len(entries))
	}
	if !entries[0].At.Before(entries[9].At) {
		t.Error("entries must be oldest first")
	}
	if got, _ := s.History("pi", 24*time.Hour, 3); len(got) != 3 || !got[2].At.Equal(entries[9].At) {
		t.Errorf("limit must keep the most recent entries")
	}
	if got, _ := s.History("pi", 5*time.Minute, 0); len(got) != 0 {
		t.Errorf("since filter: got %d entries, want 0", len(got))
	}
}

func TestSetTargetsDropsRemoved(t *testing.T) {
	s := New("1.0.0", "sync", 10)
	s.SetTargets([]Target{{Name: "a"}, {Name: "b"}})
	s.RunFinished(result("b", run.OK, time.Now()), run.TargetState{Health: "healthy"})
	s.SetTargets([]Target{{Name: "a"}})

	if _, ok := s.History("b", time.Hour, 0); ok {
		t.Error("removed target still has history")
	}
	doc := s.snapshot()
	if len(doc.Targets) != 1 || doc.Targets[0].Name != "a" {
		t.Errorf("targets = %+v", doc.Targets)
	}
}

func TestSnapshotReportsStateAndRedactedError(t *testing.T) {
	s := New("1.0.0", "sync", 10)
	s.SetInterval(5 * time.Minute)
	s.SetTargets([]Target{{Name: "pihole-2", Role: "replica"}, {Name: "pihole-1", Role: "primary"}})

	lastOK := time.Now().Add(-20 * time.Minute)
	failed := result("pihole-2", run.Failed, time.Now())
	failed.Fail(run.StageApply, errors.New(`create adlist "https://lists.example.org/x.txt?token=abc": 500`))
	s.RunFinished(failed, run.TargetState{Health: "degraded", ConsecutiveFailures: 3, LastSuccess: lastOK, BackoffUntil: time.Now().Add(10 * time.Minute)})
	s.CycleFinished(run.Cycle{Targets: 2, OK: 1, Failed: 1}, time.Now().Add(5*time.Minute))

	rr := httptest.NewRecorder()
	Handler(s).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/status", nil))
	body := rr.Body.String()
	if rr.Code != http.StatusOK || rr.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("code=%d headers=%v", rr.Code, rr.Header())
	}
	if strings.Contains(body, "abc") || !strings.Contains(body, "token=REDACTED") {
		t.Errorf("error not redacted: %s", body)
	}

	var doc statusDoc
	if err := json.Unmarshal(rr.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Targets) != 2 || doc.Targets[0].Name != "pihole-1" {
		t.Fatalf("targets not sorted: %+v", doc.Targets)
	}
	r2 := doc.Targets[1]
	if r2.Health != "degraded" || r2.ConsecutiveFailures != 3 || r2.LastSuccessAt == nil || r2.LastError == nil || r2.BackoffUntil == nil {
		t.Errorf("pihole-2 = %+v", r2)
	}
	if r2.LastRun == nil || r2.LastRun.Status != run.Failed || r2.LastRun.Stage != run.StageApply {
		t.Errorf("last run = %+v", r2.LastRun)
	}
	if doc.Targets[0].LastRun != nil {
		t.Error("target without runs must have no last_run")
	}
	if doc.Cycle.Last == nil || doc.Cycle.Last.Failed != 1 || doc.Cycle.NextRunAt == nil || doc.Cycle.IntervalSeconds != 300 {
		t.Errorf("cycle = %+v", doc.Cycle)
	}
}

func TestHistoryEndpoint(t *testing.T) {
	s := New("1.0.0", "config", 10)
	s.SetTargets([]Target{{Name: "pi"}})
	ok := result("pi", run.OK, time.Now())
	ok.Record("clients", "add", 2, nil)
	s.RunFinished(ok, run.TargetState{Health: "healthy"})
	s.CycleFinished(run.Cycle{Targets: 1, OK: 1, Changes: 2}, time.Now())
	h := Handler(s)

	get := func(url string) *httptest.ResponseRecorder {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, url, nil))
		return rr
	}

	rr := get("/status/history?target=pi&since=1h")
	var hist struct {
		Target  string  `json:"target"`
		Entries []Entry `json:"entries"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &hist)
	if rr.Code != 200 || len(hist.Entries) != 1 || hist.Entries[0].Resources["clients"].Added != 2 {
		t.Errorf("history = %d %s", rr.Code, rr.Body.String())
	}
	if rr := get("/status/history"); rr.Code != 200 || !strings.Contains(rr.Body.String(), `"changes":2`) {
		t.Errorf("cycles = %d %s", rr.Code, rr.Body.String())
	}
	if rr := get("/status/history?target=nope"); rr.Code != http.StatusNotFound {
		t.Errorf("unknown target = %d", rr.Code)
	}
	for _, bad := range []string{"since=abc", "since=-1h", "limit=0", "limit=x"} {
		if rr := get("/status/history?" + bad); rr.Code != http.StatusBadRequest {
			t.Errorf("%s = %d, want 400", bad, rr.Code)
		}
	}
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/status", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /status = %d", rr.Code)
	}
}

func TestConfigReloadRecorded(t *testing.T) {
	s := New("1.0.0", "config", 10)
	s.ConfigReloaded(false, errors.New("yaml: line 3: bad"))
	doc := s.snapshot()
	if doc.Config.LastReload == nil || doc.Config.LastReload.OK || doc.Config.LastReload.Error == "" {
		t.Errorf("reload = %+v", doc.Config.LastReload)
	}
}
