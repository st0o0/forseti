package logging

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestSetLevelAppliesToExistingLoggers(t *testing.T) {
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })

	var buf bytes.Buffer
	Setup("text", slog.LevelInfo, &buf)
	log := Component("gravity")

	log.Debug("hidden")
	SetLevel(slog.LevelDebug)
	log.Debug("visible")

	out := buf.String()
	if strings.Contains(out, "hidden") {
		t.Error("debug line emitted at info level")
	}
	if !strings.Contains(out, "visible") || !strings.Contains(out, "component=gravity") {
		t.Errorf("expected debug line with component after SetLevel, got %q", out)
	}
}

func TestSetupJSON(t *testing.T) {
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })

	var buf bytes.Buffer
	Setup("json", slog.LevelInfo, &buf)
	Component("main").Info("forseti starting", "version", "1.0.0")

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("not JSON: %v (%q)", err, buf.String())
	}
	if m["component"] != "main" || m["msg"] != "forseti starting" {
		t.Errorf("unexpected record %v", m)
	}
	if Format() != "json" {
		t.Errorf("Format() = %q, want json", Format())
	}
}

func TestParseLevel(t *testing.T) {
	for in, want := range map[string]slog.Level{"debug": slog.LevelDebug, "": slog.LevelInfo, "info": slog.LevelInfo, "warn": slog.LevelWarn, "error": slog.LevelError} {
		got, err := ParseLevel(in)
		if err != nil || got != want {
			t.Errorf("ParseLevel(%q) = %v, %v", in, got, err)
		}
	}
	if _, err := ParseLevel("verbose"); err == nil {
		t.Error("expected error for invalid level")
	}
}
