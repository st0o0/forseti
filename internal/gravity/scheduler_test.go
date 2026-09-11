package gravity

import (
	"context"
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

func TestTriggerNowNoScheduleTarget(t *testing.T) {
	srv := newGravityTestServer()
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	rec := &mockRecorder{}
	targets := []config.Target{
		{Name: "no-schedule", URL: srv.URL, Password: "pw"},
	}

	sched, err := NewScheduler(pool, targets, rec)
	if err != nil {
		t.Fatal(err)
	}

	if err := sched.TriggerNow("no-schedule", ReasonAdlistChange); err != nil {
		t.Fatalf("TriggerNow error: %v", err)
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.runs) != 1 {
		t.Fatalf("recorded runs = %d, want 1", len(rec.runs))
	}
}

func TestTriggerNowSessionError(t *testing.T) {
	pool := session.NewPool()
	targets := []config.Target{
		{Name: "bad", URL: "http://127.0.0.1:1", Password: "pw"},
	}

	rec := &mockRecorder{}
	sched, _ := NewScheduler(pool, targets, rec)

	err := sched.TriggerNow("bad", ReasonScheduled)
	if err == nil {
		t.Error("expected session error")
	}
}

func TestTriggerNowGravityAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/auth" && r.Method == http.MethodPost:
			json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]string{"sid": "test-sid"},
			})
		case r.URL.Path == "/api/auth" && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/api/action/gravity":
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	rec := &mockRecorder{}
	targets := []config.Target{
		{Name: "fail", URL: srv.URL, Password: "pw"},
	}

	sched, _ := NewScheduler(pool, targets, rec)
	err := sched.TriggerNow("fail", ReasonScheduled)
	if err == nil {
		t.Error("expected gravity trigger error")
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.runs) != 1 {
		t.Fatalf("recorded runs = %d, want 1", len(rec.runs))
	}
	if rec.runs[0].err == nil {
		t.Error("recorded run should have error")
	}
}

func TestStartContextCancellation(t *testing.T) {
	srv := newGravityTestServer()
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	targets := []config.Target{
		{Name: "alpha", URL: srv.URL, Password: "pw", Gravity: config.GravityConfig{Schedule: "* * * * *"}},
	}

	rec := &mockRecorder{}
	sched, err := NewScheduler(pool, targets, rec)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		sched.Start(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Error("Start did not return after context cancellation")
	}
}

func TestStartTickNoTriggerWhenNotDue(t *testing.T) {
	srv := newGravityTestServer()
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	rec := &mockRecorder{}
	targets := []config.Target{
		{Name: "future", URL: srv.URL, Password: "pw", Gravity: config.GravityConfig{Schedule: "0 3 * * 0"}},
	}

	sched, err := NewScheduler(pool, targets, rec)
	if err != nil {
		t.Fatal(err)
	}

	sched.mu.Lock()
	for i := range sched.entries {
		sched.entries[i].next = time.Now().Add(24 * time.Hour)
	}
	sched.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		sched.Start(ctx)
		close(done)
	}()

	time.Sleep(31 * time.Second)
	cancel()
	<-done

	rec.mu.Lock()
	n := len(rec.runs)
	rec.mu.Unlock()
	if n != 0 {
		t.Errorf("expected 0 triggers for future schedule, got %d", n)
	}
}

func TestStartTickTriggersGravity(t *testing.T) {
	srv := newGravityTestServer()
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	rec := &mockRecorder{}
	targets := []config.Target{
		{Name: "ticker-test", URL: srv.URL, Password: "pw", Gravity: config.GravityConfig{Schedule: "* * * * *"}},
	}

	sched, err := NewScheduler(pool, targets, rec)
	if err != nil {
		t.Fatal(err)
	}

	sched.mu.Lock()
	for i := range sched.entries {
		sched.entries[i].next = time.Now().Add(-time.Minute)
	}
	sched.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		sched.Start(ctx)
		close(done)
	}()

	deadline := time.After(34 * time.Second)
	for {
		rec.mu.Lock()
		n := len(rec.runs)
		rec.mu.Unlock()
		if n >= 1 {
			cancel()
			break
		}
		select {
		case <-deadline:
			cancel()
			t.Error("Start did not trigger gravity within timeout")
			<-done
			return
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}

	<-done
}

func TestStartNoEntries(t *testing.T) {
	pool := session.NewPool()
	sched, _ := NewScheduler(pool, nil, nil)

	done := make(chan struct{})
	go func() {
		ctx := context.Background()
		sched.Start(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Error("Start with no entries should return immediately")
	}
}

func TestTriggerWithSessionError(t *testing.T) {
	pool := session.NewPool()

	rec := &mockRecorder{}
	targets := []config.Target{
		{Name: "unreachable", URL: "http://127.0.0.1:1", Password: "pw", Gravity: config.GravityConfig{Schedule: "* * * * *"}},
	}

	sched, _ := NewScheduler(pool, targets, rec)

	sched.mu.Lock()
	sched.trigger(&sched.entries[0], ReasonScheduled)
	sched.mu.Unlock()

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.runs) != 1 {
		t.Fatalf("recorded runs = %d, want 1", len(rec.runs))
	}
	if rec.runs[0].err == nil {
		t.Error("expected error in recorded run")
	}
}

func TestTriggerWithNilRecorder(t *testing.T) {
	srv := newGravityTestServer()
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	targets := []config.Target{
		{Name: "test", URL: srv.URL, Password: "pw", Gravity: config.GravityConfig{Schedule: "* * * * *"}},
	}

	sched, _ := NewScheduler(pool, targets, nil)

	sched.mu.Lock()
	sched.trigger(&sched.entries[0], ReasonScheduled)
	sched.mu.Unlock()
}

func TestTriggerSuccessful(t *testing.T) {
	srv := newGravityTestServer()
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	rec := &mockRecorder{}
	targets := []config.Target{
		{Name: "test", URL: srv.URL, Password: "pw", Gravity: config.GravityConfig{Schedule: "* * * * *"}},
	}

	sched, _ := NewScheduler(pool, targets, rec)

	sched.mu.Lock()
	sched.trigger(&sched.entries[0], ReasonScheduled)
	sched.mu.Unlock()

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.runs) != 1 {
		t.Fatalf("recorded runs = %d, want 1", len(rec.runs))
	}
	if rec.runs[0].err != nil {
		t.Errorf("expected no error, got: %v", rec.runs[0].err)
	}
}

func TestTriggerGravityError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/auth" && r.Method == http.MethodPost:
			json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]string{"sid": "test-sid"},
			})
		case r.URL.Path == "/api/auth" && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/api/action/gravity":
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	pool := session.NewPool()
	defer pool.Close()

	rec := &mockRecorder{}
	targets := []config.Target{
		{Name: "err-test", URL: srv.URL, Password: "pw", Gravity: config.GravityConfig{Schedule: "* * * * *"}},
	}

	sched, _ := NewScheduler(pool, targets, rec)

	sched.mu.Lock()
	sched.trigger(&sched.entries[0], ReasonScheduled)
	sched.mu.Unlock()

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.runs) != 1 {
		t.Fatalf("recorded runs = %d, want 1", len(rec.runs))
	}
	if rec.runs[0].err == nil {
		t.Error("expected gravity error")
	}
}
