package run

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRecordAndClassify(t *testing.T) {
	r := Result{Started: time.Now()}
	r.Record("clients", "add", 2, nil)
	r.Record("adlists", "add", 1, errors.New("create adlist https://a.example/x?token=s: 400"))
	r.Record("deny", "delete", 4, nil)
	r.Finish()

	if r.Status != Partial {
		t.Errorf("status = %q, want partial", r.Status)
	}
	tot := r.Totals()
	if tot.Added != 2 || tot.Deleted != 4 || tot.Failed != 1 {
		t.Errorf("totals = %+v", tot)
	}
	cm := r.ChangeMap()
	if cm["clients"]["add"] != 2 || cm["deny"]["delete"] != 4 {
		t.Errorf("change map = %v", cm)
	}
	if _, ok := cm["adlists"]; ok {
		t.Error("failed write must not appear in change map")
	}
	if got := r.ErrorStrings(3); len(got) != 1 || got[0] == "" || strings.Contains(got[0], "token=s") {
		t.Errorf("error strings not sanitized: %v", got)
	}
}

func TestFinishOK(t *testing.T) {
	r := Result{Started: time.Now()}
	r.Finish()
	if r.Status != OK {
		t.Errorf("status = %q, want ok", r.Status)
	}
}

func TestFailKeepsStatus(t *testing.T) {
	r := Result{Started: time.Now()}
	r.Fail(StageSession, errors.New("refused"))
	r.Finish()
	if r.Status != Failed || r.Stage != StageSession {
		t.Errorf("got %q/%q", r.Status, r.Stage)
	}
}

func TestErrorsBounded(t *testing.T) {
	var r Result
	for i := 0; i < 25; i++ {
		r.AddError(errors.New("x"))
	}
	if len(r.Errors) != maxErrors || r.ErrorCount != 25 {
		t.Errorf("errors=%d count=%d", len(r.Errors), r.ErrorCount)
	}
}

func TestMetricStatus(t *testing.T) {
	if OK.MetricStatus() != "success" || Partial.MetricStatus() != "partial" || Failed.MetricStatus() != "error" || Skipped.MetricStatus() != "" {
		t.Error("unexpected metric status mapping")
	}
}

func TestCycleAdd(t *testing.T) {
	var c Cycle
	ok := Result{Status: OK}
	ok.Record("clients", "add", 3, nil)
	c.Add(ok)
	c.Add(Result{Status: Skipped})
	c.Add(Result{Status: Failed})
	if c.Targets != 3 || c.OK != 1 || c.Skipped != 1 || c.Failed != 1 || c.Changes != 3 {
		t.Errorf("cycle = %+v", c)
	}
}
