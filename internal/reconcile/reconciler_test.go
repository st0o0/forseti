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
	listGroupsErr   error
	nextGroupID     int
}

func newMockAPI() *mockAPI {
	return &mockAPI{
		domains:     make(map[string][]pihole.APIDomain),
		nextGroupID: 100,
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
	g := &pihole.APIGroup{ID: m.nextGroupID, Name: name, Comment: comment}
	m.nextGroupID++
	return g, nil
}
func (m *mockAPI) DeleteGroups(names []string) error {
	m.deletedGroups = append(m.deletedGroups, names...)
	return nil
}

func (m *mockAPI) ListAdlists() ([]pihole.APIList, error) { return m.adlists, nil }
func (m *mockAPI) CreateAdlist(address, comment string, enabled bool, groups []int) (*pihole.APIList, error) {
	m.createdAdlists = append(m.createdAdlists, address)
	return &pihole.APIList{ID: 99, Address: address}, nil
}
func (m *mockAPI) DeleteAdlists(addresses []string) error {
	m.deletedAdlists2 = append(m.deletedAdlists2, addresses...)
	return nil
}

func (m *mockAPI) ListDomains(domType, kind string) ([]pihole.APIDomain, error) {
	return m.domains[domType+"/"+kind], nil
}
func (m *mockAPI) CreateDomain(domType, kind, domain, comment string, enabled bool, groups []int) (*pihole.APIDomain, error) {
	m.createdDomains = append(m.createdDomains, domType+":"+domain)
	return &pihole.APIDomain{ID: 99, Domain: domain}, nil
}
func (m *mockAPI) DeleteDomains(domains []string) error {
	m.deletedDomains2 = append(m.deletedDomains2, domains...)
	return nil
}

func (m *mockAPI) ListDNSRecords() ([]pihole.APIDNSRecord, error) { return m.dnsRecords, nil }
func (m *mockAPI) AddDNSRecord(ip, domain string) error {
	m.addedDNS = append(m.addedDNS, ip+" "+domain)
	return nil
}
func (m *mockAPI) DeleteDNSRecord(ip, domain string) error {
	m.deletedDNS = append(m.deletedDNS, ip+" "+domain)
	return nil
}

func (m *mockAPI) ListClients() ([]pihole.APIClient, error) { return m.clients, nil }
func (m *mockAPI) CreateClient(ip, comment string, groups []int) (*pihole.APIClient, error) {
	m.createdClients = append(m.createdClients, ip)
	return &pihole.APIClient{ID: 99, Client: ip}, nil
}
func (m *mockAPI) DeleteClients(ips []string) error {
	m.deletedClients2 = append(m.deletedClients2, ips...)
	return nil
}

func TestThreeWayDiff(t *testing.T) {
	// Desired: [A, B, C], Actual: [A, C, D(managed), E(unmanaged)]
	// Result: add B, delete D, leave E alone
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

	// A second target should work independently
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
