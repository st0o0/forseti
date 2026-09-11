package reconcile

import (
	"errors"
	"testing"

	"github.com/st0o0/forseti/internal/config"
	"github.com/st0o0/forseti/internal/pihole"
)

type mockAPI struct {
	groups     []pihole.APIGroup
	adlists    []pihole.APIList
	domains    map[string][]pihole.APIDomain // "deny/exact" -> domains
	dnsRecords []pihole.APIDNSRecord
	clients    []pihole.APIClient

	createdGroups   []string
	createdAdlists  []string
	createdDomains  []string
	createdClients  []string
	deletedGroups   []string
	deletedAdlists2 []string
	deletedDomains2 []string
	deletedClients2 []string
	addedDNS        []string
	deletedDNS      []string

	listGroupsErr      error
	listAdlistsErr     error
	listDomainsErr     map[string]error // "deny/exact" -> error
	listDNSErr         error
	listClientsErr     error
	createGroupErr     error
	createAdlistErr    error
	createDomainErr    error
	createClientErr    error
	deleteGroupsErr    error
	deleteAdlistsErr   error
	deleteDomainsErr   error
	deleteClientsErr   error
	addDNSErr          error
	deleteDNSErr       error

	nextGroupID int
}

func newMockAPI() *mockAPI {
	return &mockAPI{
		domains:        make(map[string][]pihole.APIDomain),
		listDomainsErr: make(map[string]error),
		nextGroupID:    100,
	}
}

func (m *mockAPI) ListGroups() ([]pihole.APIGroup, error) {
	if m.listGroupsErr != nil {
		return nil, m.listGroupsErr
	}
	return m.groups, nil
}
func (m *mockAPI) CreateGroup(name, comment string, enabled bool) (*pihole.APIGroup, error) {
	m.createdGroups = append(m.createdGroups, name)
	if m.createGroupErr != nil {
		return nil, m.createGroupErr
	}
	g := &pihole.APIGroup{ID: m.nextGroupID, Name: name, Comment: comment}
	m.nextGroupID++
	return g, nil
}
func (m *mockAPI) DeleteGroups(names []string) error {
	m.deletedGroups = append(m.deletedGroups, names...)
	return m.deleteGroupsErr
}

func (m *mockAPI) ListAdlists() ([]pihole.APIList, error) {
	if m.listAdlistsErr != nil {
		return nil, m.listAdlistsErr
	}
	return m.adlists, nil
}
func (m *mockAPI) CreateAdlist(address, comment string, enabled bool, groups []int) (*pihole.APIList, error) {
	m.createdAdlists = append(m.createdAdlists, address)
	if m.createAdlistErr != nil {
		return nil, m.createAdlistErr
	}
	return &pihole.APIList{ID: 99, Address: address}, nil
}
func (m *mockAPI) DeleteAdlists(addresses []string) error {
	m.deletedAdlists2 = append(m.deletedAdlists2, addresses...)
	return m.deleteAdlistsErr
}

func (m *mockAPI) ListDomains(domType, kind string) ([]pihole.APIDomain, error) {
	key := domType + "/" + kind
	if err, ok := m.listDomainsErr[key]; ok && err != nil {
		return nil, err
	}
	return m.domains[key], nil
}
func (m *mockAPI) CreateDomain(domType, kind, domain, comment string, enabled bool, groups []int) (*pihole.APIDomain, error) {
	m.createdDomains = append(m.createdDomains, domType+":"+domain)
	if m.createDomainErr != nil {
		return nil, m.createDomainErr
	}
	return &pihole.APIDomain{ID: 99, Domain: domain}, nil
}
func (m *mockAPI) DeleteDomains(domains []string) error {
	m.deletedDomains2 = append(m.deletedDomains2, domains...)
	return m.deleteDomainsErr
}

func (m *mockAPI) ListDNSRecords() ([]pihole.APIDNSRecord, error) {
	if m.listDNSErr != nil {
		return nil, m.listDNSErr
	}
	return m.dnsRecords, nil
}
func (m *mockAPI) AddDNSRecord(ip, domain string) error {
	m.addedDNS = append(m.addedDNS, ip+" "+domain)
	return m.addDNSErr
}
func (m *mockAPI) DeleteDNSRecord(ip, domain string) error {
	m.deletedDNS = append(m.deletedDNS, ip+" "+domain)
	return m.deleteDNSErr
}

