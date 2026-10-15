package reconcile

import (
	"errors"
	"strings"
	"testing"

	"github.com/st0o0/forseti/internal/config"
)

func TestItemErrorRedactsAdlistKey(t *testing.T) {
	err := itemErr("adlists", "add", "https://l.example/x.txt?token=abc", errors.New("boom"))
	if strings.Contains(err.Error(), "abc") {
		t.Errorf("token leaked: %q", err.Error())
	}
	if err.Count != 1 || err.Resource != "adlists" || err.Action != "add" {
		t.Errorf("unexpected item error %+v", err)
	}
	if !errors.Is(err, err.Err) {
		t.Error("ItemError must unwrap")
	}
}

func TestBatchErr(t *testing.T) {
	err := batchErr("deny", 4, errors.New("boom"))
	if err.Count != 4 || err.Action != "delete" {
		t.Errorf("unexpected batch error %+v", err)
	}
	if !strings.Contains(err.Error(), "4 items") {
		t.Errorf("message = %q", err.Error())
	}
}

func TestApplyRecordsItemErrors(t *testing.T) {
	// Every item-level error from Apply must be an *ItemError so callers can
	// compute applied change counts.
	m := newMockAPI()
	m.createAdlistErr = errors.New("create failed")
	m.createDomainErr = errors.New("create failed")
	cfg := &config.Config{
		Adlists: []config.Adlist{{URL: "https://x.com/l.txt"}},
		Deny:    []config.DenyEntry{{Domain: "bad.com"}},
	}

	report, err := Apply(makeRT(cfg, config.Target{Name: "t"}), m, makeOpts("[forseti]"))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(report.Errors) == 0 {
		t.Fatal("expected item errors")
	}
	for _, e := range report.Errors {
		var ie *ItemError
		if !errors.As(e, &ie) {
			t.Errorf("error %v is not an ItemError", e)
		}
	}
}

func TestSensitiveSettingDetection(t *testing.T) {
	for _, m := range []SettingMapping{
		{PiholePath: "webserver/api/password"},
		{PiholePath: "webserver/api/pwhash"},
		{PiholePath: "webserver/api/totp_secret"},
		{PiholePath: "misc/token"},
		{PiholePath: "dns/upstreams", Sensitive: true},
	} {
		if !m.IsSensitive() {
			t.Errorf("%s must be sensitive", m.PiholePath)
		}
	}
	if (SettingMapping{PiholePath: "dns/upstreams"}).IsSensitive() {
		t.Error("dns/upstreams must not be sensitive")
	}
	c := SettingChange{Current: "a", Desired: "b", Sensitive: true}
	if c.DisplayCurrent() != redactedValue || c.DisplayDesired() != redactedValue {
		t.Error("sensitive values must be redacted")
	}
}
