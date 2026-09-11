package reconcile

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/st0o0/forseti/internal/config"
	"github.com/st0o0/forseti/internal/pihole"
)

type PiholeAPI interface {
	ListGroups() ([]pihole.APIGroup, error)
	CreateGroup(name, comment string, enabled bool) (*pihole.APIGroup, error)
	DeleteGroups(names []string) error
	ListAdlists() ([]pihole.APIList, error)
	CreateAdlist(address, comment string, enabled bool, groups []int) (*pihole.APIList, error)
	DeleteAdlists(addresses []string) error
	ListDomains(domType, kind string) ([]pihole.APIDomain, error)
	CreateDomain(domType, kind, domain, comment string, enabled bool, groups []int) (*pihole.APIDomain, error)
	DeleteDomains(domains []string) error
	ListDNSRecords() ([]pihole.APIDNSRecord, error)
	AddDNSRecord(ip, domain string) error
	DeleteDNSRecord(ip, domain string) error
	ListClients() ([]pihole.APIClient, error)
	CreateClient(ip, comment string, groups []int) (*pihole.APIClient, error)
	DeleteClients(ips []string) error
	UpdateAdlist(id int, groups []int) error
	UpdateClient(id int, groups []int) error
	UpdateDomain(id int, groups []int) error
	ListCNAMERecords() ([]pihole.APICNAMERecord, error)
	AddCNAMERecord(domain, target string) error
	DeleteCNAMERecord(domain, target string) error
}

type DiffAction string

const (
	ActionAdd    DiffAction = "add"
	ActionDelete DiffAction = "delete"
	ActionUpdate DiffAction = "update"
)

type DiffEntry struct {
	Action DiffAction
	Key    string
	ID     int
}

type ResourceDiff struct {
	Adds      []DiffEntry
	Deletes   []DiffEntry
	Updates   []DiffEntry
	Unchanged int
}

func (d ResourceDiff) HasChanges() bool {
	return len(d.Adds) > 0 || len(d.Deletes) > 0 || len(d.Updates) > 0
}

type DiffReport struct {
	Target       string
	Groups       ResourceDiff
	Adlists      ResourceDiff
	Deny         ResourceDiff
	Allow        ResourceDiff
	LocalDNS     ResourceDiff
	CNAME        ResourceDiff
	Clients      ResourceDiff
	NeedsGravity bool
}

type ApplyReport struct {
	Target string
	Diff   DiffReport
	Errors []error
}

func Plan(cfg *config.Config, target config.Target, api PiholeAPI, marker string) (*DiffReport, error) {

	report := &DiffReport{Target: target.Name}

	groups, err := api.ListGroups()
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}
	report.Groups = diffGroups(cfg.Groups, groups, marker)

	groupNameToID := buildGroupNameToID(groups)

	adlists, err := api.ListAdlists()
	if err != nil {
		return nil, fmt.Errorf("list adlists: %w", err)
	}
	report.Adlists = diffAdlists(cfg.Adlists, adlists, marker, groupNameToID)
	report.NeedsGravity = report.Adlists.HasChanges()

	for _, kind := range collectDenyKinds(cfg.Deny) {
		denyDomains, err := api.ListDomains("deny", kind)
		if err != nil {
			return nil, fmt.Errorf("list deny/%s: %w", kind, err)
		}
		kindDiff := diffDomains(filterDenyByKind(cfg.Deny, kind), denyDomains, marker)
		mergeDiff(&report.Deny, kindDiff)
	}

	for _, kind := range collectAllowKinds(cfg.Allow) {
		allowDomains, err := api.ListDomains("allow", kind)
		if err != nil {
			return nil, fmt.Errorf("list allow/%s: %w", kind, err)
		}
		kindDiff := diffAllowDomains(filterAllowByKind(cfg.Allow, kind), allowDomains, marker)
		mergeDiff(&report.Allow, kindDiff)
	}

	dnsRecords, err := api.ListDNSRecords()
	if err != nil {
		return nil, fmt.Errorf("list DNS records: %w", err)
	}
	report.LocalDNS = diffDNS(cfg.LocalDNS, dnsRecords, cfg.Reconcile.LocalDNSPurge)

	cnameRecords, err := api.ListCNAMERecords()
	if err != nil {
		return nil, fmt.Errorf("list CNAME records: %w", err)
	}
	report.CNAME = diffCNAME(cfg.CNAME, cnameRecords, cfg.Reconcile.CNAMEPurge)

	clients, err := api.ListClients()
	if err != nil {
		return nil, fmt.Errorf("list clients: %w", err)
	}
	report.Clients = diffClients(cfg.Clients, clients, marker, groupNameToID)

	return report, nil
}

