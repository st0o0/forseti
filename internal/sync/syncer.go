package sync

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/st0o0/forseti/internal/config"
	"github.com/st0o0/forseti/internal/metrics"
	"github.com/st0o0/forseti/internal/pihole"
	"github.com/st0o0/forseti/internal/session"
)

type Syncer struct {
	pool    *session.Pool
	cfg     *config.Config
	metrics *metrics.Server
	marker  string
}

func NewSyncer(pool *session.Pool, cfg *config.Config, m *metrics.Server) *Syncer {
	return &Syncer{
		pool:    pool,
		cfg:     cfg,
		metrics: m,
		marker:  "[forseti-sync]",
	}
}

type SyncReport struct {
	Target   string
	Duration time.Duration
	Added    int
	Deleted  int
	Errors   []error
}

func (s *Syncer) SyncAll() {
	primary, err := s.getPrimary()
	if err != nil {
		slog.Error("primary session error", "component", "sync", "error", err)
		return
	}

	resources := make(map[string]bool)
	for _, r := range s.cfg.Sync.Resources {
		resources[r] = true
	}

	var primaryState state
	if err := primaryState.load(primary, resources); err != nil {
		slog.Error("error reading primary", "component", "sync", "error", err)
		return
	}

	for _, target := range s.cfg.Targets {
		if target.Role != "replica" {
			continue
		}
		s.syncReplica(target, &primaryState, resources)
	}
}

func (s *Syncer) syncReplica(target config.Target, primary *state, resources map[string]bool) {
	start := time.Now()

	replica, err := s.pool.Get(target)
	if err != nil {
		slog.Error("sync session error", "target", target.Name, "error", err)
		s.metrics.MarkTargetUnreachable(target.Name)
		return
	}

	var replicaState state
	if err := replicaState.load(replica, resources); err != nil {
		slog.Error("sync error reading replica", "target", target.Name, "error", err)
		return
	}

	report := SyncReport{Target: target.Name}

	if resources["groups"] {
		a, d := s.syncGroups(replica, primary.groups, replicaState.groups)
		report.Added += a
		report.Deleted += d
	}

	primaryIDToName := buildPrimaryIDToName(primary.groups)
	replicaNameToID := buildReplicaNameToID(replicaState.groups)

	if resources["groups"] {
		replicaGroups, err := replica.ListGroups()
		if err != nil {
			slog.Error("error re-fetching replica groups", "target", target.Name, "error", err)
		} else {
			replicaNameToID = buildReplicaNameToID(replicaGroups)
		}
	}

	if resources["adlists"] {
		a, d := s.syncAdlists(replica, primary.adlists, replicaState.adlists, primaryIDToName, replicaNameToID)
		report.Added += a
		report.Deleted += d
	}

	if resources["deny"] {
		a, d := s.syncDomains(replica, "deny", "exact", primary.deny, replicaState.deny, primaryIDToName, replicaNameToID)
		report.Added += a
		report.Deleted += d
		a, d = s.syncDomains(replica, "deny", "regex", primary.denyRegex, replicaState.denyRegex, primaryIDToName, replicaNameToID)
		report.Added += a
		report.Deleted += d
	}

	if resources["allow"] {
		a, d := s.syncDomains(replica, "allow", "exact", primary.allow, replicaState.allow, primaryIDToName, replicaNameToID)
		report.Added += a
		report.Deleted += d
		a, d = s.syncDomains(replica, "allow", "regex", primary.allowRegex, replicaState.allowRegex, primaryIDToName, replicaNameToID)
		report.Added += a
		report.Deleted += d
	}

	if resources["local_dns"] {
		a, d := s.syncDNS(replica, primary.dns, replicaState.dns)
		report.Added += a
		report.Deleted += d
	}

	if resources["clients"] {
		a, d := s.syncClients(replica, primary.clients, replicaState.clients, primaryIDToName, replicaNameToID)
		report.Added += a
		report.Deleted += d
	}

	report.Duration = time.Since(start)

	s.metrics.RecordReconcile(metrics.ReconcileResult{
		Target:   target.Name,
		Duration: report.Duration,
		Success:  len(report.Errors) == 0,
	})

	if report.Added > 0 || report.Deleted > 0 {
		slog.Info("synced", "target", target.Name, "added", report.Added, "deleted", report.Deleted, "duration", report.Duration.Round(time.Millisecond))
	}
}

func (s *Syncer) getPrimary() (*pihole.Client, error) {
	for _, t := range s.cfg.Targets {
		if t.Name == s.cfg.Sync.Primary {
			return s.pool.Get(t)
		}
	}
	return nil, fmt.Errorf("primary target %q not found", s.cfg.Sync.Primary)
}

type state struct {
	groups    []pihole.APIGroup
	adlists   []pihole.APIList
	deny      []pihole.APIDomain
	denyRegex []pihole.APIDomain
	allow     []pihole.APIDomain
	allowRegex []pihole.APIDomain
	dns       []pihole.APIDNSRecord
	clients   []pihole.APIClient
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
	if resources["clients"] {
		st.clients, err = client.ListClients()
		if err != nil {
			return fmt.Errorf("list clients: %w", err)
		}
	}
	return nil
}

