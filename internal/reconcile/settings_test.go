package reconcile

import (
	"fmt"
	"testing"

	"github.com/st0o0/forseti/internal/config"
)

type mockSettingsAPI struct {
	config    map[string]any
	patches   []patchCall
	patchErr  error
	configErr error
}

type patchCall struct {
	Path  string
	Value any
}

func (m *mockSettingsAPI) GetConfig() (map[string]any, error) {
	if m.configErr != nil {
		return nil, m.configErr
	}
	return m.config, nil
}

func (m *mockSettingsAPI) PatchConfig(path string, value any) error {
	m.patches = append(m.patches, patchCall{Path: path, Value: value})
	return m.patchErr
}

func intPtr(v int) *int    { return &v }
func boolPtr(v bool) *bool { return &v }

func TestDiffSettings_NoSettings(t *testing.T) {
	api := &mockSettingsAPI{config: map[string]any{}}
	s := config.Settings{}

	diff, err := DiffSettings(&s, api)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff.HasChanges() {
		t.Error("expected no changes for empty settings")
	}
}

func TestDiffSettings_OneChange(t *testing.T) {
	api := &mockSettingsAPI{
		config: map[string]any{
			"dns": map[string]any{
				"cache": map[string]any{
					"optimizer": false,
					"size":      float64(10000),
				},
			},
		},
	}
	s := config.Settings{
		DNS: config.DNSSettings{
			Cache: config.CacheSettings{
				ForceOnDisk: boolPtr(true),
				Size:        intPtr(10000),
			},
		},
	}

	diff, err := DiffSettings(&s, api)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(diff.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(diff.Changes))
	}
	if diff.Changes[0].Name != "dns.cache.force_on_disk" {
		t.Errorf("expected change for dns.cache.force_on_disk, got %s", diff.Changes[0].Name)
	}
}

func TestDiffSettings_AllMatch(t *testing.T) {
	api := &mockSettingsAPI{
		config: map[string]any{
			"dns": map[string]any{
				"cache": map[string]any{
					"optimizer": true,
				},
			},
		},
	}
	s := config.Settings{
		DNS: config.DNSSettings{
			Cache: config.CacheSettings{
				ForceOnDisk: boolPtr(true),
			},
		},
	}

	diff, err := DiffSettings(&s, api)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff.HasChanges() {
		t.Errorf("expected no changes, got %d", len(diff.Changes))
	}
}

func TestDiffSettings_MultipleChanges(t *testing.T) {
	api := &mockSettingsAPI{
		config: map[string]any{
			"dns": map[string]any{
				"cache": map[string]any{
					"optimizer": false,
					"size":      float64(5000),
				},
			},
			"misc": map[string]any{
				"privacylevel": float64(0),
			},
		},
	}
	s := config.Settings{
		DNS: config.DNSSettings{
			Cache: config.CacheSettings{
				ForceOnDisk: boolPtr(true),
				Size:        intPtr(10000),
			},
		},
		Privacy: config.PrivacySettings{
			Level: intPtr(3),
		},
	}

	diff, err := DiffSettings(&s, api)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(diff.Changes) != 3 {
		t.Fatalf("expected 3 changes, got %d", len(diff.Changes))
	}
}

func TestApplySettings_AppliesChanges(t *testing.T) {
	api := &mockSettingsAPI{
		config: map[string]any{
			"dns": map[string]any{
				"cache": map[string]any{
					"optimizer": false,
				},
			},
		},
	}
	s := config.Settings{
		DNS: config.DNSSettings{
			Cache: config.CacheSettings{
				ForceOnDisk: boolPtr(true),
			},
		},
	}

	diff, err := ApplySettings("test", &s, api)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(diff.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(diff.Changes))
	}
	if len(api.patches) != 1 {
		t.Fatalf("expected 1 patch call, got %d", len(api.patches))
	}
	if api.patches[0].Path != "dns/cache/optimizer" {
		t.Errorf("expected path dns/cache/optimizer, got %s", api.patches[0].Path)
	}
}

func TestApplySettings_NoChanges(t *testing.T) {
	api := &mockSettingsAPI{
		config: map[string]any{
			"dns": map[string]any{
				"cache": map[string]any{
					"optimizer": true,
				},
			},
		},
	}
	s := config.Settings{
		DNS: config.DNSSettings{
			Cache: config.CacheSettings{
				ForceOnDisk: boolPtr(true),
			},
		},
	}

	diff, err := ApplySettings("test", &s, api)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff.HasChanges() {
		t.Error("expected no changes")
	}
	if len(api.patches) != 0 {
		t.Errorf("expected 0 patch calls, got %d", len(api.patches))
	}
}

