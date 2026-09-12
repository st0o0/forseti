package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func intP(v int) *int    { return &v }
func boolP(v bool) *bool { return &v }

func TestMergeSettingsDeepMerge(t *testing.T) {
	base := Settings{
		DNS: DNSSettings{
			Upstream: []string{"1.1.1.1"},
			Cache: CacheSettings{
				Size:        intP(10000),
				ForceOnDisk: boolP(false),
			},
			RateLimit: RateLimitSettings{
				Count:    intP(1000),
				Interval: intP(60),
			},
		},
		Blocking: BlockingSettings{Mode: "NULL"},
	}

	override := Settings{
		DNS: DNSSettings{
			Cache: CacheSettings{
				ForceOnDisk: boolP(true),
			},
		},
	}

	merged := mergeSettings(base, override)

	if *merged.DNS.Cache.ForceOnDisk != true {
		t.Error("expected force_on_disk to be overridden to true")
	}
	if *merged.DNS.Cache.Size != 10000 {
		t.Error("expected cache size to be inherited from base")
	}
	if len(merged.DNS.Upstream) != 1 || merged.DNS.Upstream[0] != "1.1.1.1" {
		t.Error("expected upstream to be inherited from base")
	}
	if merged.Blocking.Mode != "NULL" {
		t.Error("expected blocking mode to be inherited from base")
	}
}

func TestMergeSettingsUpstreamOverride(t *testing.T) {
	base := Settings{
		DNS: DNSSettings{Upstream: []string{"1.1.1.1"}},
	}
	override := Settings{
		DNS: DNSSettings{Upstream: []string{"8.8.8.8", "8.8.4.4"}},
	}

	merged := mergeSettings(base, override)
	if len(merged.DNS.Upstream) != 2 || merged.DNS.Upstream[0] != "8.8.8.8" {
		t.Errorf("expected upstream override, got %v", merged.DNS.Upstream)
	}
}

