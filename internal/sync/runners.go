package sync

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/st0o0/forseti/internal/pihole"
	"github.com/st0o0/forseti/internal/redact"
	"github.com/st0o0/forseti/internal/run"
	"github.com/st0o0/forseti/internal/worker"
)

// SnapshotRunner loads the primary's state once per cycle.
type SnapshotRunner struct {
	resources map[string]bool
	state     *state
}

func (r *SnapshotRunner) Run(client *pihole.Client, _ *run.Result) error {
	var st state
	if err := st.load(client, r.resources); err != nil {
		return worker.StageError(run.StageLoad, false, fmt.Errorf("read primary: %w", err))
	}
	r.state = &st
	return nil
}

// ReplicaRunner syncs one replica from the primary snapshot.
type ReplicaRunner struct {
	primary   *state
	resources map[string]bool
	marker    string
	purgeDNS  bool
	log       *slog.Logger
}

func (r *ReplicaRunner) Run(replica *pihole.Client, res *run.Result) error {
	var replicaState state
	if err := replicaState.load(replica, r.resources); err != nil {
		return worker.StageError(run.StageLoad, false, fmt.Errorf("read replica: %w", err))
	}
	primary := r.primary

	if r.resources["groups"] {
		r.syncGroups(replica, res, primary.groups, replicaState.groups)
	}

	primaryIDToName := buildPrimaryIDToName(primary.groups)
	replicaNameToID := buildReplicaNameToID(replicaState.groups)

	if r.resources["groups"] {
		replicaGroups, err := replica.ListGroups()
		if err != nil {
			res.AddError(fmt.Errorf("re-fetch replica groups: %w", err))
		} else {
			replicaNameToID = buildReplicaNameToID(replicaGroups)
		}
	}

	if r.resources["adlists"] {
		r.syncAdlists(replica, res, primary.adlists, replicaState.adlists, primaryIDToName, replicaNameToID)
	}
	if r.resources["allowlists"] {
		r.syncAllowlists(replica, res, primary.allowlists, replicaState.allowlists, primaryIDToName, replicaNameToID)
	}
	if r.resources["deny"] {
		r.syncDomains(replica, res, "deny", "exact", primary.deny, replicaState.deny, primaryIDToName, replicaNameToID)
		r.syncDomains(replica, res, "deny", "regex", primary.denyRegex, replicaState.denyRegex, primaryIDToName, replicaNameToID)
	}
	if r.resources["allow"] {
		r.syncDomains(replica, res, "allow", "exact", primary.allow, replicaState.allow, primaryIDToName, replicaNameToID)
		r.syncDomains(replica, res, "allow", "regex", primary.allowRegex, replicaState.allowRegex, primaryIDToName, replicaNameToID)
	}
	if r.resources["local_dns"] {
		r.syncDNS(replica, res, primary.dns, replicaState.dns)
	}
	if r.resources["cnames"] {
		r.syncCNAMEs(replica, res, primary.cnames, replicaState.cnames)
	}
	if r.resources["clients"] {
		r.syncClients(replica, res, primary.clients, replicaState.clients, primaryIDToName, replicaNameToID)
	}
	return nil
}

// record accounts one write and logs failures at debug level; the run
// summary carries them at the appropriate level.
func (r *ReplicaRunner) record(res *run.Result, resource, action, key string, err error) {
	if err != nil {
		err = fmt.Errorf("%s %s %q: %w", action, resource, key, err)
		r.log.Debug("sync write failed", "target", res.Target, "resource", resource, "action", action, "key", key, "error", err)
	}
	res.Record(resource, action, 1, err)
}

func containsInvalidChars(s string) bool {
	return strings.ContainsAny(s, "\r\n\x00")
}

func (r *ReplicaRunner) syncGroups(replica *pihole.Client, res *run.Result, primary, replicaGroups []pihole.APIGroup) {
	primaryByName := make(map[string]pihole.APIGroup)
	for _, g := range primary {
		if containsInvalidChars(g.Name) {
			r.log.Warn("skipping group with invalid characters in name", "group", g.Name)
			continue
		}
		primaryByName[strings.ToLower(g.Name)] = g
	}

	replicaByName := make(map[string]pihole.APIGroup)
	for _, g := range replicaGroups {
		replicaByName[strings.ToLower(g.Name)] = g
	}

	for name, g := range primaryByName {
		if _, exists := replicaByName[name]; !exists {
			_, err := replica.CreateGroup(g.Name, r.marker, g.Enabled)
			r.record(res, "groups", "add", g.Name, err)
		}
	}

	for name, g := range replicaByName {
		if _, exists := primaryByName[name]; !exists && strings.Contains(g.Comment, r.marker) {
			r.record(res, "groups", "delete", g.Name, replica.DeleteGroups([]string{g.Name}))
		}
	}
}

