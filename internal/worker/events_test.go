package worker

import (
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/st0o0/forseti/internal/logging/logtest"
	"github.com/st0o0/forseti/internal/reconcile"
	"github.com/st0o0/forseti/internal/run"
)

func TestHealthTransitionsAreLoggedOnce(t *testing.T) {
	rec := logtest.New(t)
	w, sess, _, _, _, _ := newTestWorker()
	w.log = rec.Logger()
	sess.err = errors.New("connection refused")

	for i := 0; i < downThreshold+2; i++ {
		_ = w.Reconcile()
	}

	if n := len(rec.Find("target degraded")); n != 1 {
		t.Errorf("degraded events = %d, want 1", n)
	}
	down := rec.Find("target down")
	if len(down) != 1 {
		t.Fatalf("down events = %d, want 1", len(down))
	}
	if down[0].Level != slog.LevelError || down[0].Attrs["consecutive_failures"] != int64(downThreshold) {
		t.Errorf("unexpected down event %+v", down[0])
	}
	if w.Health().State != Down {
		t.Errorf("health = %v, want Down", w.Health().State)
	}

	sess.err = nil
	if err := w.Reconcile(); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	recovered := rec.Find("target recovered")
	if len(recovered) != 1 || recovered[0].Attrs["failures"] != int64(downThreshold+2) {
		t.Errorf("recovered events = %+v", recovered)
	}
}

func TestDegradedBelowThreshold(t *testing.T) {
	logtest.New(t)
	w, sess, _, _, _, _ := newTestWorker()
	sess.err = errors.New("refused")
	for i := 0; i < downThreshold-1; i++ {
		_ = w.Reconcile()
	}
	if w.Health().State != Degraded {
		t.Errorf("health = %v, want Degraded", w.Health().State)
	}
}

func TestRunFinishedLevels(t *testing.T) {
	cases := []struct {
		name   string
		report *reconcile.ApplyReport
		err    error
		level  slog.Level
		status run.Status
	}{
		{"no changes", &reconcile.ApplyReport{}, nil, slog.LevelDebug, run.OK},
		{"changes", &reconcile.ApplyReport{Diff: reconcile.DiffReport{Clients: reconcile.ResourceDiff{Adds: []reconcile.DiffEntry{{Key: "10.0.0.1"}}}}}, nil, slog.LevelInfo, run.OK},
		{"partial", &reconcile.ApplyReport{
			Diff:   reconcile.DiffReport{Adlists: reconcile.ResourceDiff{Adds: []reconcile.DiffEntry{{Key: "https://a"}}}},
			Errors: []error{&reconcile.ItemError{Resource: "adlists", Action: "add", Key: "https://a", Count: 1, Err: errors.New("x")}},
		}, nil, slog.LevelWarn, run.Partial},
		{"failed", nil, errors.New("boom"), slog.LevelError, run.Failed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := logtest.New(t)
			w, _, _, cont, _, _ := newTestWorker()
			w.log = rec.Logger()
			cont.report, cont.err = tc.report, tc.err

			res := w.Run(&ConfigRunner{w: w})

			if res.Status != tc.status {
				t.Errorf("status = %q, want %q", res.Status, tc.status)
			}
			ev := rec.Find("run finished")
			if len(ev) != 1 || ev[0].Level != tc.level {
				t.Fatalf("run finished events = %+v, want one at %v", ev, tc.level)
			}
		})
	}
}

func TestAppliedCountsExcludeFailedItems(t *testing.T) {
	logtest.New(t)
	w, _, _, cont, _, rec := newTestWorker()
	cont.report = &reconcile.ApplyReport{
		Diff: reconcile.DiffReport{
			Adlists: reconcile.ResourceDiff{Adds: []reconcile.DiffEntry{{Key: "a"}, {Key: "b"}, {Key: "c"}}},
			Deny:    reconcile.ResourceDiff{Deletes: []reconcile.DiffEntry{{Key: "x"}, {Key: "y"}}},
		},
		Errors: []error{
			&reconcile.ItemError{Resource: "adlists", Action: "add", Key: "b", Count: 1, Err: errors.New("fail")},
			&reconcile.ItemError{Resource: "deny", Action: "delete", Count: 2, Err: errors.New("batch fail")},
		},
	}

	_ = w.Reconcile()

	cm := rec.lastResult.ChangeMap()
	if cm["adlists"]["add"] != 2 {
		t.Errorf("adlists add = %d, want 2", cm["adlists"]["add"])
	}
	if _, ok := cm["deny"]; ok {
		t.Errorf("failed batch delete counted: %v", cm["deny"])
	}
	if rec.lastResult.Resources["deny"].Failed != 2 {
		t.Errorf("deny failed = %d, want 2", rec.lastResult.Resources["deny"].Failed)
	}
}

func TestReconcileWorkersSkipsBackoffAndLogsCycle(t *testing.T) {
	rec := logtest.New(t)
	ok, _, _, _, _, _ := newTestWorker()
	ok.rt.Name = "a"
	backoff, _, _, cont, _, _ := newTestWorker()
	backoff.rt.Name = "b"
	backoff.backoffUntil = time.Now().Add(time.Hour)

	next := time.Now().Add(time.Minute)
	cycle := ReconcileWorkers(map[string]*TargetWorker{"a": ok, "b": backoff}, next)

	if cycle.OK != 1 || cycle.Skipped != 1 || cycle.Targets != 2 {
		t.Errorf("cycle = %+v", cycle)
	}
	if cont.calls != 0 {
		t.Error("worker in backoff must not run")
	}
	ev := rec.Find("cycle complete")
	if len(ev) != 1 || ev[0].Level != slog.LevelInfo || ev[0].Attrs["next_run"] != next.Format(time.RFC3339) {
		t.Errorf("cycle complete events = %+v", ev)
	}
}