func (m *mockAPI) ListClients() ([]pihole.APIClient, error) {
	if m.listClientsErr != nil {
		return nil, m.listClientsErr
	}
	return m.clients, nil
}
func (m *mockAPI) CreateClient(ip, comment string, groups []int) (*pihole.APIClient, error) {
	m.createdClients = append(m.createdClients, ip)
	if m.createClientErr != nil {
		return nil, m.createClientErr
	}
	return &pihole.APIClient{ID: 99, Client: ip}, nil
}
func (m *mockAPI) DeleteClients(ips []string) error {
	m.deletedClients2 = append(m.deletedClients2, ips...)
	return m.deleteClientsErr
}

// --- Diff function tests ---

func TestDiffGroups(t *testing.T) {
	t.Run("add new group", func(t *testing.T) {
		diff := diffGroups(
			[]config.Group{{Name: "kids"}, {Name: "work"}},
			[]pihole.APIGroup{{ID: 1, Name: "kids", Comment: "[forseti]"}},
			"[forseti]",
		)
		if len(diff.Adds) != 1 || diff.Adds[0].Key != "work" {
			t.Errorf("expected add 'work', got %v", diff.Adds)
		}
		if diff.Unchanged != 1 {
			t.Errorf("unchanged = %d, want 1", diff.Unchanged)
		}
	})

	t.Run("delete managed group", func(t *testing.T) {
		diff := diffGroups(
			nil,
			[]pihole.APIGroup{{ID: 1, Name: "old", Comment: "[forseti]"}},
			"[forseti]",
		)
		if len(diff.Deletes) != 1 || diff.Deletes[0].Key != "old" {
			t.Errorf("expected delete 'old', got %v", diff.Deletes)
		}
	})

	t.Run("unmanaged group not deleted", func(t *testing.T) {
		diff := diffGroups(
			nil,
			[]pihole.APIGroup{{ID: 1, Name: "manual", Comment: "user created"}},
			"[forseti]",
		)
		if len(diff.Deletes) != 0 {
			t.Errorf("should not delete unmanaged group, got %v", diff.Deletes)
		}
	})

	t.Run("case insensitive matching", func(t *testing.T) {
		diff := diffGroups(
			[]config.Group{{Name: "Kids"}},
			[]pihole.APIGroup{{ID: 1, Name: "kids", Comment: ""}},
			"[forseti]",
		)
		if diff.Unchanged != 1 {
			t.Errorf("unchanged = %d, want 1 (case insensitive)", diff.Unchanged)
		}
		if len(diff.Adds) != 0 {
			t.Errorf("should not add when case differs, got %v", diff.Adds)
		}
	})

	t.Run("empty desired and actual", func(t *testing.T) {
		diff := diffGroups(nil, nil, "[forseti]")
		if diff.HasChanges() {
			t.Error("empty diff should have no changes")
		}
	})
}

func TestDiffDomains(t *testing.T) {
	t.Run("add and delete deny domains", func(t *testing.T) {
		diff := diffDomains(
			[]config.DenyEntry{{Domain: "ads.com"}, {Domain: "new.com"}},
			[]pihole.APIDomain{
				{ID: 1, Domain: "ads.com", Comment: "[forseti]"},
				{ID: 2, Domain: "old.com", Comment: "[forseti]"},
				{ID: 3, Domain: "manual.com", Comment: "user"},
			},
			"[forseti]",
		)
		if len(diff.Adds) != 1 || diff.Adds[0].Key != "new.com" {
			t.Errorf("expected add new.com, got %v", diff.Adds)
		}
		if len(diff.Deletes) != 1 || diff.Deletes[0].Key != "old.com" {
			t.Errorf("expected delete old.com, got %v", diff.Deletes)
		}
		if diff.Unchanged != 1 {
			t.Errorf("unchanged = %d, want 1", diff.Unchanged)
		}
	})

	t.Run("unmanaged domains preserved", func(t *testing.T) {
		diff := diffDomains(
			nil,
			[]pihole.APIDomain{{ID: 1, Domain: "manual.com", Comment: "user added"}},
			"[forseti]",
		)
		if len(diff.Deletes) != 0 {
			t.Errorf("should not delete unmanaged domain, got %v", diff.Deletes)
		}
	})
}