func (s *Syncer) syncGroups(replica *pihole.Client, primary, replicaGroups []pihole.APIGroup) (added, deleted int) {
	primaryByName := make(map[string]pihole.APIGroup)
	for _, g := range primary {
		primaryByName[strings.ToLower(g.Name)] = g
	}

	replicaByName := make(map[string]pihole.APIGroup)
	for _, g := range replicaGroups {
		replicaByName[strings.ToLower(g.Name)] = g
	}

	for name, g := range primaryByName {
		if _, exists := replicaByName[name]; !exists {
			if _, err := replica.CreateGroup(g.Name, s.marker, g.Enabled); err != nil {
				slog.Error("create group failed", "component", "sync", "group", g.Name, "error", err)
			} else {
				added++
			}
		}
	}

	for name, g := range replicaByName {
		if _, exists := primaryByName[name]; !exists {
			if strings.Contains(g.Comment, s.marker) {
				if err := replica.DeleteGroups([]string{g.Name}); err != nil {
					slog.Error("delete group failed", "component", "sync", "group", g.Name, "error", err)
				} else {
					deleted++
				}
			}
		}
	}

	return
}

func (s *Syncer) syncAdlists(replica *pihole.Client, primary, replicaAdlists []pihole.APIList, primaryIDToName map[int]string, replicaNameToID map[string]int) (added, deleted int) {
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
			if _, err := replica.CreateAdlist(a.Address, s.marker, a.Enabled, groups); err != nil {
				slog.Error("create adlist failed", "component", "sync", "adlist", url, "error", err)
			} else {
				added++
			}
		}
	}

	for url, a := range replicaByURL {
		if _, exists := primaryByURL[url]; !exists {
			if strings.Contains(a.Comment, s.marker) {
				if err := replica.DeleteAdlists([]string{url}); err != nil {
					slog.Error("delete adlist failed", "component", "sync", "adlist", url, "error", err)
				} else {
					deleted++
				}
			}
		}
	}

	return
}

func (s *Syncer) syncDomains(replica *pihole.Client, domType, kind string, primary, replicaDomains []pihole.APIDomain, primaryIDToName map[int]string, replicaNameToID map[string]int) (added, deleted int) {
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
			if _, err := replica.CreateDomain(domType, kind, d.Domain, s.marker, d.Enabled, groups); err != nil {
				slog.Error("create domain failed", "component", "sync", "type", domType, "kind", kind, "domain", domain, "error", err)
			} else {
				added++
			}
		}
	}

	for domain, d := range replicaByDomain {
		if _, exists := primaryByDomain[domain]; !exists {
			if strings.Contains(d.Comment, s.marker) {
				if err := replica.DeleteDomains([]string{domain}); err != nil {
					slog.Error("delete domain failed", "component", "sync", "type", domType, "domain", domain, "error", err)
				} else {
					deleted++
				}
			}
		}
	}

	return
}

func (s *Syncer) syncDNS(replica *pihole.Client, primary, replicaDNS []pihole.APIDNSRecord) (added, deleted int) {
	primaryKeys := make(map[string]pihole.APIDNSRecord)
	for _, r := range primary {
		primaryKeys[r.IP+" "+r.Domain] = r
	}

	replicaKeys := make(map[string]pihole.APIDNSRecord)
	for _, r := range replicaDNS {
		replicaKeys[r.IP+" "+r.Domain] = r
	}

	for key, r := range primaryKeys {
		if _, exists := replicaKeys[key]; !exists {
			if err := replica.AddDNSRecord(r.IP, r.Domain); err != nil {
				slog.Error("add DNS failed", "component", "sync", "record", key, "error", err)
			} else {
				added++
			}
		}
	}

	for key, r := range replicaKeys {
		if _, exists := primaryKeys[key]; !exists {
			if err := replica.DeleteDNSRecord(r.IP, r.Domain); err != nil {
				slog.Error("delete DNS failed", "component", "sync", "record", key, "error", err)
			} else {
				deleted++
			}
		}
	}

	return
}

func (s *Syncer) syncClients(replica *pihole.Client, primary, replicaClients []pihole.APIClient, primaryIDToName map[int]string, replicaNameToID map[string]int) (added, deleted int) {
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
			if _, err := replica.CreateClient(c.Client, s.marker, groups); err != nil {
				slog.Error("create client failed", "component", "sync", "client", ip, "error", err)
			} else {
				added++
			}
		}
	}

	for ip, c := range replicaByIP {
		if _, exists := primaryByIP[ip]; !exists {
			if strings.Contains(c.Comment, s.marker) {
				if err := replica.DeleteClients([]string{ip}); err != nil {
					slog.Error("delete client failed", "component", "sync", "client", ip, "error", err)
				} else {
					deleted++
				}
			}
		}
	}

	return
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