func Apply(cfg *config.Config, target config.Target, api PiholeAPI, marker string) (*ApplyReport, error) {

	report := &ApplyReport{Target: target.Name}

	// Fetch actual state and compute diffs
	groups, err := api.ListGroups()
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}
	report.Diff.Groups = diffGroups(cfg.Groups, groups, marker)

	// Apply groups first (forward order for adds)
	groupNameToID := buildGroupNameToID(groups)
	for _, entry := range report.Diff.Groups.Adds {
		g, err := api.CreateGroup(entry.Key, marker, true)
		if err != nil {
			report.Errors = append(report.Errors, fmt.Errorf("create group %q: %w", entry.Key, err))
			continue
		}
		groupNameToID[entry.Key] = g.ID
	}
	if keys := collectKeys(report.Diff.Groups.Deletes); len(keys) > 0 {
		if err := api.DeleteGroups(keys); err != nil {
			report.Errors = append(report.Errors, fmt.Errorf("delete groups: %w", err))
		}
	}

	// Adlists
	adlists, err := api.ListAdlists()
	if err != nil {
		return nil, fmt.Errorf("list adlists: %w", err)
	}
	report.Diff.Adlists = diffAdlists(cfg.Adlists, adlists, marker, groupNameToID)
	for _, entry := range report.Diff.Adlists.Adds {
		adlist := findAdlistByURL(cfg.Adlists, entry.Key)
		groupIDs := resolveGroupIDs(adlist.Groups, groupNameToID)
		if _, err := api.CreateAdlist(entry.Key, marker, true, groupIDs); err != nil {
			report.Errors = append(report.Errors, fmt.Errorf("create adlist %q: %w", entry.Key, err))
		}
	}
	for _, entry := range report.Diff.Adlists.Updates {
		adlist := findAdlistByURL(cfg.Adlists, entry.Key)
		groupIDs := resolveGroupIDs(adlist.Groups, groupNameToID)
		if err := api.UpdateAdlist(entry.ID, groupIDs); err != nil {
			report.Errors = append(report.Errors, fmt.Errorf("update adlist %q: %w", entry.Key, err))
		}
	}
	if keys := collectKeys(report.Diff.Adlists.Deletes); len(keys) > 0 {
		if err := api.DeleteAdlists(keys); err != nil {
			report.Errors = append(report.Errors, fmt.Errorf("delete adlists: %w", err))
		}
	}
	report.Diff.NeedsGravity = report.Diff.Adlists.HasChanges()

	// Deny domains (per-kind)
	for _, kind := range collectDenyKinds(cfg.Deny) {
		denyDomains, err := api.ListDomains("deny", kind)
		if err != nil {
			return nil, fmt.Errorf("list deny/%s: %w", kind, err)
		}
		kindDeny := filterDenyByKind(cfg.Deny, kind)
		kindDiff := diffDomains(kindDeny, denyDomains, marker)
		for _, entry := range kindDiff.Adds {
			if _, err := api.CreateDomain("deny", kind, entry.Key, marker, true, nil); err != nil {
				report.Errors = append(report.Errors, fmt.Errorf("create deny/%s %q: %w", kind, entry.Key, err))
			}
		}
		if keys := collectKeys(kindDiff.Deletes); len(keys) > 0 {
			if err := api.DeleteDomains(keys); err != nil {
				report.Errors = append(report.Errors, fmt.Errorf("delete deny/%s domains: %w", kind, err))
			}
		}
		mergeDiff(&report.Diff.Deny, kindDiff)
	}

	// Allow domains (per-kind)
	for _, kind := range collectAllowKinds(cfg.Allow) {
		allowDomains, err := api.ListDomains("allow", kind)
		if err != nil {
			return nil, fmt.Errorf("list allow/%s: %w", kind, err)
		}
		kindAllow := filterAllowByKind(cfg.Allow, kind)
		kindDiff := diffAllowDomains(kindAllow, allowDomains, marker)
		for _, entry := range kindDiff.Adds {
			if _, err := api.CreateDomain("allow", kind, entry.Key, marker, true, nil); err != nil {
				report.Errors = append(report.Errors, fmt.Errorf("create allow/%s %q: %w", kind, entry.Key, err))
			}
		}
		if keys := collectKeys(kindDiff.Deletes); len(keys) > 0 {
			if err := api.DeleteDomains(keys); err != nil {
				report.Errors = append(report.Errors, fmt.Errorf("delete allow/%s domains: %w", kind, err))
			}
		}
		mergeDiff(&report.Diff.Allow, kindDiff)
	}

	// Local DNS
	dnsRecords, err := api.ListDNSRecords()
	if err != nil {
		return nil, fmt.Errorf("list DNS: %w", err)
	}
	report.Diff.LocalDNS = diffDNS(cfg.LocalDNS, dnsRecords, cfg.Reconcile.LocalDNSPurge)
	for _, entry := range report.Diff.LocalDNS.Adds {
		parts := strings.SplitN(entry.Key, " ", 2)
		if err := api.AddDNSRecord(parts[0], parts[1]); err != nil {
			report.Errors = append(report.Errors, fmt.Errorf("add DNS %q: %w", entry.Key, err))
		}
	}
	for _, entry := range report.Diff.LocalDNS.Deletes {
		parts := strings.SplitN(entry.Key, " ", 2)
		if err := api.DeleteDNSRecord(parts[0], parts[1]); err != nil {
			report.Errors = append(report.Errors, fmt.Errorf("delete DNS %q: %w", entry.Key, err))
		}
	}

	// CNAME
	cnameRecords, err := api.ListCNAMERecords()
	if err != nil {
		return nil, fmt.Errorf("list CNAME: %w", err)
	}
	report.Diff.CNAME = diffCNAME(cfg.CNAME, cnameRecords, cfg.Reconcile.CNAMEPurge)
	for _, entry := range report.Diff.CNAME.Adds {
		parts := strings.SplitN(entry.Key, ",", 2)
		if err := api.AddCNAMERecord(parts[0], parts[1]); err != nil {
			report.Errors = append(report.Errors, fmt.Errorf("add CNAME %q: %w", entry.Key, err))
		}
	}
	for _, entry := range report.Diff.CNAME.Deletes {
		parts := strings.SplitN(entry.Key, ",", 2)
		if err := api.DeleteCNAMERecord(parts[0], parts[1]); err != nil {
			report.Errors = append(report.Errors, fmt.Errorf("delete CNAME %q: %w", entry.Key, err))
		}
	}
	if report.Diff.CNAME.HasChanges() {
		slog.Warn("CNAME changes will trigger FTL restart (brief DNS outage)")
	}

	// Clients
	clients, err := api.ListClients()
	if err != nil {
		return nil, fmt.Errorf("list clients: %w", err)
	}
	report.Diff.Clients = diffClients(cfg.Clients, clients, marker, groupNameToID)
	for _, entry := range report.Diff.Clients.Adds {
		client := findClientByMatch(cfg.Clients, entry.Key)
		groupIDs := resolveGroupIDs(client.Groups, groupNameToID)
		if _, err := api.CreateClient(entry.Key, marker, groupIDs); err != nil {
			report.Errors = append(report.Errors, fmt.Errorf("create client %q: %w", entry.Key, err))
		}
	}
	for _, entry := range report.Diff.Clients.Updates {
		client := findClientByMatch(cfg.Clients, entry.Key)
		groupIDs := resolveGroupIDs(client.Groups, groupNameToID)
		if err := api.UpdateClient(entry.ID, groupIDs); err != nil {
			report.Errors = append(report.Errors, fmt.Errorf("update client %q: %w", entry.Key, err))
		}
	}
	if keys := collectKeys(report.Diff.Clients.Deletes); len(keys) > 0 {
		if err := api.DeleteClients(keys); err != nil {
			report.Errors = append(report.Errors, fmt.Errorf("delete clients: %w", err))
		}
	}

	report.Diff.Target = target.Name
	return report, nil
}