func TestDiffAllowDomains(t *testing.T) {
	t.Run("add new allow domain", func(t *testing.T) {
		diff := diffAllowDomains(
			[]config.AllowEntry{{Domain: "good.com"}},
			nil,
			"[forseti]",
		)
		if len(diff.Adds) != 1 || diff.Adds[0].Key != "good.com" {
			t.Errorf("expected add good.com, got %v", diff.Adds)
		}
	})

	t.Run("existing allow domain unchanged", func(t *testing.T) {
		diff := diffAllowDomains(
			[]config.AllowEntry{{Domain: "good.com"}},
			[]pihole.APIDomain{{ID: 1, Domain: "good.com", Comment: "[forseti]"}},
			"[forseti]",
		)
		if diff.Unchanged != 1 {
			t.Errorf("unchanged = %d, want 1", diff.Unchanged)
		}
		if diff.HasChanges() {
			t.Error("should have no changes")
		}
	})

	t.Run("delete managed allow domain", func(t *testing.T) {
		diff := diffAllowDomains(
			nil,
			[]pihole.APIDomain{{ID: 1, Domain: "old-allow.com", Comment: "[forseti]"}},
			"[forseti]",
		)
		if len(diff.Deletes) != 1 || diff.Deletes[0].Key != "old-allow.com" {
			t.Errorf("expected delete old-allow.com, got %v", diff.Deletes)
		}
	})

	t.Run("unmanaged allow domain preserved", func(t *testing.T) {
		diff := diffAllowDomains(
			nil,
			[]pihole.APIDomain{{ID: 1, Domain: "user.com", Comment: "manual"}},
			"[forseti]",
		)
		if len(diff.Deletes) != 0 {
			t.Errorf("should not delete unmanaged allow domain, got %v", diff.Deletes)
		}
	})
}

func TestDiffDNS(t *testing.T) {
	t.Run("add new DNS record", func(t *testing.T) {
		diff := diffDNS(
			[]config.LocalDNSEntry{{IP: "192.168.1.1", Domain: "router.local"}},
			nil,
		)
		if len(diff.Adds) != 1 || diff.Adds[0].Key != "192.168.1.1 router.local" {
			t.Errorf("expected add, got %v", diff.Adds)
		}
	})

	t.Run("existing DNS record unchanged", func(t *testing.T) {
		diff := diffDNS(
			[]config.LocalDNSEntry{{IP: "192.168.1.1", Domain: "router.local"}},
			[]pihole.APIDNSRecord{{IP: "192.168.1.1", Domain: "router.local"}},
		)
		if diff.Unchanged != 1 {
			t.Errorf("unchanged = %d, want 1", diff.Unchanged)
		}
		if diff.HasChanges() {
			t.Error("should have no changes")
		}
	})

	t.Run("delete DNS record not in desired", func(t *testing.T) {
		diff := diffDNS(
			nil,
			[]pihole.APIDNSRecord{{IP: "10.0.0.1", Domain: "old.local"}},
		)
		if len(diff.Deletes) != 1 || diff.Deletes[0].Key != "10.0.0.1 old.local" {
			t.Errorf("expected delete, got %v", diff.Deletes)
		}
	})

	t.Run("mixed add delete and unchanged", func(t *testing.T) {
		diff := diffDNS(
			[]config.LocalDNSEntry{
				{IP: "192.168.1.1", Domain: "router.local"},
				{IP: "192.168.1.2", Domain: "new.local"},
			},
			[]pihole.APIDNSRecord{
				{IP: "192.168.1.1", Domain: "router.local"},
				{IP: "10.0.0.1", Domain: "old.local"},
			},
		)
		if len(diff.Adds) != 1 {
			t.Errorf("adds = %d, want 1", len(diff.Adds))
		}
		if len(diff.Deletes) != 1 {
			t.Errorf("deletes = %d, want 1", len(diff.Deletes))
		}
		if diff.Unchanged != 1 {
			t.Errorf("unchanged = %d, want 1", diff.Unchanged)
		}
	})

	t.Run("empty desired and actual", func(t *testing.T) {
		diff := diffDNS(nil, nil)
		if diff.HasChanges() {
			t.Error("empty diff should have no changes")
		}
	})
}

