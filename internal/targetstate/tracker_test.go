package targetstate

import (
	"errors"
	"testing"

	"github.com/st0o0/forseti/internal/logging/logtest"
)

type fakeGauge struct{ up, down int }

func (f *fakeGauge) MarkTargetReachable(string)   { f.up++ }
func (f *fakeGauge) MarkTargetUnreachable(string) { f.down++ }

func TestObserveLogsTransitionsOnly(t *testing.T) {
	rec := logtest.New(t)
	g := &fakeGauge{}
	tr := New(g)

	refused := errors.New("dial tcp 10.0.0.2:80: connection refused")
	for i := 0; i < 5; i++ {
		tr.Observe("pi", "collector", refused)
	}
	tr.Observe("pi", "worker", nil)
	tr.Observe("pi", "collector", nil)

	if n := len(rec.Find("target unreachable")); n != 1 {
		t.Errorf("unreachable events = %d, want 1", n)
	}
	if n := len(rec.Find("target connected")); n != 1 {
		t.Errorf("connected events = %d, want 1", n)
	}
	if e := rec.Find("target unreachable")[0]; e.Attrs["source"] != "collector" || e.Attrs["component"] != "session" {
		t.Errorf("unexpected attrs %v", e.Attrs)
	}
	if g.down != 5 || g.up != 2 {
		t.Errorf("gauge updates down=%d up=%d", g.down, g.up)
	}
}

func TestNilTrackerIsNoop(t *testing.T) {
	var tr *Tracker
	tr.Observe("pi", "worker", errors.New("x"))
	tr.Forget(nil)
}

func TestForget(t *testing.T) {
	logtest.New(t)
	tr := New(nil)
	tr.Observe("a", "worker", nil)
	tr.Observe("b", "worker", nil)
	tr.Forget(map[string]bool{"a": true})
	if _, ok := tr.states["b"]; ok {
		t.Error("b not forgotten")
	}
}
