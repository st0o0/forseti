package worker

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/st0o0/forseti/internal/config"
	"github.com/st0o0/forseti/internal/pihole"
	"github.com/st0o0/forseti/internal/reconcile"
)

func newReadyServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/info/ftl" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ftl":{"database":{"gravity":100,"groups":2,"lists":2,"clients":1},"pid":1,"uptime":100}}`))
			return
		}
		if r.URL.Path == "/api/info/login" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"dns":true}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv
}


type mockSessions struct {
	client       *pihole.Client
	err          error
	calls        int
	invalids     []string
	acquireCalls int
	releaseCalls int
}

func (m *mockSessions) Get(target config.Target) (*pihole.Client, error) {
	m.calls++
	return m.client, m.err
}

func (m *mockSessions) Invalidate(name string) {
	m.invalids = append(m.invalids, name)
}

func (m *mockSessions) Acquire(name string)        { m.acquireCalls++ }
func (m *mockSessions) TryAcquire(name string) bool { return true }
func (m *mockSessions) Release(name string)         { m.releaseCalls++ }

type mockSettings struct {
	diffResult  *reconcile.SettingsDiff
	diffErr     error
	applyResult *reconcile.SettingsDiff
	applyErr    error
	diffCalls   int
	applyCalls  int
}

func (m *mockSettings) Diff(settings *config.Settings, client *pihole.Client) (*reconcile.SettingsDiff, error) {
	m.diffCalls++
	return m.diffResult, m.diffErr
}

func (m *mockSettings) Apply(name string, settings *config.Settings, client *pihole.Client) (*reconcile.SettingsDiff, error) {
	m.applyCalls++
	return m.applyResult, m.applyErr
}

func (m *mockSettings) BuildDesiredList(settings *config.Settings) []reconcile.SettingMapping {
	return nil
}

type mockContent struct {
	report    *reconcile.ApplyReport
	err       error
	calls     int
	lastClient *pihole.Client
}

func (m *mockContent) Apply(rt *config.ResolvedTarget, client *pihole.Client, opts reconcile.ReconcileOptions) (*reconcile.ApplyReport, error) {
	m.calls++
	m.lastClient = client
	return m.report, m.err
}

type mockGravity struct {
	calls   int
	targets []string
	err     error
}

func (m *mockGravity) TriggerNow(target string, reason string) error {
	m.calls++
	m.targets = append(m.targets, target)
	return m.err
}

func (m *mockGravity) TriggerAsync(target string, reason string) {
	m.calls++
	m.targets = append(m.targets, target)
}

type mockRecorder struct {
	reconcileCalls    int
	settingsDriftCalls int
	unreachableCalls  int
	lastTarget        string
	lastSuccess       bool
}

func (m *mockRecorder) RecordReconcileResult(target string, duration time.Duration, success bool, changes map[string]map[string]int, drift map[string]int) {
	m.reconcileCalls++
	m.lastTarget = target
	m.lastSuccess = success
}

func (m *mockRecorder) UpdateSettingsDrift(target string, drifted map[string]bool) {
	m.settingsDriftCalls++
}

func (m *mockRecorder) MarkTargetUnreachable(target string) {
	m.unreachableCalls++
}

func (m *mockRecorder) RecordTargetHealth(target string, state string) {
}

func newTestWorker() (*TargetWorker, *mockSessions, *mockSettings, *mockContent, *mockGravity, *mockRecorder) {
	readySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/info/ftl" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ftl":{"database":{"gravity":100,"groups":2},"pid":1,"uptime":100}}`))
			return
		}
		if r.URL.Path == "/api/info/login" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"dns":true}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	sess := &mockSessions{client: pihole.NewClient(readySrv.URL, "pw", 0)}
	sett := &mockSettings{
		diffResult: &reconcile.SettingsDiff{},
	}
	cont := &mockContent{
		report: &reconcile.ApplyReport{},
	}
	grav := &mockGravity{}
	rec := &mockRecorder{}

	rt := config.ResolvedTarget{
		Target: config.Target{Name: "test", URL: "http://test:1000", Password: "pw"},
	}

	w := NewTargetWorker(rt, Dependencies{
		Sessions:  sess,
		Settings:  sett,
		Content:   cont,
		Gravity:   grav,
		Recorder:  rec,
		Interval:  5 * time.Minute,
		Marker:    "[forseti]",
	})

	return w, sess, sett, cont, grav, rec
}

// --- Health + Backoff tests ---

func TestHealthyToDegradedOnFailure(t *testing.T) {
	w, sess, _, _, _, _ := newTestWorker()
	sess.err = errors.New("connection refused")

	_ = w.Reconcile()

	h := w.Health()
	if h.State != Degraded {
		t.Errorf("health = %v, want Degraded", h.State)
	}
	if h.ConsecutiveFails != 1 {
		t.Errorf("consecutiveFails = %d, want 1", h.ConsecutiveFails)
	}
}

