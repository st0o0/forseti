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
	Status   string
	Added    int
	Deleted  int
	Changes  map[string]map[string]int
	Errors   []error
}

// add merges a per-resource result into the report under the given
// resource type name (matching config-mode type names).
func (r *SyncReport) add(resource string, res resourceResult) {
	r.Added += res.added
	r.Deleted += res.deleted
	r.Errors = append(r.Errors, res.errs...)
	if res.added == 0 && res.deleted == 0 {
		return
	}
	if r.Changes == nil {
		r.Changes = make(map[string]map[string]int)
	}
	if r.Changes[resource] == nil {
		r.Changes[resource] = make(map[string]int)
	}
	r.Changes[resource]["add"] += res.added
	r.Changes[resource]["delete"] += res.deleted
}

// resourceResult is the outcome of syncing one resource type to a replica.
type resourceResult struct {
	added   int
	deleted int
	errs    []error
}

func (s *Syncer) SyncAll() {
	resources := make(map[string]bool)
	for _, r := range s.cfg.Sync.Resources {
		resources[r] = true
	}

	primaryState, ok := s.loadPrimary(resources)
	if !ok {
		return
	}

	for _, target := range s.cfg.Targets {
		if target.Role != "replica" {
			continue
		}
		s.syncReplica(target, primaryState, resources)
	}
}

// loadPrimary reads the primary's state while holding its API gate.
func (s *Syncer) loadPrimary(resources map[string]bool) (*state, bool) {
	name := s.cfg.Sync.Primary
	s.pool.Acquire(name)
	defer s.pool.Release(name)

	primary, err := s.getPrimary()
	if err != nil {
		slog.Error("primary session error", "error", err)
		s.pool.Invalidate(name)
		s.metrics.MarkTargetUnreachable(name)
		return nil, false
	}

	var primaryState state
	if err := primaryState.load(primary, resources); err != nil {
		slog.Error("error reading primary", "error", err)
		s.pool.Invalidate(name)
		return nil, false
	}
	s.metrics.MarkTargetReachable(name)
	return &primaryState, true
}

func (s *Syncer) syncReplica(target config.Target, primary *state, resources map[string]bool) SyncReport {
	s.pool.Acquire(target.Name)
	defer s.pool.Release(target.Name)

	start := time.Now()
	log := slog.With("target", target.Name)
	report := SyncReport{Target: target.Name, Status: metrics.RunError}

	defer func() {
		report.Duration = time.Since(start)
		s.metrics.RecordReconcile(metrics.ReconcileResult{
			Target:   target.Name,
			Duration: report.Duration,
			Status:   report.Status,
			Changes:  report.Changes,
		})
	}()

	replica, err := s.pool.Get(target)
	if err != nil {
		log.Error("sync session error", "error", err)
		s.pool.Invalidate(target.Name)
		s.metrics.MarkTargetUnreachable(target.Name)
		report.Errors = append(report.Errors, err)
		return report
	}

	var replicaState state
	if err := replicaState.load(replica, resources); err != nil {
		log.Error("reading replica state failed", "error", err)
		s.pool.Invalidate(target.Name)
		report.Errors = append(report.Errors, err)
		return report
	}
	s.metrics.MarkTargetReachable(target.Name)

	if resources["groups"] {
		report.add("groups", s.syncGroups(log, replica, primary.groups, replicaState.groups))
	}

	primaryIDToName := buildPrimaryIDToName(primary.groups)
	replicaNameToID := buildReplicaNameToID(replicaState.groups)

	if resources["groups"] {
		replicaGroups, err := replica.ListGroups()
		if err != nil {
			log.Error("re-fetching replica groups failed", "error", err)
			report.Errors = append(report.Errors, fmt.Errorf("re-fetch replica groups: %w", err))
		} else {
			replicaNameToID = buildReplicaNameToID(replicaGroups)
		}
	}

	if resources["adlists"] {
		report.add("adlists", s.syncAdlists(log, replica, primary.adlists, replicaState.adlists, primaryIDToName, replicaNameToID))
	}

	if resources["deny"] {
		report.add("deny", s.syncDomains(log, replica, "deny", "exact", primary.deny, replicaState.deny, primaryIDToName, replicaNameToID))
		report.add("deny", s.syncDomains(log, replica, "deny", "regex", primary.denyRegex, replicaState.denyRegex, primaryIDToName, replicaNameToID))
	}

	if resources["allow"] {
		report.add("allow", s.syncDomains(log, replica, "allow", "exact", primary.allow, replicaState.allow, primaryIDToName, replicaNameToID))
		report.add("allow", s.syncDomains(log, replica, "allow", "regex", primary.allowRegex, replicaState.allowRegex, primaryIDToName, replicaNameToID))
	}

	if resources["local_dns"] {
		report.add("local_dns", s.syncDNS(log, replica, primary.dns, replicaState.dns))
	}

	if resources["clients"] {
		report.add("clients", s.syncClients(log, replica, primary.clients, replicaState.clients, primaryIDToName, replicaNameToID))
	}

	report.Status = metrics.RunSuccess
	if len(report.Errors) > 0 {
		report.Status = metrics.RunPartial
	}

	if report.Added > 0 || report.Deleted > 0 {
		log.Info("synced", "added", report.Added, "deleted", report.Deleted, "duration", time.Since(start).Round(time.Millisecond))
	}
	return report
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

func (s *Syncer) syncGroups(log *slog.Logger, replica *pihole.Client, primary, replicaGroups []pihole.APIGroup) (res resourceResult) {
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
				log.Error("create group failed", "group", g.Name, "error", err)
				res.errs = append(res.errs, fmt.Errorf("create group %s: %w", g.Name, err))
			} else {
				res.added++
			}
		}
	}

	for name, g := range replicaByName {
		if _, exists := primaryByName[name]; !exists {
			if strings.Contains(g.Comment, s.marker) {
				if err := replica.DeleteGroups([]string{g.Name}); err != nil {
					log.Error("delete group failed", "group", g.Name, "error", err)
					res.errs = append(res.errs, fmt.Errorf("delete group %s: %w", g.Name, err))
				} else {
					res.deleted++
				}
			}
		}
	}

	return
}

