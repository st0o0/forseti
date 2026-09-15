package session

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/st0o0/forseti/internal/config"
)

func newTestServer(loginCount *atomic.Int32) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/auth" {
			if r.Method == http.MethodPost {
				loginCount.Add(1)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"session": map[string]string{"sid": "test-sid"},
				})
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
}

func TestPoolGetCreatesSession(t *testing.T) {
	var loginCount atomic.Int32
	srv := newTestServer(&loginCount)
	defer srv.Close()

	pool := NewPool()
	defer pool.Close()

	target := config.Target{Name: "test", URL: srv.URL, Password: "pw"}

	c, err := pool.Get(target)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if !c.HasSession() {
		t.Error("client should have session after Get()")
	}
	if loginCount.Load() != 1 {
		t.Errorf("login count = %d, want 1", loginCount.Load())
	}
}

func TestPoolGetReusesSession(t *testing.T) {
	var loginCount atomic.Int32
	srv := newTestServer(&loginCount)
	defer srv.Close()

	pool := NewPool()
	defer pool.Close()

	target := config.Target{Name: "test", URL: srv.URL, Password: "pw"}

	c1, _ := pool.Get(target)
	c2, _ := pool.Get(target)

	if c1 != c2 {
		t.Error("Get() should return same client instance")
	}
	if loginCount.Load() != 1 {
		t.Errorf("login count = %d, want 1 (reused)", loginCount.Load())
	}
}

func TestPoolConcurrentAccess(t *testing.T) {
	var loginCount atomic.Int32
	srv := newTestServer(&loginCount)
	defer srv.Close()

	pool := NewPool()
	defer pool.Close()

	target := config.Target{Name: "test", URL: srv.URL, Password: "pw"}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, err := pool.Get(target)
			if err != nil {
				t.Errorf("Get() error: %v", err)
				return
			}
			if !c.HasSession() {
				t.Error("client should have session")
			}
		}()
	}
	wg.Wait()

	if got := loginCount.Load(); got != 1 {
		t.Errorf("login count = %d, want 1 (all goroutines should share)", got)
	}
}

func TestPoolSetCallbacks(t *testing.T) {
	var loginCount atomic.Int32
	srv := newTestServer(&loginCount)
	defer srv.Close()

	pool := NewPool()
	defer pool.Close()

	var newSessionCalls atomic.Int32
	var closeCalls atomic.Int32

	pool.SetCallbacks(PoolCallbacks{
		OnNewSession: func(target string) {
			newSessionCalls.Add(1)
		},
		OnClose: func() {
			closeCalls.Add(1)
		},
	})

	target := config.Target{Name: "test", URL: srv.URL, Password: "pw"}
	_, err := pool.Get(target)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}

	if got := newSessionCalls.Load(); got != 1 {
		t.Errorf("OnNewSession called %d times, want 1", got)
	}

	// Get again should reuse, no new callback
	_, _ = pool.Get(target)
	if got := newSessionCalls.Load(); got != 1 {
		t.Errorf("OnNewSession called %d times after reuse, want 1", got)
	}

	pool.Close()
	if got := closeCalls.Load(); got != 1 {
		t.Errorf("OnClose called %d times, want 1", got)
	}
}

func TestPoolSetCallbacksOnReauth(t *testing.T) {
	var loginCount atomic.Int32
	var reauthCalls atomic.Int32

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/auth" {
			if r.Method == http.MethodPost {
				loginCount.Add(1)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"session": map[string]string{"sid": "new-sid"},
				})
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}

		callCount++
		sid := r.Header.Get("X-FTL-SID")
		if callCount == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if sid == "new-sid" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"queries": map[string]any{"total": 100},
				"clients": map[string]any{"active": 5, "total": 10},
				"gravity": map[string]any{"domains_being_blocked": 50, "last_update": 0},
			})
		}
	}))
	defer srv.Close()

	pool := NewPool()
	defer pool.Close()

	pool.SetCallbacks(PoolCallbacks{
		OnReauth: func(target string) {
			reauthCalls.Add(1)
		},
	})

	target := config.Target{Name: "test", URL: srv.URL, Password: "pw"}
	client, err := pool.Get(target)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}

	_, _ = client.GetStats()

	if got := reauthCalls.Load(); got != 1 {
		t.Errorf("OnReauth called %d times, want 1", got)
	}
}