func TestDegradedToHealthyOnSuccess(t *testing.T) {
	w, sess, _, _, _, _ := newTestWorker()

	sess.err = errors.New("fail")
	_ = w.Reconcile()
	if w.Health().State != Degraded {
		t.Fatal("should be degraded after failure")
	}

	sess.err = nil
	_ = w.Reconcile()

	h := w.Health()
	if h.State != Healthy {
		t.Errorf("health = %v, want Healthy", h.State)
	}
	if h.ConsecutiveFails != 0 {
		t.Errorf("consecutiveFails = %d, want 0", h.ConsecutiveFails)
	}
}

func TestBackoffIncreasesWithFailures(t *testing.T) {
	w, sess, _, _, _, _ := newTestWorker()
	sess.err = errors.New("fail")

	_ = w.Reconcile()
	b1 := w.Health().BackoffUntil

	_ = w.Reconcile()
	b2 := w.Health().BackoffUntil

	if !b2.After(b1) {
		t.Error("backoff should increase with consecutive failures")
	}
}

func TestBackoffCappedAt30Min(t *testing.T) {
	w, sess, _, _, _, _ := newTestWorker()
	sess.err = errors.New("fail")

	for i := 0; i < 20; i++ {
		_ = w.Reconcile()
	}

	h := w.Health()
	maxExpected := time.Now().Add(maxBackoff + time.Second)
	if h.BackoffUntil.After(maxExpected) {
		t.Errorf("backoff %v exceeds max %v", h.BackoffUntil, maxExpected)
	}
}

func TestShouldSkipDuringBackoff(t *testing.T) {
	w, sess, _, _, _, _ := newTestWorker()
	sess.err = errors.New("fail")

	_ = w.Reconcile()

	if !w.ShouldSkip(time.Now()) {
		t.Error("should skip during backoff")
	}

	future := time.Now().Add(1 * time.Hour)
	if w.ShouldSkip(future) {
		t.Error("should not skip after backoff expires")
	}
}

// --- Orchestration tests ---

func TestNormalReconcileCycle(t *testing.T) {
	w, sess, sett, cont, grav, rec := newTestWorker()
	cont.report = &reconcile.ApplyReport{
		Diff: reconcile.DiffReport{
			Adlists: reconcile.ResourceDiff{
				Adds: []reconcile.DiffEntry{{Key: "http://list.txt"}},
			},
			NeedsGravity: true,
		},
	}
	w.deps.GravityOnChange = true

	err := w.Reconcile()
	if err != nil {
		t.Fatalf("Reconcile error: %v", err)
	}

	if sess.calls != 1 {
		t.Errorf("session.Get calls = %d, want 1", sess.calls)
	}
	if sett.diffCalls != 0 {
		t.Errorf("settings.Diff calls = %d, want 0 (empty settings)", sett.diffCalls)
	}
	if cont.calls != 1 {
		t.Errorf("content.Apply calls = %d, want 1", cont.calls)
	}
	if grav.calls != 1 {
		t.Errorf("gravity.TriggerNow calls = %d, want 1", grav.calls)
	}
	if rec.reconcileCalls != 1 {
		t.Errorf("recorder.RecordReconcile calls = %d, want 1", rec.reconcileCalls)
	}
	if w.Health().State != Healthy {
		t.Errorf("health = %v, want Healthy", w.Health().State)
	}
}

func TestSettingsChangeFTLRestart(t *testing.T) {
	srv := newReadyServer(t)
	w, _, sett, cont, _, _ := newTestWorker()
	w.rt.Settings = config.Settings{DNS: config.DNSSettings{ListeningMode: "all"}}

	firstClient := pihole.NewClient(srv.URL, "pw", 0)
	secondClient := pihole.NewClient(srv.URL, "pw", 0)

	customSess := &countingSessions{
		clients:  []*pihole.Client{firstClient, secondClient},
	}
	w.deps.Sessions = customSess

	sett.diffResult = &reconcile.SettingsDiff{
		Changes: []reconcile.SettingChange{{Name: "dns.listening_mode"}},
	}
	cont.report = &reconcile.ApplyReport{}

	err := w.Reconcile()
	if err != nil {
		t.Fatalf("Reconcile error: %v", err)
	}

	if customSess.getCalls < 2 {
		t.Errorf("session.Get calls = %d, want >= 2 (initial + refresh after restart)", customSess.getCalls)
	}

	if len(customSess.invalids) < 1 {
		t.Errorf("session.Invalidate calls = %d, want >= 1 (after FTL restart)", len(customSess.invalids))
	}

	if cont.calls != 1 {
		t.Errorf("content.Apply calls = %d, want 1", cont.calls)
	}
}

type countingSessions struct {
	clients  []*pihole.Client
	getCalls int
	invalids []string
}

func (c *countingSessions) Get(target config.Target) (*pihole.Client, error) {
	idx := c.getCalls
	c.getCalls++
	if idx < len(c.clients) {
		return c.clients[idx], nil
	}
	return c.clients[len(c.clients)-1], nil
}

func (c *countingSessions) Invalidate(name string) {
	c.invalids = append(c.invalids, name)
}