func TestDiffClients(t *testing.T) {
	t.Run("add new client", func(t *testing.T) {
		diff := diffClients(
			[]config.ClientEntry{{Match: "192.168.1.100"}},
			nil,
			"[forseti]",
		)
		if len(diff.Adds) != 1 || diff.Adds[0].Key != "192.168.1.100" {
			t.Errorf("expected add, got %v", diff.Adds)
		}
	})

	t.Run("existing client unchanged", func(t *testing.T) {
		diff := diffClients(
			[]config.ClientEntry{{Match: "192.168.1.100"}},
			[]pihole.APIClient{{ID: 1, Client: "192.168.1.100", Comment: "[forseti]"}},
			"[forseti]",
		)
		if diff.Unchanged != 1 {
			t.Errorf("unchanged = %d, want 1", diff.Unchanged)
		}
	})

	t.Run("delete managed client", func(t *testing.T) {
		diff := diffClients(
			nil,
			[]pihole.APIClient{{ID: 1, Client: "192.168.1.200", Comment: "[forseti]"}},
			"[forseti]",
		)
		if len(diff.Deletes) != 1 || diff.Deletes[0].Key != "192.168.1.200" {
			t.Errorf("expected delete, got %v", diff.Deletes)
		}
	})

	t.Run("unmanaged client preserved", func(t *testing.T) {
		diff := diffClients(
			nil,
			[]pihole.APIClient{{ID: 1, Client: "192.168.1.200", Comment: "manual"}},
			"[forseti]",
		)
		if len(diff.Deletes) != 0 {
			t.Errorf("should not delete unmanaged client, got %v", diff.Deletes)
		}
	})
}

// --- Helper function tests ---

func TestBuildGroupNameToID(t *testing.T) {
	t.Run("maps names to IDs", func(t *testing.T) {
		m := buildGroupNameToID([]pihole.APIGroup{
			{ID: 1, Name: "Kids"},
			{ID: 2, Name: "Work"},
		})
		if m["kids"] != 1 {
			t.Errorf("kids = %d, want 1", m["kids"])
		}
		if m["work"] != 2 {
			t.Errorf("work = %d, want 2", m["work"])
		}
	})

	t.Run("empty groups", func(t *testing.T) {
		m := buildGroupNameToID(nil)
		if len(m) != 0 {
			t.Errorf("expected empty map, got %v", m)
		}
	})
}

func TestCollectKeys(t *testing.T) {
	t.Run("collects keys", func(t *testing.T) {
		keys := collectKeys([]DiffEntry{
			{Key: "a"},
			{Key: "b"},
		})
		if len(keys) != 2 || keys[0] != "a" || keys[1] != "b" {
			t.Errorf("expected [a b], got %v", keys)
		}
	})

	t.Run("empty input", func(t *testing.T) {
		keys := collectKeys(nil)
		if len(keys) != 0 {
			t.Errorf("expected empty, got %v", keys)
		}
	})
}

func TestFindAdlistByURL(t *testing.T) {
	adlists := []config.Adlist{
		{URL: "https://a.com/list.txt", Comment: "A"},
		{URL: "https://b.com/list.txt", Comment: "B"},
	}

	t.Run("found", func(t *testing.T) {
		a := findAdlistByURL(adlists, "https://b.com/list.txt")
		if a.Comment != "B" {
			t.Errorf("expected B, got %v", a)
		}
	})

	t.Run("not found returns zero value", func(t *testing.T) {
		a := findAdlistByURL(adlists, "https://c.com/list.txt")
		if a.URL != "" {
			t.Errorf("expected zero Adlist, got %v", a)
		}
	})
}

func TestFindClientByMatch(t *testing.T) {
	clients := []config.ClientEntry{
		{Match: "192.168.1.1", Comment: "router"},
		{Match: "192.168.1.2", Comment: "desktop"},
	}

	t.Run("found", func(t *testing.T) {
		c := findClientByMatch(clients, "192.168.1.2")
		if c.Comment != "desktop" {
			t.Errorf("expected desktop, got %v", c)
		}
	})

	t.Run("not found returns zero value", func(t *testing.T) {
		c := findClientByMatch(clients, "10.0.0.1")
		if c.Match != "" {
			t.Errorf("expected zero ClientEntry, got %v", c)
		}
	})
}

func TestResolveGroupIDs(t *testing.T) {
	nameToID := map[string]int{
		"kids": 1,
		"work": 2,
	}

	t.Run("resolves existing groups", func(t *testing.T) {
		ids := resolveGroupIDs([]string{"Kids", "Work"}, nameToID)
		if len(ids) != 2 || ids[0] != 1 || ids[1] != 2 {
			t.Errorf("expected [1 2], got %v", ids)
		}
	})

	t.Run("skips unknown groups", func(t *testing.T) {
		ids := resolveGroupIDs([]string{"Kids", "Unknown"}, nameToID)
		if len(ids) != 1 || ids[0] != 1 {
			t.Errorf("expected [1], got %v", ids)
		}
	})

	t.Run("empty input", func(t *testing.T) {
		ids := resolveGroupIDs(nil, nameToID)
		if len(ids) != 0 {
			t.Errorf("expected empty, got %v", ids)
		}
	})
}