func (r *ReplicaRunner) syncAdlists(replica *pihole.Client, res *run.Result, primary, replicaAdlists []pihole.APIList, primaryIDToName map[int]string, replicaNameToID map[string]int) {
	primaryByURL := make(map[string]pihole.APIList)
	for _, a := range primary {
		primaryByURL[a.Address] = a
	}

	replicaByURL := make(map[string]pihole.APIList)
	for _, a := range replicaAdlists {
		replicaByURL[a.Address] = a
	}

	for url, a := range primaryByURL {
		if _, exists := replicaByURL[url]; !exists {
			groups := translateGroupIDs(a.Groups, primaryIDToName, replicaNameToID)
			_, err := replica.CreateAdlist(a.Address, r.marker, a.Enabled, groups)
			r.record(res, "adlists", "add", redact.URL(url), err)
		}
	}

	for url, a := range replicaByURL {
		if _, exists := primaryByURL[url]; !exists && strings.Contains(a.Comment, r.marker) {
			r.record(res, "adlists", "delete", redact.URL(url), replica.DeleteAdlists([]string{url}))
		}
	}
}

func (r *ReplicaRunner) syncDomains(replica *pihole.Client, res *run.Result, domType, kind string, primary, replicaDomains []pihole.APIDomain, primaryIDToName map[int]string, replicaNameToID map[string]int) {
	primaryByDomain := make(map[string]pihole.APIDomain)
	for _, d := range primary {
		primaryByDomain[d.Domain] = d
	}

	replicaByDomain := make(map[string]pihole.APIDomain)
	for _, d := range replicaDomains {
		replicaByDomain[d.Domain] = d
	}

	for domain, d := range primaryByDomain {
		if _, exists := replicaByDomain[domain]; !exists {
			groups := translateGroupIDs(d.Groups, primaryIDToName, replicaNameToID)
			_, err := replica.CreateDomain(domType, kind, d.Domain, r.marker, d.Enabled, groups)
			r.record(res, domType, "add", kind+":"+domain, err)
		}
	}

	for domain, d := range replicaByDomain {
		if _, exists := primaryByDomain[domain]; !exists && strings.Contains(d.Comment, r.marker) {
			r.record(res, domType, "delete", kind+":"+domain, replica.DeleteDomains([]string{domain}))
		}
	}
}

func (r *ReplicaRunner) syncDNS(replica *pihole.Client, res *run.Result, primary, replicaDNS []pihole.APIDNSRecord) {
	primaryKeys := make(map[string]pihole.APIDNSRecord)
	for _, rec := range primary {
		primaryKeys[rec.IP+" "+rec.Domain] = rec
	}

	replicaKeys := make(map[string]pihole.APIDNSRecord)
	for _, rec := range replicaDNS {
		replicaKeys[rec.IP+" "+rec.Domain] = rec
	}

	for key, rec := range primaryKeys {
		if _, exists := replicaKeys[key]; !exists {
			r.record(res, "local_dns", "add", key, replica.AddDNSRecord(rec.IP, rec.Domain))
		}
	}

	if !r.purgeDNS {
		return
	}

	for key, rec := range replicaKeys {
		if _, exists := primaryKeys[key]; !exists {
			r.record(res, "local_dns", "delete", key, replica.DeleteDNSRecord(rec.IP, rec.Domain))
		}
	}
}

func (r *ReplicaRunner) syncClients(replica *pihole.Client, res *run.Result, primary, replicaClients []pihole.APIClient, primaryIDToName map[int]string, replicaNameToID map[string]int) {
	primaryByIP := make(map[string]pihole.APIClient)
	for _, c := range primary {
		primaryByIP[c.Client] = c
	}

	replicaByIP := make(map[string]pihole.APIClient)
	for _, c := range replicaClients {
		replicaByIP[c.Client] = c
	}

	for ip, c := range primaryByIP {
		if _, exists := replicaByIP[ip]; !exists {
			groups := translateGroupIDs(c.Groups, primaryIDToName, replicaNameToID)
			_, err := replica.CreateClient(c.Client, r.marker, groups)
			r.record(res, "clients", "add", ip, err)
		}
	}

	for ip, c := range replicaByIP {
		if _, exists := primaryByIP[ip]; !exists && strings.Contains(c.Comment, r.marker) {
			r.record(res, "clients", "delete", ip, replica.DeleteClients([]string{ip}))
		}
	}
}