func (c *countingSessions) Acquire(name string)        {}
func (c *countingSessions) TryAcquire(name string) bool { return true }
func (c *countingSessions) Release(name string)         {}

func TestSessionFailureSkipsEverything(t *testing.T) {
	w, sess, sett, cont, grav, rec := newTestWorker()
	sess.err = errors.New("connection refused")

	err := w.Reconcile()
	if err == nil {
		t.Fatal("expected error")
	}

	if sett.diffCalls != 0 {
		t.Errorf("settings should not be called, got %d calls", sett.diffCalls)
	}
	if cont.calls != 0 {
		t.Errorf("content should not be called, got %d calls", cont.calls)
	}
	if grav.calls != 0 {
		t.Errorf("gravity should not be called, got %d calls", grav.calls)
	}
	if rec.unreachableCalls != 1 {
		t.Errorf("MarkTargetUnreachable calls = %d, want 1", rec.unreachableCalls)
	}
	if w.Health().State != Degraded {
		t.Errorf("health = %v, want Degraded", w.Health().State)
	}
}

func TestSettingsFailureContinuesToContent(t *testing.T) {
	w, _, sett, cont, _, _ := newTestWorker()
	w.rt.Settings = config.Settings{DNS: config.DNSSettings{ListeningMode: "all"}}
	sett.applyErr = errors.New("settings apply failed")
	sett.diffResult = &reconcile.SettingsDiff{}
	cont.report = &reconcile.ApplyReport{}

	err := w.Reconcile()
	if err != nil {
		t.Fatalf("Reconcile should succeed despite settings error: %v", err)
	}

	if sett.applyCalls != 1 {
		t.Errorf("settings.Apply calls = %d, want 1", sett.applyCalls)
	}
	if cont.calls != 1 {
		t.Errorf("content.Apply calls = %d, want 1 (should continue after settings failure)", cont.calls)
	}
}

func TestContentNeedsGravityTriggered(t *testing.T) {
	w, _, _, cont, grav, _ := newTestWorker()
	w.deps.GravityOnChange = true
	cont.report = &reconcile.ApplyReport{
		Diff: reconcile.DiffReport{NeedsGravity: true},
	}

	_ = w.Reconcile()

	if grav.calls != 1 {
		t.Errorf("gravity.TriggerNow calls = %d, want 1", grav.calls)
	}
}

func TestContentNoGravityNotTriggered(t *testing.T) {
	w, _, _, cont, grav, _ := newTestWorker()
	w.deps.GravityOnChange = true
	cont.report = &reconcile.ApplyReport{
		Diff: reconcile.DiffReport{NeedsGravity: false},
	}

	_ = w.Reconcile()

	if grav.calls != 0 {
		t.Errorf("gravity.TriggerNow calls = %d, want 0", grav.calls)
	}
}

func TestGravityCorruptionSetsDown(t *testing.T) {
	w, _, _, cont, grav, _ := newTestWorker()
	w.deps.GravityOnChange = true
	cont.err = &pihole.APIError{StatusCode: 400, Message: `no such table: group`}

	_ = w.Reconcile()

	if w.Health().State != Down {
		t.Errorf("health = %v, want Down", w.Health().State)
	}
	if grav.calls != 0 {
		t.Errorf("gravity should NOT be triggered on corruption, got %d calls", grav.calls)
	}
}

func TestUpdateConfigPreservesHealth(t *testing.T) {
	w, sess, _, _, _, _ := newTestWorker()
	sess.err = errors.New("fail")
	_ = w.Reconcile()
	_ = w.Reconcile()

	if w.Health().ConsecutiveFails != 2 {
		t.Fatalf("expected 2 failures, got %d", w.Health().ConsecutiveFails)
	}

	newRT := config.ResolvedTarget{
		Target: config.Target{Name: "test", URL: "http://new:1000", Password: "pw2"},
	}
	w.UpdateConfig(newRT)

	if w.Health().ConsecutiveFails != 2 {
		t.Errorf("UpdateConfig should preserve failure count, got %d", w.Health().ConsecutiveFails)
	}
	if w.rt.URL != "http://new:1000" {
		t.Errorf("URL should be updated, got %q", w.rt.URL)
	}
}

func TestReconcileAcquiresAndReleasesGate(t *testing.T) {
	w, sess, _, _, _, _ := newTestWorker()

	_ = w.Reconcile()

	if sess.acquireCalls != 1 {
		t.Errorf("Acquire calls = %d, want 1", sess.acquireCalls)
	}
	if sess.releaseCalls != 1 {
		t.Errorf("Release calls = %d, want 1", sess.releaseCalls)
	}
}

func TestReconcileReleasesGateOnSessionError(t *testing.T) {
	w, sess, _, _, _, _ := newTestWorker()
	sess.err = errors.New("connection refused")

	_ = w.Reconcile()

	if sess.acquireCalls != 1 {
		t.Errorf("Acquire calls = %d, want 1", sess.acquireCalls)
	}
	if sess.releaseCalls != 1 {
		t.Errorf("Release calls = %d, want 1 (gate must be released on error)", sess.releaseCalls)
	}
}

