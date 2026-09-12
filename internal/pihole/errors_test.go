package pihole

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"syscall"
	"testing"
	"time"
)

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"api 404", &APIError{StatusCode: 404, Message: "not found"}, true},
		{"api 500", &APIError{StatusCode: 500, Message: "internal"}, false},
		{"api 400", &APIError{StatusCode: 400, Message: "bad"}, false},
		{"wrapped 404", fmt.Errorf("delete: %w", &APIError{StatusCode: 404, Message: "gone"}), true},
		{"non-api error", fmt.Errorf("connection reset"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNotFound(tt.err); got != tt.want {
				t.Errorf("IsNotFound() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsTransient(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"api 503", &APIError{StatusCode: 503, Message: "unavailable"}, true},
		{"api 400 database_error", &APIError{StatusCode: 400, Message: `{"error":{"key":"database_error","message":"Could not read"}}`}, true},
		{"api 400 Database not available", &APIError{StatusCode: 400, Message: `Database not available`}, true},
		{"api 400 bad_request", &APIError{StatusCode: 400, Message: `{"error":{"key":"bad_request","message":"Invalid domain"}}`}, false},
		{"api 404", &APIError{StatusCode: 404, Message: "not found"}, false},
		{"api 500", &APIError{StatusCode: 500, Message: "internal"}, false},
		{"connection refused", &net.OpError{Op: "dial", Err: &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}}, true},
		{"connection reset", &net.OpError{Op: "read", Err: &net.OpError{Op: "read", Err: syscall.ECONNRESET}}, true},
		{"wrapped transient", fmt.Errorf("list: %w", &APIError{StatusCode: 503, Message: "unavailable"}), true},
		{"wrapped non-transient", fmt.Errorf("list: %w", &APIError{StatusCode: 500, Message: "broken"}), false},
		{"non-api error", fmt.Errorf("something else"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsTransient(tt.err); got != tt.want {
				t.Errorf("IsTransient() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseAPIError(t *testing.T) {
	t.Run("structured error", func(t *testing.T) {
		body := []byte(`{"error":{"key":"database_error","message":"Could not read","hint":"no such table: group"},"took":0.001}`)
		ae := parseAPIError(400, body)
		if ae.Key != "database_error" {
			t.Errorf("Key = %q, want database_error", ae.Key)
		}
		if ae.Hint != "no such table: group" {
			t.Errorf("Hint = %q, want 'no such table: group'", ae.Hint)
		}
		if ae.StatusCode != 400 {
			t.Errorf("StatusCode = %d, want 400", ae.StatusCode)
		}
	})

	t.Run("plain text error", func(t *testing.T) {
		body := []byte(`not json`)
		ae := parseAPIError(500, body)
		if ae.Key != "" {
			t.Errorf("Key should be empty for non-JSON, got %q", ae.Key)
		}
		if ae.Message != "not json" {
			t.Errorf("Message = %q, want 'not json'", ae.Message)
		}
	})

	t.Run("structured used by IsTransient", func(t *testing.T) {
		body := []byte(`{"error":{"key":"database_error","message":"Could not read domains","hint":"Database not available"},"took":0.001}`)
		ae := parseAPIError(400, body)
		if !IsTransient(ae) {
			t.Error("parseAPIError with database_error key should be transient")
		}
	})

	t.Run("structured used by IsGravityCorrupted", func(t *testing.T) {
		body := []byte(`{"error":{"key":"database_error","message":"Could not read","hint":"no such table: group"},"took":0.001}`)
		ae := parseAPIError(400, body)
		if !IsGravityCorrupted(ae) {
			t.Error("parseAPIError with 'no such table' hint should be gravity corrupted")
		}
	})
}

func TestIsGravityCorrupted(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"no such table group", &APIError{StatusCode: 400, Message: `{"error":{"key":"database_error","message":"Could not read","hint":"no such table: group"}}`}, true},
		{"no such table gravity", &APIError{StatusCode: 400, Message: `{"error":{"key":"database_error","message":"Could not remove","hint":"no such table: gravity"}}`}, true},
		{"database not available", &APIError{StatusCode: 400, Message: `Database not available`}, false},
		{"readonly database", &APIError{StatusCode: 400, Message: `attempt to write a readonly database`}, false},
		{"api 500", &APIError{StatusCode: 500, Message: "internal"}, false},
		{"non-api error", fmt.Errorf("connection refused"), false},
		{"wrapped corruption", fmt.Errorf("list: %w", &APIError{StatusCode: 400, Message: `no such table: group`}), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsGravityCorrupted(tt.err); got != tt.want {
				t.Errorf("IsGravityCorrupted() = %v, want %v", got, tt.want)
			}
		})
	}
}

func gravityServer(t *testing.T, delay time.Duration) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/auth" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]any{"sid": "s"},
			})
			return
		}
		if r.Method == http.MethodDelete && r.URL.Path == "/api/auth" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.URL.Path == "/api/action/gravity" {
			time.Sleep(delay)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	c := NewClient(srv.URL, "pw")
	_ = c.Login()
	return c
}