// Diff functions

func diffGroups(desired []config.Group, actual []pihole.APIGroup, marker string) ResourceDiff {
	var diff ResourceDiff
	actualByName := make(map[string]pihole.APIGroup)
	for _, g := range actual {
		actualByName[strings.ToLower(g.Name)] = g
	}

	desiredNames := make(map[string]bool)
	for _, g := range desired {
		lower := strings.ToLower(g.Name)
		desiredNames[lower] = true
		if _, exists := actualByName[lower]; exists {
			diff.Unchanged++
		} else {
			diff.Adds = append(diff.Adds, DiffEntry{Action: ActionAdd, Key: g.Name})
		}
	}

	for _, g := range actual {
		lower := strings.ToLower(g.Name)
		if !desiredNames[lower] && strings.Contains(g.Comment, marker) {
			diff.Deletes = append(diff.Deletes, DiffEntry{Action: ActionDelete, Key: g.Name, ID: g.ID})
		}
	}
	return diff
}

func diffAdlists(desired []config.Adlist, actual []pihole.APIList, marker string, groupNameToID map[string]int) ResourceDiff {
	var diff ResourceDiff
	actualByURL := make(map[string]pihole.APIList)
	for _, a := range actual {
		actualByURL[a.Address] = a
	}

	desiredURLs := make(map[string]bool)
	for _, a := range desired {
		desiredURLs[a.URL] = true
		if existing, exists := actualByURL[a.URL]; exists {
			desiredGroups := resolveGroupIDs(a.Groups, groupNameToID)
			if !groupsEqual(desiredGroups, existing.Groups) {
				diff.Updates = append(diff.Updates, DiffEntry{Action: ActionUpdate, Key: a.URL, ID: existing.ID})
			} else {
				diff.Unchanged++
			}
		} else {
			diff.Adds = append(diff.Adds, DiffEntry{Action: ActionAdd, Key: a.URL})
		}
	}

	for _, a := range actual {
		if !desiredURLs[a.Address] && strings.Contains(a.Comment, marker) {
			diff.Deletes = append(diff.Deletes, DiffEntry{Action: ActionDelete, Key: a.Address, ID: a.ID})
		}
	}
	return diff
}