func (r *ReplicaRunner) syncAllowlists(replica *pihole.Client, res *run.Result, primary, replicaAllowlists []pihole.APIList, primaryIDToName map[int]string, replicaNameToID map[string]int) {
	primaryByURL := make(map[string]pihole.APIList)
	for _, a := range primary {
		primaryByURL[a.Address] = a
	}

	replicaByURL := make(map[string]pihole.APIList)
	for _, a := range replicaAllowlists {
		replicaByURL[a.Address] = a
	}

	for url, a := range primaryByURL {
		if _, exists := replicaByURL[url]; !exists {
			groups := translateGroupIDs(a.Groups, primaryIDToName, replicaNameToID)
			_, err := replica.CreateAllowlist(a.Address, r.marker, a.Enabled, groups)
			r.record(res, "allowlists", "add", redact.URL(url), err)
		}
	}

	for url, a := range replicaByURL {
		if _, exists := primaryByURL[url]; !exists && strings.Contains(a.Comment, r.marker) {
			r.record(res, "allowlists", "delete", redact.URL(url), replica.DeleteAllowlists([]string{url}))
		}
	}
}

func (r *ReplicaRunner) syncCNAMEs(replica *pihole.Client, res *run.Result, primary, replicaCNAMEs []pihole.APICNAMERecord) {
	primaryKeys := make(map[string]pihole.APICNAMERecord)
	for _, rec := range primary {
		primaryKeys[rec.Domain+","+rec.Target] = rec
	}

	replicaKeys := make(map[string]pihole.APICNAMERecord)
	for _, rec := range replicaCNAMEs {
		replicaKeys[rec.Domain+","+rec.Target] = rec
	}

	changed := false
	for key, rec := range primaryKeys {
		if _, exists := replicaKeys[key]; !exists {
			r.record(res, "cnames", "add", key, replica.AddCNAMERecord(rec.Domain, rec.Target))
			changed = true
		}
	}

	if r.purgeDNS {
		for key, rec := range replicaKeys {
			if _, exists := primaryKeys[key]; !exists {
				r.record(res, "cnames", "delete", key, replica.DeleteCNAMERecord(rec.Domain, rec.Target))
				changed = true
			}
		}
	}

	if changed {
		r.log.Warn("CNAME changes will trigger FTL restart (brief DNS outage)", "target", res.Target)
	}
}

type state struct {
	groups     []pihole.APIGroup
	adlists    []pihole.APIList
	allowlists []pihole.APIList
	deny       []pihole.APIDomain
	denyRegex  []pihole.APIDomain
	allow      []pihole.APIDomain
	allowRegex []pihole.APIDomain
	dns        []pihole.APIDNSRecord
	cnames     []pihole.APICNAMERecord
	clients    []pihole.APIClient
}

func (st *state) load(client *pihole.Client, resources map[string]bool) error {
	var err error
	if resources["groups"] {
		st.groups, err = client.ListGroups()
		if err != nil {
			return fmt.Errorf("list groups: %w", err)
		}
	}
	if resources["adlists"] {
		st.adlists, err = client.ListAdlists()
		if err != nil {
			return fmt.Errorf("list adlists: %w", err)
		}
	}
	if resources["allowlists"] {
		st.allowlists, err = client.ListAllowlists()
		if err != nil {
			return fmt.Errorf("list allowlists: %w", err)
		}
	}
	if resources["deny"] {
		st.deny, err = client.ListDomains("deny", "exact")
		if err != nil {
			return fmt.Errorf("list deny/exact: %w", err)
		}
		st.denyRegex, err = client.ListDomains("deny", "regex")
		if err != nil {
			return fmt.Errorf("list deny/regex: %w", err)
		}
	}
	if resources["allow"] {
		st.allow, err = client.ListDomains("allow", "exact")
		if err != nil {
			return fmt.Errorf("list allow/exact: %w", err)
		}
		st.allowRegex, err = client.ListDomains("allow", "regex")
		if err != nil {
			return fmt.Errorf("list allow/regex: %w", err)
		}
	}
	if resources["local_dns"] {
		st.dns, err = client.ListDNSRecords()
		if err != nil {
			return fmt.Errorf("list dns: %w", err)
		}
	}
	if resources["cnames"] {
		st.cnames, err = client.ListCNAMERecords()
		if err != nil {
			return fmt.Errorf("list cnames: %w", err)
		}
	}
	if resources["clients"] {
		st.clients, err = client.ListClients()
		if err != nil {
			return fmt.Errorf("list clients: %w", err)
		}
	}
	return nil
}

func buildPrimaryIDToName(groups []pihole.APIGroup) map[int]string {
	m := make(map[int]string)
	for _, g := range groups {
		m[g.ID] = g.Name
	}
	return m
}

func buildReplicaNameToID(groups []pihole.APIGroup) map[string]int {
	m := make(map[string]int)
	for _, g := range groups {
		m[strings.ToLower(g.Name)] = g.ID
	}
	return m
}

func translateGroupIDs(primaryIDs []int, primaryIDToName map[int]string, replicaNameToID map[string]int) []int {
	var result []int
	for _, id := range primaryIDs {
		name, ok := primaryIDToName[id]
		if !ok {
			continue
		}
		replicaID, ok := replicaNameToID[strings.ToLower(name)]
		if !ok {
			continue
		}
		result = append(result, replicaID)
	}
	return result
}