func TestPoolGetLoginError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	pool := NewPool()
	defer pool.Close()

	target := config.Target{Name: "test", URL: srv.URL, Password: "pw"}
	_, err := pool.Get(target)
	if err == nil {
		t.Fatal("Get() should error on login failure")
	}
}

func TestPoolInvalidateForcesRelogin(t *testing.T) {
	var loginCount atomic.Int32
	srv := newTestServer(&loginCount)
	defer srv.Close()

	pool := NewPool()
	defer pool.Close()

	target := config.Target{Name: "test", URL: srv.URL, Password: "pw"}

	c1, err := pool.Get(target)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}

	pool.Invalidate("test")

	c2, err := pool.Get(target)
	if err != nil {
		t.Fatalf("Get() after invalidate error: %v", err)
	}

	if c1 == c2 {
		t.Error("Get() after Invalidate should return a new client")
	}
	if loginCount.Load() != 2 {
		t.Errorf("login count = %d, want 2", loginCount.Load())
	}
}

func TestPoolInvalidateUnknownTarget(t *testing.T) {
	pool := NewPool()
	pool.Invalidate("nonexistent")
}

func TestPoolCloseWithError(t *testing.T) {
	var loginCount atomic.Int32
	srv := newTestServer(&loginCount)

	pool := NewPool()
	target := config.Target{Name: "errclose", URL: srv.URL, Password: "pw"}
	_, err := pool.Get(target)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}

	srv.Close()

	err = pool.Close()
	if err == nil {
		t.Error("Close() should return error when server is down")
	}
}

func TestPoolCloseAllSessions(t *testing.T) {
	var loginCount atomic.Int32
	srv := newTestServer(&loginCount)
	defer srv.Close()

	pool := NewPool()

	targets := []config.Target{
		{Name: "alpha", URL: srv.URL, Password: "pw"},
		{Name: "beta", URL: srv.URL, Password: "pw"},
	}

	for _, target := range targets {
		_, err := pool.Get(target)
		if err != nil {
			t.Fatalf("Get(%s) error: %v", target.Name, err)
		}
	}

	if err := pool.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	for _, target := range targets {
		c, err := pool.Get(target)
		if err != nil {
			t.Fatalf("Get(%s) after close error: %v", target.Name, err)
		}
		if !c.HasSession() {
			t.Errorf("client %s should have new session after re-Get()", target.Name)
		}
	}

	if got := loginCount.Load(); got != 4 {
		t.Errorf("login count = %d, want 4 (2 initial + 2 after close)", got)
	}
}

func TestPoolAcquireRelease(t *testing.T) {
	pool := NewPool()
	pool.SetAPIConfig(map[string]config.APIConfig{
		"test": {MaxConcurrent: 2},
	})

	pool.Acquire("test")
	pool.Acquire("test")

	if pool.TryAcquire("test") {
		t.Error("TryAcquire should fail when gate is full")
	}

	pool.Release("test")

	if !pool.TryAcquire("test") {
		t.Error("TryAcquire should succeed after Release")
	}

	pool.Release("test")
	pool.Release("test")
}

func TestPoolTryAcquireAvailable(t *testing.T) {
	pool := NewPool()
	pool.SetAPIConfig(map[string]config.APIConfig{
		"test": {MaxConcurrent: 1},
	})

	if !pool.TryAcquire("test") {
		t.Error("TryAcquire should succeed when gate is empty")
	}
	pool.Release("test")
}

func TestPoolReleaseUnknownTarget(t *testing.T) {
	pool := NewPool()
	pool.Release("nonexistent")
}