func diffDomains(desired []config.DenyEntry, actual []pihole.APIDomain, marker string) ResourceDiff {
	var diff ResourceDiff
	actualByDomain := make(map[string]pihole.APIDomain)
	for _, d := range actual {
		actualByDomain[d.Domain] = d
	}

	desiredDomains := make(map[string]bool)
	for _, d := range desired {
		desiredDomains[d.Domain] = true
		if _, exists := actualByDomain[d.Domain]; exists {
			diff.Unchanged++
		} else {
			diff.Adds = append(diff.Adds, DiffEntry{Action: ActionAdd, Key: d.Domain})
		}
	}

	for _, d := range actual {
		if !desiredDomains[d.Domain] && strings.Contains(d.Comment, marker) {
			diff.Deletes = append(diff.Deletes, DiffEntry{Action: ActionDelete, Key: d.Domain, ID: d.ID})
		}
	}
	return diff
}

func diffAllowDomains(desired []config.AllowEntry, actual []pihole.APIDomain, marker string) ResourceDiff {
	var diff ResourceDiff
	actualByDomain := make(map[string]pihole.APIDomain)
	for _, d := range actual {
		actualByDomain[d.Domain] = d
	}

	desiredDomains := make(map[string]bool)
	for _, d := range desired {
		desiredDomains[d.Domain] = true
		if _, exists := actualByDomain[d.Domain]; exists {
			diff.Unchanged++
		} else {
			diff.Adds = append(diff.Adds, DiffEntry{Action: ActionAdd, Key: d.Domain})
		}
	}

	for _, d := range actual {
		if !desiredDomains[d.Domain] && strings.Contains(d.Comment, marker) {
			diff.Deletes = append(diff.Deletes, DiffEntry{Action: ActionDelete, Key: d.Domain, ID: d.ID})
		}
	}
	return diff
}

func diffDNS(desired []config.LocalDNSEntry, actual []pihole.APIDNSRecord, purge bool) ResourceDiff {
	var diff ResourceDiff
	actualKeys := make(map[string]bool)
	for _, r := range actual {
		actualKeys[r.IP+" "+r.Domain] = true
	}

	desiredKeys := make(map[string]bool)
	for _, d := range desired {
		key := d.IP + " " + d.Domain
		desiredKeys[key] = true
		if actualKeys[key] {
			diff.Unchanged++
		} else {
			diff.Adds = append(diff.Adds, DiffEntry{Action: ActionAdd, Key: key})
		}
	}

	if purge {
		for _, r := range actual {
			key := r.IP + " " + r.Domain
			if !desiredKeys[key] {
				diff.Deletes = append(diff.Deletes, DiffEntry{Action: ActionDelete, Key: key})
			}
		}
	}
	return diff
}

func diffCNAME(desired []config.CNAMEEntry, actual []pihole.APICNAMERecord, purge bool) ResourceDiff {
	var diff ResourceDiff
	actualKeys := make(map[string]bool)
	for _, r := range actual {
		actualKeys[r.Domain+","+r.Target] = true
	}

	desiredKeys := make(map[string]bool)
	for _, d := range desired {
		key := d.Domain + "," + d.Target
		desiredKeys[key] = true
		if actualKeys[key] {
			diff.Unchanged++
		} else {
			diff.Adds = append(diff.Adds, DiffEntry{Action: ActionAdd, Key: key})
		}
	}

	if purge {
		for _, r := range actual {
			key := r.Domain + "," + r.Target
			if !desiredKeys[key] {
				diff.Deletes = append(diff.Deletes, DiffEntry{Action: ActionDelete, Key: key})
			}
		}
	}
	return diff
}

