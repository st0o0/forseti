package metrics

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// liveness tracks the watch loop heartbeat for /healthz.
type liveness struct {
	mu       sync.RWMutex
	started  time.Time
	last     time.Time
	interval time.Duration
}

// Heartbeat records that the watch loop completed a cycle.
func (s *Server) Heartbeat() {
	s.liveness.mu.Lock()
	s.liveness.last = time.Now()
	s.liveness.mu.Unlock()
}

// SetLivenessInterval sets the loop interval the stall threshold is derived
// from: max(3*interval, interval+2m).
func (s *Server) SetLivenessInterval(d time.Duration) {
	s.liveness.mu.Lock()
	s.liveness.interval = d
	if s.liveness.started.IsZero() {
		s.liveness.started = time.Now()
	}
	s.liveness.mu.Unlock()
}

func stallThreshold(interval time.Duration) time.Duration {
	a, b := 3*interval, interval+2*time.Minute
	if a > b {
		return a
	}
	return b
}

// Alive reports whether the loop has made progress within the threshold.
// Before the first heartbeat the process start counts as progress.
func (s *Server) Alive(now time.Time) bool {
	s.liveness.mu.RLock()
	defer s.liveness.mu.RUnlock()
	ref := s.liveness.last
	if ref.IsZero() {
		ref = s.liveness.started
	}
	if ref.IsZero() || s.liveness.interval <= 0 {
		return true
	}
	return now.Sub(ref) <= stallThreshold(s.liveness.interval)
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	status, code := "ok", http.StatusOK
	if !s.Alive(time.Now()) {
		status, code = "stalled", http.StatusServiceUnavailable
	}
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": status})
}