func (s *Syncer) syncAdlists(log *slog.Logger, replica *pihole.Client, primary, replicaAdlists []pihole.APIList, primaryIDToName map[int]string, replicaNameToID map[string]int) (res resourceResult) {
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
				log.Error("create adlist failed", "adlist", url, "error", err)
				res.errs = append(res.errs, fmt.Errorf("create adlist %s: %w", url, err))
			} else {
				res.added++
			}
		}
	}

	for url, a := range replicaByURL {
		if _, exists := primaryByURL[url]; !exists {
			if strings.Contains(a.Comment, s.marker) {
				if err := replica.DeleteAdlists([]string{url}); err != nil {
					log.Error("delete adlist failed", "adlist", url, "error", err)
					res.errs = append(res.errs, fmt.Errorf("delete adlist %s: %w", url, err))
				} else {
					res.deleted++
				}
			}
		}
	}

	return
}

func (s *Syncer) syncDomains(log *slog.Logger, replica *pihole.Client, domType, kind string, primary, replicaDomains []pihole.APIDomain, primaryIDToName map[int]string, replicaNameToID map[string]int) (res resourceResult) {
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
				log.Error("create domain failed", "type", domType, "kind", kind, "domain", domain, "error", err)
				res.errs = append(res.errs, fmt.Errorf("create %s/%s domain %s: %w", domType, kind, domain, err))
			} else {
				res.added++
			}
		}
	}

	for domain, d := range replicaByDomain {
		if _, exists := primaryByDomain[domain]; !exists {
			if strings.Contains(d.Comment, s.marker) {
				if err := replica.DeleteDomains([]string{domain}); err != nil {
					log.Error("delete domain failed", "type", domType, "domain", domain, "error", err)
					res.errs = append(res.errs, fmt.Errorf("delete %s domain %s: %w", domType, domain, err))
				} else {
					res.deleted++
				}
			}
		}
	}

	return
}

func (s *Syncer) syncDNS(log *slog.Logger, replica *pihole.Client, primary, replicaDNS []pihole.APIDNSRecord) (res resourceResult) {
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
				log.Error("add DNS failed", "record", key, "error", err)
				res.errs = append(res.errs, fmt.Errorf("add DNS %s: %w", key, err))
			} else {
				res.added++
			}
		}
	}

	if !s.cfg.Sync.PurgeLocalDNS() {
		return
	}

	for key, r := range replicaKeys {
		if _, exists := primaryKeys[key]; !exists {
			if err := replica.DeleteDNSRecord(r.IP, r.Domain); err != nil {
				log.Error("delete DNS failed", "record", key, "error", err)
				res.errs = append(res.errs, fmt.Errorf("delete DNS %s: %w", key, err))
			} else {
				res.deleted++
			}
		}
	}

	return
}

func (s *Syncer) syncClients(log *slog.Logger, replica *pihole.Client, primary, replicaClients []pihole.APIClient, primaryIDToName map[int]string, replicaNameToID map[string]int) (res resourceResult) {
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
				log.Error("create client failed", "client", ip, "error", err)
				res.errs = append(res.errs, fmt.Errorf("create client %s: %w", ip, err))
			} else {
				res.added++
			}
		}
	}

	for ip, c := range replicaByIP {
		if _, exists := primaryByIP[ip]; !exists {
			if strings.Contains(c.Comment, s.marker) {
				if err := replica.DeleteClients([]string{ip}); err != nil {
					log.Error("delete client failed", "client", ip, "error", err)
					res.errs = append(res.errs, fmt.Errorf("delete client %s: %w", ip, err))
				} else {
					res.deleted++
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