func diffClients(desired []config.ClientEntry, actual []pihole.APIClient, marker string, groupNameToID map[string]int) ResourceDiff {
	var diff ResourceDiff
	actualByIP := make(map[string]pihole.APIClient)
	for _, c := range actual {
		actualByIP[c.Client] = c
	}

	desiredMatches := make(map[string]bool)
	for _, c := range desired {
		desiredMatches[c.Match] = true
		if existing, exists := actualByIP[c.Match]; exists {
			desiredGroups := resolveGroupIDs(c.Groups, groupNameToID)
			if !groupsEqual(desiredGroups, existing.Groups) {
				diff.Updates = append(diff.Updates, DiffEntry{Action: ActionUpdate, Key: c.Match, ID: existing.ID})
			} else {
				diff.Unchanged++
			}
		} else {
			diff.Adds = append(diff.Adds, DiffEntry{Action: ActionAdd, Key: c.Match})
		}
	}

	for _, c := range actual {
		if !desiredMatches[c.Client] && strings.Contains(c.Comment, marker) {
			diff.Deletes = append(diff.Deletes, DiffEntry{Action: ActionDelete, Key: c.Client, ID: c.ID})
		}
	}
	return diff
}

// Helpers

func groupsEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	sa := make([]int, len(a))
	sb := make([]int, len(b))
	copy(sa, a)
	copy(sb, b)
	sort.Ints(sa)
	sort.Ints(sb)
	for i := range sa {
		if sa[i] != sb[i] {
			return false
		}
	}
	return true
}

func buildGroupNameToID(groups []pihole.APIGroup) map[string]int {
	m := make(map[string]int)
	for _, g := range groups {
		m[strings.ToLower(g.Name)] = g.ID
	}
	return m
}

func resolveGroupIDs(names []string, nameToID map[string]int) []int {
	ids := make([]int, 0, len(names))
	for _, name := range names {
		if id, ok := nameToID[strings.ToLower(name)]; ok {
			ids = append(ids, id)
		}
	}
	return ids
}

func collectKeys(entries []DiffEntry) []string {
	keys := make([]string, 0, len(entries))
	for _, e := range entries {
		keys = append(keys, e.Key)
	}
	return keys
}

func findAdlistByURL(adlists []config.Adlist, url string) config.Adlist {
	for _, a := range adlists {
		if a.URL == url {
			return a
		}
	}
	return config.Adlist{}
}

func findClientByMatch(clients []config.ClientEntry, match string) config.ClientEntry {
	for _, c := range clients {
		if c.Match == match {
			return c
		}
	}
	return config.ClientEntry{}
}

func collectDenyKinds(entries []config.DenyEntry) []string {
	seen := map[string]bool{}
	for _, e := range entries {
		k := e.Kind
		if k == "" {
			k = "exact"
		}
		seen[k] = true
	}
	if len(seen) == 0 {
		seen["exact"] = true
	}
	kinds := make([]string, 0, len(seen))
	for k := range seen {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	return kinds
}

func collectAllowKinds(entries []config.AllowEntry) []string {
	seen := map[string]bool{}
	for _, e := range entries {
		k := e.Kind
		if k == "" {
			k = "exact"
		}
		seen[k] = true
	}
	if len(seen) == 0 {
		seen["exact"] = true
	}
	kinds := make([]string, 0, len(seen))
	for k := range seen {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	return kinds
}

func filterDenyByKind(entries []config.DenyEntry, kind string) []config.DenyEntry {
	var out []config.DenyEntry
	for _, e := range entries {
		k := e.Kind
		if k == "" {
			k = "exact"
		}
		if k == kind {
			out = append(out, e)
		}
	}
	return out
}

func filterAllowByKind(entries []config.AllowEntry, kind string) []config.AllowEntry {
	var out []config.AllowEntry
	for _, e := range entries {
		k := e.Kind
		if k == "" {
			k = "exact"
		}
		if k == kind {
			out = append(out, e)
		}
	}
	return out
}

func mergeDiff(target *ResourceDiff, source ResourceDiff) {
	target.Adds = append(target.Adds, source.Adds...)
	target.Deletes = append(target.Deletes, source.Deletes...)
	target.Updates = append(target.Updates, source.Updates...)
	target.Unchanged += source.Unchanged
}
