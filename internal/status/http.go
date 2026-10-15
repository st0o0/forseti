package status

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

const (
	defaultSince = 24 * time.Hour
	defaultLimit = 500
)

type statusDoc struct {
	Version       string      `json:"version"`
	Mode          string      `json:"mode"`
	StartedAt     time.Time   `json:"started_at"`
	UptimeSeconds int64       `json:"uptime_seconds"`
	Config        configDoc   `json:"config"`
	Cycle         cycleDoc    `json:"cycle"`
	Targets       []targetDoc `json:"targets"`
}

type configDoc struct {
	LoadedAt   time.Time  `json:"loaded_at"`
	LastReload *reloadDoc `json:"last_reload,omitempty"`
}

type reloadDoc struct {
	At    time.Time `json:"at"`
	OK    bool      `json:"ok"`
	Error string    `json:"error,omitempty"`
}

type cycleDoc struct {
	IntervalSeconds int64       `json:"interval_seconds"`
	Last            *CycleEntry `json:"last,omitempty"`
	NextRunAt       *time.Time  `json:"next_run_at,omitempty"`
}

type errorDoc struct {
	At      time.Time `json:"at"`
	Message string    `json:"message"`
}

type targetDoc struct {
	Name                string     `json:"name"`
	Role                string     `json:"role,omitempty"`
	Health              string     `json:"health"`
	ConsecutiveFailures int        `json:"consecutive_failures"`
	LastSuccessAt       *time.Time `json:"last_success_at,omitempty"`
	BackoffUntil        *time.Time `json:"backoff_until,omitempty"`
	LastError           *errorDoc  `json:"last_error,omitempty"`
	LastRun             *Entry     `json:"last_run,omitempty"`
}

func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

// snapshot builds the /status document.
func (s *Store) snapshot() statusDoc {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now()
	doc := statusDoc{
		Version:       s.version,
		Mode:          s.mode,
		StartedAt:     s.started,
		UptimeSeconds: int64(now.Sub(s.started).Seconds()),
		Config:        configDoc{LoadedAt: s.loadedAt},
		Cycle:         cycleDoc{IntervalSeconds: int64(s.interval.Seconds())},
		Targets:       []targetDoc{},
	}
	if s.reload != nil {
		doc.Config.LastReload = &reloadDoc{At: s.reload.at, OK: s.reload.ok, Error: s.reload.err}
	}
	if c, ok := s.cycles.last(); ok {
		doc.Cycle.Last = &c
		doc.Cycle.NextRunAt = timePtr(c.NextRun)
	}
	for _, name := range s.targetNames() {
		ts := s.targets[name]
		td := targetDoc{
			Name:                name,
			Role:                ts.role,
			Health:              ts.state.Health,
			ConsecutiveFailures: ts.state.ConsecutiveFailures,
			LastSuccessAt:       timePtr(ts.state.LastSuccess),
		}
		if ts.state.BackoffUntil.After(now) {
			td.BackoffUntil = timePtr(ts.state.BackoffUntil)
		}
		if ts.lastError != "" {
			td.LastError = &errorDoc{At: ts.lastErrorAt, Message: ts.lastError}
		}
		if e, ok := ts.history.last(); ok {
			td.LastRun = &e
		}
		doc.Targets = append(doc.Targets, td)
	}
	return doc
}

// Handler serves /status and /status/history.
func Handler(s *Store) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, s.snapshot())
	})
	mux.HandleFunc("/status/history", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		q := r.URL.Query()
		since := defaultSince
		if v := q.Get("since"); v != "" {
			d, err := time.ParseDuration(v)
			if err != nil || d <= 0 {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid since"})
				return
			}
			since = d
		}
		limit := defaultLimit
		if v := q.Get("limit"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n <= 0 {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid limit"})
				return
			}
			limit = min(n, DefaultCapacity)
		}

		target := q.Get("target")
		if target == "" {
			writeJSON(w, http.StatusOK, map[string]any{"cycles": s.Cycles(since, limit)})
			return
		}
		entries, ok := s.History(target, since, limit)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown target"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"target": target, "entries": entries})
	})
	return mux
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