func TestPoolGateLazyCreation(t *testing.T) {
	pool := NewPool()

	if !pool.TryAcquire("lazy") {
		t.Error("TryAcquire should create gate lazily with default capacity")
	}

	for i := 0; i < 3; i++ {
		pool.TryAcquire("lazy")
	}

	if pool.TryAcquire("lazy") {
		t.Error("TryAcquire should fail at default capacity (4)")
	}

	pool.Release("lazy")
	pool.Release("lazy")
	pool.Release("lazy")
	pool.Release("lazy")
}

func TestPoolConcurrentAcquire(t *testing.T) {
	pool := NewPool()
	pool.SetAPIConfig(map[string]config.APIConfig{
		"test": {MaxConcurrent: 1},
	})

	pool.Acquire("test")

	acquired := make(chan struct{})
	go func() {
		pool.Acquire("test")
		close(acquired)
	}()

	select {
	case <-acquired:
		t.Fatal("Acquire should block when gate is full")
	case <-time.After(50 * time.Millisecond):
	}

	pool.Release("test")

	select {
	case <-acquired:
	case <-time.After(1 * time.Second):
		t.Fatal("Acquire should unblock after Release")
	}

	pool.Release("test")
}

func TestPoolSetAPIConfigUpdatesGate(t *testing.T) {
	pool := NewPool()
	pool.SetAPIConfig(map[string]config.APIConfig{
		"target": {MaxConcurrent: 2},
	})

	pool.Acquire("target")
	pool.Acquire("target")

	if pool.TryAcquire("target") {
		t.Error("should fail at capacity 2")
	}

	pool.Release("target")
	pool.Release("target")

	pool.SetAPIConfig(map[string]config.APIConfig{
		"target": {MaxConcurrent: 4},
	})

	for i := 0; i < 4; i++ {
		if !pool.TryAcquire("target") {
			t.Errorf("slot %d should succeed with new capacity 4", i+1)
		}
	}

	if pool.TryAcquire("target") {
		t.Error("should fail at new capacity 4")
	}

	for i := 0; i < 4; i++ {
		pool.Release("target")
	}
}

func TestPoolGateIndependentTargets(t *testing.T) {
	pool := NewPool()
	pool.SetAPIConfig(map[string]config.APIConfig{
		"alpha": {MaxConcurrent: 1},
		"beta":  {MaxConcurrent: 1},
	})

	pool.Acquire("alpha")

	if !pool.TryAcquire("beta") {
		t.Error("beta should be independent of alpha")
	}

	if pool.TryAcquire("alpha") {
		t.Error("alpha should be full")
	}

	pool.Release("alpha")
	pool.Release("beta")
}

func TestPoolMultipleReleaseSafe(t *testing.T) {
	pool := NewPool()
	pool.SetAPIConfig(map[string]config.APIConfig{
		"test": {MaxConcurrent: 1},
	})

	pool.Acquire("test")
	pool.Release("test")
	pool.Release("test")
	pool.Release("test")

	if !pool.TryAcquire("test") {
		t.Error("gate should work after multiple releases")
	}
	pool.Release("test")
}

func TestInvalidateCallsOnInvalidate(t *testing.T) {
	var loginCount atomic.Int32
	srv := newTestServer(&loginCount)
	defer srv.Close()

	pool := NewPool()
	defer pool.Close()

	var invalidated []string
	pool.SetCallbacks(PoolCallbacks{
		OnInvalidate: func(target string) { invalidated = append(invalidated, target) },
	})

	target := config.Target{Name: "alpha", URL: srv.URL, Password: "pw"}
	_, err := pool.Get(target)
	if err != nil {
		t.Fatal(err)
	}

	pool.Invalidate("alpha")
	if len(invalidated) != 1 || invalidated[0] != "alpha" {
		t.Errorf("OnInvalidate calls = %v, want [alpha]", invalidated)
	}

	pool.Invalidate("nonexistent")
	if len(invalidated) != 1 {
		t.Errorf("OnInvalidate should not fire for unknown target, got %d calls", len(invalidated))
	}
}
