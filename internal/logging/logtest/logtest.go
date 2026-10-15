// Package logtest captures slog output in tests.
package logtest

import (
	"context"
	"log/slog"
	"sync"
	"testing"
)

// Entry is one captured log record with its attributes flattened.
type Entry struct {
	Level slog.Level
	Msg   string
	Attrs map[string]any
}

// Recorder collects log records.
type Recorder struct {
	mu      sync.Mutex
	entries []Entry
}

// New installs a recording handler as the default logger for the duration
// of the test (tests using it must not run in parallel) and returns the
// recorder. Loggers obtained via logging.Component after this call record.
func New(t testing.TB) *Recorder {
	t.Helper()
	r := &Recorder{}
	prev := slog.Default()
	slog.SetDefault(slog.New(&handler{rec: r}))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return r
}

// Logger returns a logger writing to this recorder.
func (r *Recorder) Logger() *slog.Logger {
	return slog.New(&handler{rec: r})
}

// Entries returns a copy of all captured entries.
func (r *Recorder) Entries() []Entry {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Entry, len(r.entries))
	copy(out, r.entries)
	return out
}

// Find returns all entries with the given message.
func (r *Recorder) Find(msg string) []Entry {
	var out []Entry
	for _, e := range r.Entries() {
		if e.Msg == msg {
			out = append(out, e)
		}
	}
	return out
}

type handler struct {
	rec   *Recorder
	attrs []slog.Attr
	group string
}

func (h *handler) Enabled(context.Context, slog.Level) bool { return true }

func (h *handler) Handle(_ context.Context, r slog.Record) error {
	e := Entry{Level: r.Level, Msg: r.Message, Attrs: map[string]any{}}
	for _, a := range h.attrs {
		addAttr(e.Attrs, "", a)
	}
	r.Attrs(func(a slog.Attr) bool {
		addAttr(e.Attrs, h.group, a)
		return true
	})
	h.rec.mu.Lock()
	h.rec.entries = append(h.rec.entries, e)
	h.rec.mu.Unlock()
	return nil
}

func addAttr(m map[string]any, prefix string, a slog.Attr) {
	v := a.Value.Resolve()
	key := a.Key
	if prefix != "" {
		key = prefix + "." + key
	}
	if v.Kind() == slog.KindGroup {
		for _, ga := range v.Group() {
			p := key
			if a.Key == "" {
				p = prefix
			}
			addAttr(m, p, ga)
		}
		return
	}
	m[key] = v.Any()
}

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	nh := *h
	nh.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &nh
}

func (h *handler) WithGroup(name string) slog.Handler {
	nh := *h
	if nh.group != "" {
		nh.group += "." + name
	} else {
		nh.group = name
	}
	return &nh
}
