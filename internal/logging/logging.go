// Package logging builds Forseti's process-wide slog handler and hands out
// per-component loggers.
//
// The handler is created once by Setup and reads its level from a shared
// slog.LevelVar, so SetLevel takes effect for every logger handed out
// earlier, including those captured by long-lived components.
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
)

var (
	mu     sync.Mutex
	level  = new(slog.LevelVar)
	format = "text"
)

// Setup installs the default logger with the given format ("text" or "json")
// and level, writing to out.
func Setup(fmtName string, lvl slog.Level, out io.Writer) {
	mu.Lock()
	defer mu.Unlock()

	level.Set(lvl)
	opts := &slog.HandlerOptions{Level: level}
	var h slog.Handler
	if fmtName == "json" {
		h = slog.NewJSONHandler(out, opts)
		format = "json"
	} else {
		h = slog.NewTextHandler(out, opts)
		format = "text"
	}
	slog.SetDefault(slog.New(h))
}

// SetLevel changes the level of every logger created from the default handler.
func SetLevel(lvl slog.Level) {
	level.Set(lvl)
}

// Format returns the format chosen by the last Setup call.
func Format() string {
	mu.Lock()
	defer mu.Unlock()
	return format
}

// Component returns the default logger tagged with component=name.
func Component(name string) *slog.Logger {
	return slog.Default().With("component", name)
}

// ParseLevel maps a config log_level value to a slog level.
func ParseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug, nil
	case "", "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	}
	return slog.LevelInfo, fmt.Errorf("unknown log level %q", s)
}
