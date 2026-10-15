package pihole

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/st0o0/forseti/internal/logging/logtest"
)

func TestAPIErrorJSONRendering(t *testing.T) {
	e := parseAPIError(400, []byte(`{"error":{"key":"bad_request","message":"Invalid domain","hint":null}}`))
	if got := e.Error(); got != "pihole api: 400 bad_request: Invalid domain" {
		t.Errorf("Error() = %q", got)
	}
	if !strings.Contains(e.Message, "bad_request") {
		t.Error("raw body must stay in Message")
	}
}

func TestAPIErrorTruncatesHTMLBody(t *testing.T) {
	body := "<html>\n<body>" + strings.Repeat("Bad Gateway ", 500) + "</body></html>"
	got := parseAPIError(502, []byte(body)).Error()
	if strings.Contains(got, "\n") {
		t.Error("newlines not collapsed")
	}
	if len(got) > len("pihole api: 502 ")+maxErrorBody+3 {
		t.Errorf("not truncated: %d bytes", len(got))
	}
}

func TestRequestDebugLogCarriesStatusAndTarget(t *testing.T) {
	rec := logtest.New(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"groups":[]}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "", 0)
	c.Log = rec.Logger().With("component", "pihole", "target", "pi")
	if _, err := c.ListGroups(); err != nil {
		t.Fatal(err)
	}

	entries := rec.Find("api request")
	if len(entries) != 1 {
		t.Fatalf("api request entries = %d, want 1", len(entries))
	}
	a := entries[0].Attrs
	if a["target"] != "pi" || a["status"] != int64(200) || a["path"] != "/api/groups" || a["duration"] == nil {
		t.Errorf("unexpected attrs %v", a)
	}
}
