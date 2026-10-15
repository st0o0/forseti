package redact

import (
	"strings"
	"testing"
)

func TestURL(t *testing.T) {
	tests := map[string]string{
		"https://lists.example.org/x.txt?token=abc123":     "https://lists.example.org/x.txt?token=REDACTED",
		"https://user:pw@host.example/list.txt":            "https://host.example/list.txt",
		"https://a.example/l.txt?b=2&a=1":                  "https://a.example/l.txt?a=REDACTED&b=REDACTED",
		"https://raw.githubusercontent.com/x/y/main/hosts": "https://raw.githubusercontent.com/x/y/main/hosts",
	}
	for in, want := range tests {
		if got := URL(in); got != want {
			t.Errorf("URL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTextRedactsEmbeddedURLs(t *testing.T) {
	in := `create adlist "https://lists.example.org/x.txt?token=abc123": pihole api 400`
	got := Text(in)
	if strings.Contains(got, "abc123") {
		t.Errorf("token leaked: %q", got)
	}
	if !strings.Contains(got, "token=REDACTED") {
		t.Errorf("expected redacted token, got %q", got)
	}
}

func TestTextCollapsesAndTruncates(t *testing.T) {
	got := Text("line one\n\nline two " + strings.Repeat("x", 400))
	if strings.Contains(got, "\n") {
		t.Error("newlines not collapsed")
	}
	if len([]rune(got)) > maxTextRunes+3 {
		t.Errorf("not truncated: %d runes", len([]rune(got)))
	}
}
