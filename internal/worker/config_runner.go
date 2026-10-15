package worker

import (
	"errors"
	"time"

	"github.com/st0o0/forseti/internal/pihole"
	"github.com/st0o0/forseti/internal/reconcile"
	"github.com/st0o0/forseti/internal/run"
)

// ConfigRunner is the config-mode step of a run: reconcile settings, then
// content, then trigger gravity if adlists changed.
type ConfigRunner struct {
	w *TargetWorker
}

func (c *ConfigRunner) Run(client *pihole.Client, res *run.Result) error {
	w := c.w

	client, err := w.reconcileSettings(client)
	if err != nil {
		return StageError(run.StageSettings, true, err)
	}

	report, err := w.deps.Content.Apply(&w.rt, client, reconcile.ReconcileOptions{
		Marker:        w.deps.Marker,
		LocalDNSPurge: w.deps.LocalDNSPurge,
		CNAMEPurge:    w.deps.CNAMEPurge,
	})
	if err != nil {
		return err
	}

	recordApplied(res, report)
	w.deps.Recorder.RecordDrift(w.rt.Name, buildDriftMap(&report.Diff))

	if report.Diff.NeedsGravity && w.deps.GravityOnChange {
		if err := client.CheckReadiness(); err != nil {
			w.log.Warn("skipping gravity, target not ready", "target", w.rt.Name, "error", err)
		} else {
			w.deps.Gravity.TriggerAsync(w.rt.Name, "adlist_change")
		}
	}
	return nil
}

// recordApplied fills res with applied and failed counts per resource: the
// planned diff minus the items reported as failed by reconcile.Apply.
func recordApplied(res *run.Result, report *reconcile.ApplyReport) {
	failed := map[[2]string]int{}
	for _, e := range report.Errors {
		var ie *reconcile.ItemError
		if errors.As(e, &ie) {
			failed[[2]string{ie.Resource, ie.Action}] += ie.Count
			res.Record(ie.Resource, ie.Action, ie.Count, e)
			continue
		}
		res.AddError(e)
	}

	planned := func(resource string, d reconcile.ResourceDiff) {
		for action, n := range map[string]int{"add": len(d.Adds), "update": len(d.Updates), "delete": len(d.Deletes)} {
			applied := n - failed[[2]string{resource, action}]
			if applied > 0 {
				res.Record(resource, action, applied, nil)
			}
		}
	}
	planned("groups", report.Diff.Groups)
	planned("adlists", report.Diff.Adlists)
	planned("deny", report.Diff.Deny)
	planned("allow", report.Diff.Allow)
	planned("local_dns", report.Diff.LocalDNS)
	planned("cname", report.Diff.CNAME)
	planned("clients", report.Diff.Clients)
}

func (w *TargetWorker) reconcileSettings(client *pihole.Client) (*pihole.Client, error) {
	if w.rt.Settings.IsEmpty() {
		return client, nil
	}

	settingsDiff, err := w.deps.Settings.Diff(&w.rt.Settings, client)
	if err != nil {
		w.log.Error("settings diff error", "target", w.rt.Name, "error", err)
	} else {
		w.reportSettingsDrift(settingsDiff)
	}

	if _, err := w.deps.Settings.Apply(w.rt.Name, &w.rt.Settings, client); err != nil {
		w.log.Error("settings reconcile error", "target", w.rt.Name, "error", err)
		return client, nil
	}

	if settingsDiff == nil || !settingsDiff.HasChanges() {
		return client, nil
	}

	w.log.Info("waiting for FTL ready after settings change", "target", w.rt.Name)
	if err := client.WaitForReady(30*time.Second, 500*time.Millisecond); err != nil {
		w.deps.Sessions.Invalidate(w.rt.Name)
		return nil, err
	}

	w.deps.Sessions.Invalidate(w.rt.Name)
	newClient, err := w.deps.Sessions.Get(w.rt.Target)
	if err != nil {
		w.deps.Sessions.Invalidate(w.rt.Name)
		return nil, err
	}

	if err := newClient.CheckReadiness(); err != nil {
		w.deps.Sessions.Invalidate(w.rt.Name)
		return nil, err
	}

	postDiff, err := w.deps.Settings.Diff(&w.rt.Settings, newClient)
	if err != nil {
		w.log.Error("settings post-apply diff error", "target", w.rt.Name, "error", err)
	} else {
		w.reportSettingsDrift(postDiff)
	}

	return newClient, nil
}

func (w *TargetWorker) reportSettingsDrift(diff *reconcile.SettingsDiff) {
	drifted := make(map[string]bool)
	for _, m := range w.deps.Settings.BuildDesiredList(&w.rt.Settings) {
		drifted[m.ForsetiPath] = false
	}
	for _, c := range diff.Changes {
		drifted[c.Name] = true
	}
	w.deps.Recorder.UpdateSettingsDrift(w.rt.Name, drifted)
}

func buildDriftMap(diff *reconcile.DiffReport) map[string]int {
	m := make(map[string]int, 7)
	count := func(name string, d reconcile.ResourceDiff) {
		m[name] = len(d.Adds) + len(d.Updates) + len(d.Deletes)
	}
	count("groups", diff.Groups)
	count("adlists", diff.Adlists)
	count("deny", diff.Deny)
	count("allow", diff.Allow)
	count("local_dns", diff.LocalDNS)
	count("cname", diff.CNAME)
	count("clients", diff.Clients)
	return m
}