func TestGravityExtendedTimeout(t *testing.T) {
	c := gravityServer(t, 2*time.Second)
	c.GravityTimeout = 10 * time.Second
	c.httpClient.Timeout = 1 * time.Second

	if err := c.TriggerGravity(); err != nil {
		t.Fatalf("TriggerGravity() should succeed with extended timeout, got: %v", err)
	}

	if c.httpClient.Timeout != 1*time.Second {
		t.Errorf("original timeout should be restored, got %v", c.httpClient.Timeout)
	}
}

func TestGravityTimeoutExpired(t *testing.T) {
	c := gravityServer(t, 3*time.Second)
	c.GravityTimeout = 500 * time.Millisecond

	err := c.TriggerGravity()
	if err == nil {
		t.Fatal("TriggerGravity() should timeout")
	}
}

func readinessServer(t *testing.T, ftlResponse string, ftlStatus int) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/auth" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]any{"sid": "s"},
			})
			return
		}
		if r.Method == http.MethodDelete && r.URL.Path == "/api/auth" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.URL.Path == "/api/info/ftl" {
			w.WriteHeader(ftlStatus)
			_, _ = w.Write([]byte(ftlResponse))
			return
		}
		if r.URL.Path == "/api/info/login" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"dns":true}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	c := NewClient(srv.URL, "pw")
	_ = c.Login()
	return c
}

func TestCheckReadinessSuccess(t *testing.T) {
	c := readinessServer(t, `{"ftl":{"database":{"gravity":100,"groups":2},"pid":42,"uptime":300}}`, 200)

	if err := c.CheckReadiness(); err != nil {
		t.Fatalf("CheckReadiness() should succeed: %v", err)
	}
}

func TestCheckReadinessPidZero(t *testing.T) {
	c := readinessServer(t, `{"ftl":{"database":{"gravity":0,"groups":0},"pid":0,"uptime":0}}`, 200)

	err := c.CheckReadiness()
	if err == nil {
		t.Fatal("CheckReadiness() should fail with pid=0")
	}
}

func TestCheckReadinessConnectionRefused(t *testing.T) {
	c := NewClient("http://127.0.0.1:1", "pw")

	err := c.CheckReadiness()
	if err == nil {
		t.Fatal("CheckReadiness() should fail on unreachable server")
	}
}

func TestCheckAliveSuccess(t *testing.T) {
	c := readinessServer(t, "", 200)

	if err := c.CheckAlive(); err != nil {
		t.Fatalf("CheckAlive() should succeed: %v", err)
	}
}

func TestCheckAliveUnreachable(t *testing.T) {
	c := NewClient("http://127.0.0.1:1", "pw")

	err := c.CheckAlive()
	if err == nil {
		t.Fatal("CheckAlive() should fail on unreachable server")
	}
}
