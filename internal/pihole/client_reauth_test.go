package pihole

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestAutoReauthOn401(t *testing.T) {
	var loginCount atomic.Int32
	var callCount atomic.Int32

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

		callCount.Add(1)
		sid := r.Header.Get("X-FTL-SID")

		if sid != "new-sid" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		json.NewEncoder(w).Encode(map[string]any{
			"queries": map[string]any{"total": 100},
			"clients": map[string]any{"active": 5, "total": 10},
			"gravity": map[string]any{"domains_being_blocked": 50, "last_update": 0},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-password")
	c.sid = "expired-sid"

	stats, err := c.GetStats()
	if err != nil {
		t.Fatalf("GetStats() error: %v", err)
	}
	if stats.QueriesTotal != 100 {
		t.Errorf("QueriesTotal = %d, want 100", stats.QueriesTotal)
	}

	if got := loginCount.Load(); got != 1 {
		t.Errorf("login count = %d, want 1", got)
	}
	if got := callCount.Load(); got != 2 {
		t.Errorf("call count = %d, want 2 (first 401 + retry)", got)
	}
}

func TestOnReauthCallbackCalled(t *testing.T) {
	var reauthCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/auth" {
			if r.Method == http.MethodPost {
				json.NewEncoder(w).Encode(map[string]any{
					"session": map[string]string{"sid": "new-sid"},
				})
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}

		sid := r.Header.Get("X-FTL-SID")
		if sid != "new-sid" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		json.NewEncoder(w).Encode(map[string]any{
			"queries": map[string]any{"total": 100},
			"clients": map[string]any{"active": 5, "total": 10},
			"gravity": map[string]any{"domains_being_blocked": 50, "last_update": 0},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-password")
	c.sid = "expired-sid"
	c.OnReauth = func() { reauthCount.Add(1) }

	_, err := c.GetStats()
	if err != nil {
		t.Fatalf("GetStats() error: %v", err)
	}

	if got := reauthCount.Load(); got != 1 {
		t.Errorf("OnReauth called %d times, want 1", got)
	}
}

func TestOnReauthNotCalledOnFailure(t *testing.T) {
	var reauthCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "wrong")
	c.sid = "expired-sid"
	c.OnReauth = func() { reauthCount.Add(1) }

	_, _ = c.GetStats()

	if got := reauthCount.Load(); got != 0 {
		t.Errorf("OnReauth called %d times on failed reauth, want 0", got)
	}
}

func TestNoReauthOnLoginPath(t *testing.T) {
	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "wrong-password")
	err := c.Login()
	if err == nil {
		t.Fatal("Login() should have failed")
	}
	if got := callCount.Load(); got != 1 {
		t.Errorf("call count = %d, want 1 (no retry loop on /api/auth)", got)
	}
}

func TestNoReauthWithoutPassword(t *testing.T) {
	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "")
	c.sid = "some-sid"

	_, err := c.GetStats()
	if err == nil {
		t.Fatal("GetStats() should error on 401 without password")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != 401 {
		t.Errorf("StatusCode = %d, want 401", apiErr.StatusCode)
	}
	if got := callCount.Load(); got != 1 {
		t.Errorf("call count = %d, want 1 (no retry without password)", got)
	}
}