func TestDiffSettings_GetConfigError(t *testing.T) {
	api := &mockSettingsAPI{
		configErr: fmt.Errorf("connection refused"),
	}
	s := config.Settings{
		DNS: config.DNSSettings{
			Cache: config.CacheSettings{
				ForceOnDisk: boolPtr(true),
			},
		},
	}

	_, err := DiffSettings(&s, api)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildDesiredSettingsList_NewDNSFields(t *testing.T) {
	s := &config.Settings{
		DNS: config.DNSSettings{
			Port:          intPtr(5353),
			DomainNeeded:  boolPtr(true),
			BogusPriv:     boolPtr(true),
			DNSSEC:        boolPtr(false),
			ListeningMode: "all",
			QueryLogging:  boolPtr(true),
			ResolveIPv4:   boolPtr(true),
			ResolveIPv6:   boolPtr(false),
			RevServer: config.RevServerSettings{
				Enabled: boolPtr(true),
				CIDR:    "192.168.1.0/24",
				Target:  "192.168.1.1",
				Domain:  "lan",
			},
		},
	}

	mappings := BuildDesiredSettingsList(s)
	expected := map[string]string{
		"dns.port":               "dns/port",
		"dns.domain_needed":      "dns/domainNeeded",
		"dns.bogus_priv":         "dns/bogusPriv",
		"dns.dnssec":             "dns/dnssec",
		"dns.listening_mode":     "dns/listeningMode",
		"dns.query_logging":      "dns/queryLogging",
		"dns.resolve_ipv4":       "dns/resolveIPv4",
		"dns.resolve_ipv6":       "dns/resolveIPv6",
		"dns.rev_server.enabled": "dns/revServer/active",
		"dns.rev_server.cidr":    "dns/revServer/cidr",
		"dns.rev_server.target":  "dns/revServer/target",
		"dns.rev_server.domain":  "dns/revServer/domain",
	}

	found := make(map[string]bool)
	for _, m := range mappings {
		found[m.ForsetiPath] = true
		if exp, ok := expected[m.ForsetiPath]; ok {
			if m.PiholePath != exp {
				t.Errorf("%s: pihole path = %q, want %q", m.ForsetiPath, m.PiholePath, exp)
			}
		}
	}
	for k := range expected {
		if !found[k] {
			t.Errorf("missing mapping for %s", k)
		}
	}
}

func TestBuildDesiredSettingsList_DHCPFields(t *testing.T) {
	s := &config.Settings{
		DHCP: config.DHCPSettings{
			Active:      boolPtr(true),
			Start:       "192.168.1.100",
			End:         "192.168.1.200",
			Router:      "192.168.1.1",
			LeaseTime:   intPtr(24),
			Domain:      "lan",
			IPv6:        boolPtr(false),
			RapidCommit: boolPtr(true),
		},
	}

	mappings := BuildDesiredSettingsList(s)
	expected := map[string]string{
		"dhcp.active":       "dhcp/active",
		"dhcp.start":        "dhcp/start",
		"dhcp.end":          "dhcp/end",
		"dhcp.router":       "dhcp/router",
		"dhcp.lease_time":   "dhcp/leaseTime",
		"dhcp.domain":       "dhcp/domain",
		"dhcp.ipv6":         "dhcp/ipv6",
		"dhcp.rapid_commit": "dhcp/rapidCommit",
	}

	found := make(map[string]bool)
	for _, m := range mappings {
		found[m.ForsetiPath] = true
	}
	for k := range expected {
		if !found[k] {
			t.Errorf("missing mapping for %s", k)
		}
	}
}

func TestBuildDesiredSettingsList_MiscAndWebserver(t *testing.T) {
	s := &config.Settings{
		Webserver: config.WebserverSettings{Port: intPtr(8080)},
		Misc: config.MiscSettings{
			Nice:         intPtr(-10),
			DelayStartup: intPtr(5),
			Check: config.CheckSettings{
				Load:  boolPtr(true),
				Disk:  intPtr(90),
				Shmem: intPtr(80),
			},
		},
	}

	mappings := BuildDesiredSettingsList(s)
	expected := []string{
		"webserver.port", "misc.nice", "misc.delay_startup",
		"misc.check.load", "misc.check.disk", "misc.check.shmem",
	}

	found := make(map[string]bool)
	for _, m := range mappings {
		found[m.ForsetiPath] = true
	}
	for _, k := range expected {
		if !found[k] {
			t.Errorf("missing mapping for %s", k)
		}
	}
}

func TestBuildDesiredSettingsList_BlockingNewFields(t *testing.T) {
	s := &config.Settings{
		Blocking: config.BlockingSettings{
			Active: boolPtr(true),
			Timer:  intPtr(30),
		},
	}

	mappings := BuildDesiredSettingsList(s)
	found := make(map[string]bool)
	for _, m := range mappings {
		found[m.ForsetiPath] = true
	}

	if !found["blocking.active"] {
		t.Error("missing mapping for blocking.active")
	}
	if !found["blocking.timer"] {
		t.Error("missing mapping for blocking.timer")
	}
}
