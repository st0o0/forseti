package session

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/st0o0/forseti/internal/config"
)

func newTestServer(loginCount *atomic.Int32) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/auth" {
			if r.Method == http.MethodPost {
				loginCount.Add(1)
				json.NewEncoder(w).Encode(map[string]any{
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
				json.NewEncoder(w).Encode(map[string]any{
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
			json.NewEncoder(w).Encode(map[string]any{
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
