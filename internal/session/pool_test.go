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
