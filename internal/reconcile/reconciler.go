package reconcile

import (
	"fmt"
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
}

type DiffAction string

const (
	ActionAdd    DiffAction = "add"
	ActionDelete DiffAction = "delete"
)

type DiffEntry struct {
	Action DiffAction
	Key    string
	ID     int
}

type ResourceDiff struct {
	Adds      []DiffEntry
	Deletes   []DiffEntry
	Unchanged int
}

func (d ResourceDiff) HasChanges() bool {
	return len(d.Adds) > 0 || len(d.Deletes) > 0
}

type DiffReport struct {
	Target       string
	Groups       ResourceDiff
	Adlists      ResourceDiff
	Deny         ResourceDiff
	Allow        ResourceDiff
	LocalDNS     ResourceDiff
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

	adlists, err := api.ListAdlists()
	if err != nil {
		return nil, fmt.Errorf("list adlists: %w", err)
	}
	report.Adlists = diffAdlists(cfg.Adlists, adlists, marker)
	report.NeedsGravity = report.Adlists.HasChanges()

	denyDomains, err := api.ListDomains("deny", "exact")
	if err != nil {
		return nil, fmt.Errorf("list deny domains: %w", err)
	}
	report.Deny = diffDomains(cfg.Deny, denyDomains, marker)

	allowDomains, err := api.ListDomains("allow", "exact")
	if err != nil {
		return nil, fmt.Errorf("list allow domains: %w", err)
	}
	report.Allow = diffAllowDomains(cfg.Allow, allowDomains, marker)

	dnsRecords, err := api.ListDNSRecords()
	if err != nil {
		return nil, fmt.Errorf("list DNS records: %w", err)
	}
	report.LocalDNS = diffDNS(cfg.LocalDNS, dnsRecords)

	clients, err := api.ListClients()
	if err != nil {
		return nil, fmt.Errorf("list clients: %w", err)
	}
	report.Clients = diffClients(cfg.Clients, clients, marker)

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
	report.Diff.Adlists = diffAdlists(cfg.Adlists, adlists, marker)
	for _, entry := range report.Diff.Adlists.Adds {
		adlist := findAdlistByURL(cfg.Adlists, entry.Key)
		groupIDs := resolveGroupIDs(adlist.Groups, groupNameToID)
		if _, err := api.CreateAdlist(entry.Key, marker, true, groupIDs); err != nil {
			report.Errors = append(report.Errors, fmt.Errorf("create adlist %q: %w", entry.Key, err))
		}
	}
	if keys := collectKeys(report.Diff.Adlists.Deletes); len(keys) > 0 {
		if err := api.DeleteAdlists(keys); err != nil {
			report.Errors = append(report.Errors, fmt.Errorf("delete adlists: %w", err))
		}
	}
	report.Diff.NeedsGravity = report.Diff.Adlists.HasChanges()

	// Deny domains
	denyDomains, err := api.ListDomains("deny", "exact")
	if err != nil {
		return nil, fmt.Errorf("list deny: %w", err)
	}
	report.Diff.Deny = diffDomains(cfg.Deny, denyDomains, marker)
	for _, entry := range report.Diff.Deny.Adds {
		if _, err := api.CreateDomain("deny", "exact", entry.Key, marker, true, nil); err != nil {
			report.Errors = append(report.Errors, fmt.Errorf("create deny %q: %w", entry.Key, err))
		}
	}
	if keys := collectKeys(report.Diff.Deny.Deletes); len(keys) > 0 {
		if err := api.DeleteDomains(keys); err != nil {
			report.Errors = append(report.Errors, fmt.Errorf("delete deny domains: %w", err))
		}
	}

	// Allow domains
	allowDomains, err := api.ListDomains("allow", "exact")
	if err != nil {
		return nil, fmt.Errorf("list allow: %w", err)
	}
	report.Diff.Allow = diffAllowDomains(cfg.Allow, allowDomains, marker)
	for _, entry := range report.Diff.Allow.Adds {
		if _, err := api.CreateDomain("allow", "exact", entry.Key, marker, true, nil); err != nil {
			report.Errors = append(report.Errors, fmt.Errorf("create allow %q: %w", entry.Key, err))
		}
	}
	if keys := collectKeys(report.Diff.Allow.Deletes); len(keys) > 0 {
		if err := api.DeleteDomains(keys); err != nil {
			report.Errors = append(report.Errors, fmt.Errorf("delete allow domains: %w", err))
		}
	}

	// Local DNS
	dnsRecords, err := api.ListDNSRecords()
	if err != nil {
		return nil, fmt.Errorf("list DNS: %w", err)
	}
	report.Diff.LocalDNS = diffDNS(cfg.LocalDNS, dnsRecords)
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

	// Clients
	clients, err := api.ListClients()
	if err != nil {
		return nil, fmt.Errorf("list clients: %w", err)
	}
	report.Diff.Clients = diffClients(cfg.Clients, clients, marker)
	for _, entry := range report.Diff.Clients.Adds {
		client := findClientByMatch(cfg.Clients, entry.Key)
		groupIDs := resolveGroupIDs(client.Groups, groupNameToID)
		if _, err := api.CreateClient(entry.Key, marker, groupIDs); err != nil {
			report.Errors = append(report.Errors, fmt.Errorf("create client %q: %w", entry.Key, err))
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

func diffAdlists(desired []config.Adlist, actual []pihole.APIList, marker string) ResourceDiff {
	var diff ResourceDiff
	actualByURL := make(map[string]pihole.APIList)
	for _, a := range actual {
		actualByURL[a.Address] = a
	}

	desiredURLs := make(map[string]bool)
	for _, a := range desired {
		desiredURLs[a.URL] = true
		if _, exists := actualByURL[a.URL]; exists {
			diff.Unchanged++
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

func diffDNS(desired []config.LocalDNSEntry, actual []pihole.APIDNSRecord) ResourceDiff {
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

	for _, r := range actual {
		key := r.IP + " " + r.Domain
		if !desiredKeys[key] {
			diff.Deletes = append(diff.Deletes, DiffEntry{Action: ActionDelete, Key: key})
		}
	}
	return diff
}

func diffClients(desired []config.ClientEntry, actual []pihole.APIClient, marker string) ResourceDiff {
	var diff ResourceDiff
	actualByIP := make(map[string]pihole.APIClient)
	for _, c := range actual {
		actualByIP[c.Client] = c
	}

	desiredMatches := make(map[string]bool)
	for _, c := range desired {
		desiredMatches[c.Match] = true
		if _, exists := actualByIP[c.Match]; exists {
			diff.Unchanged++
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