func TestLoadWithOverrideFile(t *testing.T) {
	dir := t.TempDir()

	overrideContent := `
settings:
  dns:
    cache:
      force_on_disk: true
deny:
  - domain: extra.example.com
`
	targetsDir := filepath.Join(dir, "targets")
	if err := os.Mkdir(targetsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(targetsDir, "kids.yaml"), []byte(overrideContent), 0o644); err != nil {
		t.Fatalf("write override: %v", err)
	}

	mainConfig := `
targets:
  - name: pihole-main
    url: http://192.168.1.1:80
    password: test

  - name: pihole-kids
    url: http://192.168.1.2:80
    password: test
    file: targets/kids.yaml

settings:
  dns:
    cache:
      size: 10000
      force_on_disk: false

deny:
  - domain: ads.example.com
`
	if err := os.WriteFile(filepath.Join(dir, "forseti.yaml"), []byte(mainConfig), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, resolved, err := Load(filepath.Join(dir, "forseti.yaml"))
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if len(resolved) != 2 {
		t.Fatalf("expected 2 resolved targets, got %d", len(resolved))
	}

	main := resolved[0]
	kids := resolved[1]

	if len(main.Deny) != 1 {
		t.Errorf("main deny count = %d, want 1", len(main.Deny))
	}
	if *main.Settings.DNS.Cache.ForceOnDisk != false {
		t.Error("main should have force_on_disk=false")
	}

	if len(kids.Deny) != 2 {
		t.Errorf("kids deny count = %d, want 2 (global + override)", len(kids.Deny))
	}
	if *kids.Settings.DNS.Cache.ForceOnDisk != true {
		t.Error("kids should have force_on_disk=true (overridden)")
	}
	if *kids.Settings.DNS.Cache.Size != 10000 {
		t.Error("kids should inherit cache size from global")
	}
}

func TestLoadWithOverrideExclude(t *testing.T) {
	dir := t.TempDir()

	overrideContent := `
exclude:
  allow:
    - domain: gaming.example.com
`
	targetsDir := filepath.Join(dir, "targets")
	if err := os.Mkdir(targetsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(targetsDir, "kids.yaml"), []byte(overrideContent), 0o644); err != nil {
		t.Fatalf("write override: %v", err)
	}

	mainConfig := `
targets:
  - name: pihole-kids
    url: http://192.168.1.2:80
    password: test
    file: targets/kids.yaml

allow:
  - domain: safe.example.com
  - domain: gaming.example.com
`
	if err := os.WriteFile(filepath.Join(dir, "forseti.yaml"), []byte(mainConfig), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, resolved, err := Load(filepath.Join(dir, "forseti.yaml"))
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	kids := resolved[0]
	if len(kids.Allow) != 1 {
		t.Fatalf("kids allow count = %d, want 1 (gaming excluded)", len(kids.Allow))
	}
	if kids.Allow[0].Domain != "safe.example.com" {
		t.Errorf("remaining allow = %s, want safe.example.com", kids.Allow[0].Domain)
	}
}

func TestLoadOverrideMissingFile(t *testing.T) {
	dir := t.TempDir()
	mainConfig := `
targets:
  - name: test
    url: http://localhost:80
    password: test
    file: nonexistent.yaml
`
	if err := os.WriteFile(filepath.Join(dir, "forseti.yaml"), []byte(mainConfig), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, _, err := Load(filepath.Join(dir, "forseti.yaml"))
	if err == nil {
		t.Fatal("expected error for missing override file")
	}
}

func TestLoadOverrideDisallowedFields(t *testing.T) {
	dir := t.TempDir()

	overrideContent := `
targets:
  - name: sneaky
    url: http://bad
    password: nope
`
	if err := os.WriteFile(filepath.Join(dir, "override.yaml"), []byte(overrideContent), 0o644); err != nil {
		t.Fatalf("write override: %v", err)
	}

	mainConfig := `
targets:
  - name: test
    url: http://localhost:80
    password: test
    file: override.yaml
`
	if err := os.WriteFile(filepath.Join(dir, "forseti.yaml"), []byte(mainConfig), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, _, err := Load(filepath.Join(dir, "forseti.yaml"))
	if err == nil {
		t.Fatal("expected error for disallowed field in override")
	}
	if !strings.Contains(err.Error(), "not allowed") {
		t.Errorf("error should mention 'not allowed', got: %v", err)
	}
}

func TestLoadOverrideDuplicateAfterMerge(t *testing.T) {
	dir := t.TempDir()

	overrideContent := `
deny:
  - domain: ads.example.com
`
	if err := os.WriteFile(filepath.Join(dir, "override.yaml"), []byte(overrideContent), 0o644); err != nil {
		t.Fatalf("write override: %v", err)
	}

	mainConfig := `
targets:
  - name: test
    url: http://localhost:80
    password: test
    file: override.yaml
deny:
  - domain: ads.example.com
`
	if err := os.WriteFile(filepath.Join(dir, "forseti.yaml"), []byte(mainConfig), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, _, err := Load(filepath.Join(dir, "forseti.yaml"))
	if err == nil {
		t.Fatal("expected error for duplicate after merge")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("error should mention duplicate, got: %v", err)
	}
}

func TestLoadNoOverride(t *testing.T) {
	dir := t.TempDir()

	mainConfig := `
targets:
  - name: test
    url: http://localhost:80
    password: test
deny:
  - domain: ads.example.com
`
	if err := os.WriteFile(filepath.Join(dir, "forseti.yaml"), []byte(mainConfig), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, resolved, err := Load(filepath.Join(dir, "forseti.yaml"))
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(resolved) != 1 {
		t.Fatalf("expected 1 resolved target, got %d", len(resolved))
	}
	if len(resolved[0].Deny) != 1 {
		t.Errorf("deny count = %d, want 1", len(resolved[0].Deny))
	}
}

func TestSettingsValidation(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{
			name: "invalid blocking mode",
			yaml: `
targets:
  - name: test
    url: http://localhost:80
    password: test
settings:
  blocking:
    mode: INVALID
`,
			wantErr: "blocking.mode",
		},
		{
			name: "negative cache size",
			yaml: `
targets:
  - name: test
    url: http://localhost:80
    password: test
settings:
  dns:
    cache:
      size: -1
`,
			wantErr: "cache.size",
		},
		{
			name: "privacy level out of range",
			yaml: `
targets:
  - name: test
    url: http://localhost:80
    password: test
settings:
  privacy:
    level: 5
`,
			wantErr: "privacy.level",
		},
		{
			name: "invalid dns port",
			yaml: `
targets:
  - name: test
    url: http://localhost:80
    password: test
settings:
  dns:
    port: 0
`,
			wantErr: "dns.port",
		},
		{
			name: "invalid listening mode",
			yaml: `
targets:
  - name: test
    url: http://localhost:80
    password: test
settings:
  dns:
    listening_mode: invalid
`,
			wantErr: "listening_mode",
		},
		{
			name: "negative blocking timer",
			yaml: `
targets:
  - name: test
    url: http://localhost:80
    password: test
settings:
  blocking:
    timer: -1
`,
			wantErr: "blocking.timer",
		},
		{
			name: "dhcp start without end",
			yaml: `
targets:
  - name: test
    url: http://localhost:80
    password: test
settings:
  dhcp:
    start: "192.168.1.100"
`,
			wantErr: "dhcp",
		},
		{
			name: "invalid dhcp start IP",
			yaml: `
targets:
  - name: test
    url: http://localhost:80
    password: test
settings:
  dhcp:
    start: "not-an-ip"
    end: "192.168.1.200"
    router: "192.168.1.1"
`,
			wantErr: "dhcp.start",
		},
		{
			name: "invalid webserver port",
			yaml: `
targets:
  - name: test
    url: http://localhost:80
    password: test
settings:
  webserver:
    port: 99999
`,
			wantErr: "webserver.port",
		},
		{
			name: "misc disk threshold out of range",
			yaml: `
targets:
  - name: test
    url: http://localhost:80
    password: test
settings:
  misc:
    check:
      disk: 120
`,
			wantErr: "misc.check.disk",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTestConfig(t, tt.yaml)
			_, _, err := Load(path)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error should contain %q, got: %v", tt.wantErr, err)
			}
		})
	}
}