func TestResourceDiffHasChanges(t *testing.T) {
	t.Run("no changes", func(t *testing.T) {
		d := ResourceDiff{Unchanged: 5}
		if d.HasChanges() {
			t.Error("should not have changes")
		}
	})

	t.Run("has adds", func(t *testing.T) {
		d := ResourceDiff{Adds: []DiffEntry{{Key: "x"}}}
		if !d.HasChanges() {
			t.Error("should have changes")
		}
	})

	t.Run("has deletes", func(t *testing.T) {
		d := ResourceDiff{Deletes: []DiffEntry{{Key: "x"}}}
		if !d.HasChanges() {
			t.Error("should have changes")
		}
	})
}

// --- Plan error path tests ---

func TestPlanListErrors(t *testing.T) {
	cfg := &config.Config{}
	target := config.Target{Name: "test"}
	marker := "[forseti]"

	t.Run("ListGroups error", func(t *testing.T) {
		mock := newMockAPI()
		mock.listGroupsErr = errors.New("fail")
		_, err := Plan(cfg, target, mock, marker)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("ListAdlists error", func(t *testing.T) {
		mock := newMockAPI()
		mock.listAdlistsErr = errors.New("fail")
		_, err := Plan(cfg, target, mock, marker)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("ListDomains deny error", func(t *testing.T) {
		mock := newMockAPI()
		mock.listDomainsErr["deny/exact"] = errors.New("fail")
		_, err := Plan(cfg, target, mock, marker)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("ListDomains allow error", func(t *testing.T) {
		mock := newMockAPI()
		mock.listDomainsErr["allow/exact"] = errors.New("fail")
		_, err := Plan(cfg, target, mock, marker)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("ListDNSRecords error", func(t *testing.T) {
		mock := newMockAPI()
		mock.listDNSErr = errors.New("fail")
		_, err := Plan(cfg, target, mock, marker)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("ListClients error", func(t *testing.T) {
		mock := newMockAPI()
		mock.listClientsErr = errors.New("fail")
		_, err := Plan(cfg, target, mock, marker)
		if err == nil {
			t.Error("expected error")
		}
	})
}

// --- Apply comprehensive tests ---

func TestThreeWayDiff(t *testing.T) {
	mock := newMockAPI()
	mock.adlists = []pihole.APIList{
		{ID: 1, Address: "A", Comment: ""},
		{ID: 3, Address: "C", Comment: ""},
		{ID: 4, Address: "D", Comment: "[forseti] managed"},
		{ID: 5, Address: "E", Comment: "manual entry"},
	}

	cfg := &config.Config{
		Adlists: []config.Adlist{
			{URL: "A"},
			{URL: "B"},
			{URL: "C"},
		},
	}

	target := config.Target{Name: "test"}
	report, err := Plan(cfg, target, mock, "[forseti]")
	if err != nil {
		t.Fatalf("Plan() error: %v", err)
	}

	if len(report.Adlists.Adds) != 1 || report.Adlists.Adds[0].Key != "B" {
		t.Errorf("expected add B, got adds: %v", report.Adlists.Adds)
	}
	if len(report.Adlists.Deletes) != 1 || report.Adlists.Deletes[0].Key != "D" {
		t.Errorf("expected delete D, got deletes: %v", report.Adlists.Deletes)
	}
	if report.Adlists.Unchanged != 2 {
		t.Errorf("unchanged = %d, want 2", report.Adlists.Unchanged)
	}
}

func TestReconcileOrdering(t *testing.T) {
	mock := newMockAPI()

	cfg := &config.Config{
		Groups:  []config.Group{{Name: "kids"}},
		Clients: []config.ClientEntry{{Match: "192.168.1.100", Groups: []string{"kids"}}},
	}

	target := config.Target{Name: "test"}
	report, err := Apply(cfg, target, mock, "[forseti]")
	if err != nil {
		t.Fatalf("Apply() error: %v", err)
	}

	if len(mock.createdGroups) != 1 || mock.createdGroups[0] != "kids" {
		t.Errorf("expected group 'kids' created, got: %v", mock.createdGroups)
	}
	if len(mock.createdClients) != 1 || mock.createdClients[0] != "192.168.1.100" {
		t.Errorf("expected client created, got: %v", mock.createdClients)
	}
	if len(report.Errors) > 0 {
		t.Errorf("unexpected errors: %v", report.Errors)
	}
}

func TestGravityOnlyOnAdlistChanges(t *testing.T) {
	t.Run("adlist changes set NeedsGravity", func(t *testing.T) {
		mock := newMockAPI()
		cfg := &config.Config{
			Adlists: []config.Adlist{{URL: "https://example.com/list.txt"}},
		}
		target := config.Target{Name: "test"}
		report, _ := Apply(cfg, target, mock, "[forseti]")
		if !report.Diff.NeedsGravity {
			t.Error("NeedsGravity should be true when adlists change")
		}
	})

	t.Run("no adlist changes NeedsGravity false", func(t *testing.T) {
		mock := newMockAPI()
		mock.adlists = []pihole.APIList{
			{ID: 1, Address: "https://example.com/list.txt"},
		}
		cfg := &config.Config{
			Adlists: []config.Adlist{{URL: "https://example.com/list.txt"}},
			Deny:    []config.DenyEntry{{Domain: "ads.example.com"}},
		}
		target := config.Target{Name: "test"}
		report, _ := Apply(cfg, target, mock, "[forseti]")
		if report.Diff.NeedsGravity {
			t.Error("NeedsGravity should be false when only domains change")
		}
	})
}

func TestMultiTargetIndependentFailure(t *testing.T) {
	failMock := newMockAPI()
	failMock.listGroupsErr = errors.New("connection refused")

	cfg := &config.Config{}
	target := config.Target{Name: "failing-target"}

	_, err := Plan(cfg, target, failMock, "[forseti]")
	if err == nil {
		t.Error("expected error for failing target")
	}

	okMock := newMockAPI()
	target2 := config.Target{Name: "ok-target"}
	report, err := Plan(cfg, target2, okMock, "[forseti]")
	if err != nil {
		t.Fatalf("ok target should not error: %v", err)
	}
	if report.Target != "ok-target" {
		t.Errorf("target = %q, want ok-target", report.Target)
	}
}

func TestApplyListErrors(t *testing.T) {
	cfg := &config.Config{}
	target := config.Target{Name: "test"}
	marker := "[forseti]"

	t.Run("ListGroups error", func(t *testing.T) {
		mock := newMockAPI()
		mock.listGroupsErr = errors.New("fail")
		_, err := Apply(cfg, target, mock, marker)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("ListAdlists error", func(t *testing.T) {
		mock := newMockAPI()
		mock.listAdlistsErr = errors.New("fail")
		_, err := Apply(cfg, target, mock, marker)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("ListDomains deny error", func(t *testing.T) {
		mock := newMockAPI()
		mock.listDomainsErr["deny/exact"] = errors.New("fail")
		_, err := Apply(cfg, target, mock, marker)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("ListDomains allow error", func(t *testing.T) {
		mock := newMockAPI()
		mock.listDomainsErr["allow/exact"] = errors.New("fail")
		_, err := Apply(cfg, target, mock, marker)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("ListDNSRecords error", func(t *testing.T) {
		mock := newMockAPI()
		mock.listDNSErr = errors.New("fail")
		_, err := Apply(cfg, target, mock, marker)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("ListClients error", func(t *testing.T) {
		mock := newMockAPI()
		mock.listClientsErr = errors.New("fail")
		_, err := Apply(cfg, target, mock, marker)
		if err == nil {
			t.Error("expected error")
		}
	})
}

func TestApplyCreateErrors(t *testing.T) {
	target := config.Target{Name: "test"}
	marker := "[forseti]"

	t.Run("CreateGroup error", func(t *testing.T) {
		mock := newMockAPI()
		mock.createGroupErr = errors.New("fail")
		cfg := &config.Config{Groups: []config.Group{{Name: "test"}}}
		report, err := Apply(cfg, target, mock, marker)
		if err != nil {
			t.Fatalf("Apply should not return hard error: %v", err)
		}
		if len(report.Errors) != 1 {
			t.Errorf("expected 1 error, got %d", len(report.Errors))
		}
	})

	t.Run("CreateAdlist error", func(t *testing.T) {
		mock := newMockAPI()
		mock.createAdlistErr = errors.New("fail")
		cfg := &config.Config{Adlists: []config.Adlist{{URL: "https://x.com/l.txt"}}}
		report, err := Apply(cfg, target, mock, marker)
		if err != nil {
			t.Fatalf("Apply should not return hard error: %v", err)
		}
		if len(report.Errors) != 1 {
			t.Errorf("expected 1 error, got %d", len(report.Errors))
		}
	})

	t.Run("CreateDomain deny error", func(t *testing.T) {
		mock := newMockAPI()
		mock.createDomainErr = errors.New("fail")
		cfg := &config.Config{Deny: []config.DenyEntry{{Domain: "bad.com"}}}
		report, err := Apply(cfg, target, mock, marker)
		if err != nil {
			t.Fatalf("Apply should not return hard error: %v", err)
		}
		if len(report.Errors) != 1 {
			t.Errorf("expected 1 error, got %d", len(report.Errors))
		}
	})

	t.Run("CreateDomain allow error", func(t *testing.T) {
		mock := newMockAPI()
		mock.createDomainErr = errors.New("fail")
		cfg := &config.Config{Allow: []config.AllowEntry{{Domain: "good.com"}}}
		report, err := Apply(cfg, target, mock, marker)
		if err != nil {
			t.Fatalf("Apply should not return hard error: %v", err)
		}
		if len(report.Errors) != 1 {
			t.Errorf("expected 1 error, got %d", len(report.Errors))
		}
	})

	t.Run("AddDNSRecord error", func(t *testing.T) {
		mock := newMockAPI()
		mock.addDNSErr = errors.New("fail")
		cfg := &config.Config{LocalDNS: []config.LocalDNSEntry{{IP: "1.2.3.4", Domain: "test.local"}}}
		report, err := Apply(cfg, target, mock, marker)
		if err != nil {
			t.Fatalf("Apply should not return hard error: %v", err)
		}
		if len(report.Errors) != 1 {
			t.Errorf("expected 1 error, got %d", len(report.Errors))
		}
	})

	t.Run("CreateClient error", func(t *testing.T) {
		mock := newMockAPI()
		mock.createClientErr = errors.New("fail")
		cfg := &config.Config{Clients: []config.ClientEntry{{Match: "192.168.1.1"}}}
		report, err := Apply(cfg, target, mock, marker)
		if err != nil {
			t.Fatalf("Apply should not return hard error: %v", err)
		}
		if len(report.Errors) != 1 {
			t.Errorf("expected 1 error, got %d", len(report.Errors))
		}
	})
}

func TestApplyDeleteErrors(t *testing.T) {
	target := config.Target{Name: "test"}
	marker := "[forseti]"

	t.Run("DeleteGroups error", func(t *testing.T) {
		mock := newMockAPI()
		mock.deleteGroupsErr = errors.New("fail")
		mock.groups = []pihole.APIGroup{{ID: 1, Name: "old", Comment: "[forseti]"}}
		cfg := &config.Config{}
		report, err := Apply(cfg, target, mock, marker)
		if err != nil {
			t.Fatalf("Apply should not return hard error: %v", err)
		}
		if len(report.Errors) != 1 {
			t.Errorf("expected 1 error, got %d", len(report.Errors))
		}
	})

	t.Run("DeleteAdlists error", func(t *testing.T) {
		mock := newMockAPI()
		mock.deleteAdlistsErr = errors.New("fail")
		mock.adlists = []pihole.APIList{{ID: 1, Address: "http://old.com/l.txt", Comment: "[forseti]"}}
		cfg := &config.Config{}
		report, err := Apply(cfg, target, mock, marker)
		if err != nil {
			t.Fatalf("Apply should not return hard error: %v", err)
		}
		if len(report.Errors) != 1 {
			t.Errorf("expected 1 error, got %d", len(report.Errors))
		}
	})

	t.Run("DeleteDomains deny error", func(t *testing.T) {
		mock := newMockAPI()
		mock.deleteDomainsErr = errors.New("fail")
		mock.domains["deny/exact"] = []pihole.APIDomain{{ID: 1, Domain: "old.com", Comment: "[forseti]"}}
		cfg := &config.Config{}
		report, err := Apply(cfg, target, mock, marker)
		if err != nil {
			t.Fatalf("Apply should not return hard error: %v", err)
		}
		if len(report.Errors) != 1 {
			t.Errorf("expected 1 error, got %d", len(report.Errors))
		}
	})

	t.Run("DeleteDomains allow error", func(t *testing.T) {
		mock := newMockAPI()
		mock.deleteDomainsErr = errors.New("fail")
		mock.domains["allow/exact"] = []pihole.APIDomain{{ID: 1, Domain: "old.com", Comment: "[forseti]"}}
		cfg := &config.Config{}
		report, err := Apply(cfg, target, mock, marker)
		if err != nil {
			t.Fatalf("Apply should not return hard error: %v", err)
		}
		if len(report.Errors) != 1 {
			t.Errorf("expected 1 error, got %d", len(report.Errors))
		}
	})

	t.Run("DeleteDNSRecord error", func(t *testing.T) {
		mock := newMockAPI()
		mock.deleteDNSErr = errors.New("fail")
		mock.dnsRecords = []pihole.APIDNSRecord{{IP: "10.0.0.1", Domain: "old.local"}}
		cfg := &config.Config{}
		report, err := Apply(cfg, target, mock, marker)
		if err != nil {
			t.Fatalf("Apply should not return hard error: %v", err)
		}
		if len(report.Errors) != 1 {
			t.Errorf("expected 1 error, got %d", len(report.Errors))
		}
	})

	t.Run("DeleteClients error", func(t *testing.T) {
		mock := newMockAPI()
		mock.deleteClientsErr = errors.New("fail")
		mock.clients = []pihole.APIClient{{ID: 1, Client: "192.168.1.200", Comment: "[forseti]"}}
		cfg := &config.Config{}
		report, err := Apply(cfg, target, mock, marker)
		if err != nil {
			t.Fatalf("Apply should not return hard error: %v", err)
		}
		if len(report.Errors) != 1 {
			t.Errorf("expected 1 error, got %d", len(report.Errors))
		}
	})
}

func TestApplyFullReconcile(t *testing.T) {
	mock := newMockAPI()
	mock.groups = []pihole.APIGroup{{ID: 1, Name: "existing", Comment: ""}}
	mock.adlists = []pihole.APIList{{ID: 1, Address: "http://keep.com/l.txt", Comment: ""}}

	cfg := &config.Config{
		Groups:   []config.Group{{Name: "existing"}, {Name: "new-group"}},
		Adlists:  []config.Adlist{{URL: "http://keep.com/l.txt"}, {URL: "http://new.com/l.txt"}},
		Deny:     []config.DenyEntry{{Domain: "ads.com"}},
		Allow:    []config.AllowEntry{{Domain: "safe.com"}},
		LocalDNS: []config.LocalDNSEntry{{IP: "192.168.1.1", Domain: "router.local"}},
		Clients:  []config.ClientEntry{{Match: "192.168.1.100", Groups: []string{"new-group"}}},
	}

	target := config.Target{Name: "full-test"}
	report, err := Apply(cfg, target, mock, "[forseti]")
	if err != nil {
		t.Fatalf("Apply() error: %v", err)
	}

	if len(report.Errors) > 0 {
		t.Errorf("unexpected errors: %v", report.Errors)
	}
	if report.Diff.Target != "full-test" {
		t.Errorf("target = %q, want full-test", report.Diff.Target)
	}
	if len(mock.createdGroups) != 1 || mock.createdGroups[0] != "new-group" {
		t.Errorf("created groups = %v, want [new-group]", mock.createdGroups)
	}
	if len(mock.createdAdlists) != 1 || mock.createdAdlists[0] != "http://new.com/l.txt" {
		t.Errorf("created adlists = %v", mock.createdAdlists)
	}
	if len(mock.createdDomains) != 2 {
		t.Errorf("created domains = %v, want 2 (deny + allow)", mock.createdDomains)
	}
	if len(mock.addedDNS) != 1 {
		t.Errorf("added DNS = %v, want 1", mock.addedDNS)
	}
	if len(mock.createdClients) != 1 {
		t.Errorf("created clients = %v, want 1", mock.createdClients)
	}
}

func TestPlanNeedsGravity(t *testing.T) {
	mock := newMockAPI()
	cfg := &config.Config{
		Adlists: []config.Adlist{{URL: "http://new.com/l.txt"}},
	}
	target := config.Target{Name: "test"}
	report, err := Plan(cfg, target, mock, "[forseti]")
	if err != nil {
		t.Fatal(err)
	}
	if !report.NeedsGravity {
		t.Error("Plan should set NeedsGravity when adlists differ")
	}
}

func TestPlanNoGravityOnDomainChanges(t *testing.T) {
	mock := newMockAPI()
	cfg := &config.Config{
		Deny: []config.DenyEntry{{Domain: "ads.com"}},
	}
	target := config.Target{Name: "test"}
	report, err := Plan(cfg, target, mock, "[forseti]")
	if err != nil {
		t.Fatal(err)
	}
	if report.NeedsGravity {
		t.Error("Plan should not set NeedsGravity for domain-only changes")
	}
}
