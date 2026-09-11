package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeTestConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "forseti.yml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadFullConfig(t *testing.T) {
	t.Setenv("TEST_PASSWORD", "secret123")

	cfg := `
metrics:
  port: 8080
  path: /prom
  scrape_interval: 1m

targets:
  - name: pihole-router
    url: http://192.168.1.1:80
    password: ${TEST_PASSWORD}

reconcile:
  interval: 5m
  marker: "[managed]"
  gravity_on_change: false

groups:
  - name: default
    comment: "Default group"
  - name: kids
    comment: "Kids group"

adlists:
  - url: https://example.com/list.txt
    comment: "Test list"
    groups: [default]

deny:
  - domain: ads.example.com

allow:
  - domain: safe.example.com

local_dns:
  - domain: nas.lan
    ip: 192.168.1.50

clients:
  - match: 192.168.1.0/24
    comment: "LAN"
    groups: [default, kids]
`
	path := writeTestConfig(t, cfg)
	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if c.Metrics.Port != 8080 {
		t.Errorf("Metrics.Port = %d, want 8080", c.Metrics.Port)
	}
	if c.Metrics.Path != "/prom" {
		t.Errorf("Metrics.Path = %q, want /prom", c.Metrics.Path)
	}
	if c.Metrics.ScrapeInterval.Duration != time.Minute {
		t.Errorf("ScrapeInterval = %v, want 1m", c.Metrics.ScrapeInterval.Duration)
	}
	if len(c.Targets) != 1 {
		t.Fatalf("len(Targets) = %d, want 1", len(c.Targets))
	}
	if c.Targets[0].Password != "secret123" {
		t.Errorf("Target password = %q, want secret123", c.Targets[0].Password)
	}
	if c.Reconcile.Marker != "[managed]" {
		t.Errorf("Marker = %q, want [managed]", c.Reconcile.Marker)
	}
	if *c.Reconcile.GravityOnChange != false {
		t.Error("GravityOnChange = true, want false")
	}
	if len(c.Groups) != 2 {
		t.Errorf("len(Groups) = %d, want 2", len(c.Groups))
	}
	if len(c.Adlists) != 1 {
		t.Errorf("len(Adlists) = %d, want 1", len(c.Adlists))
	}
	if len(c.Deny) != 1 {
		t.Errorf("len(Deny) = %d, want 1", len(c.Deny))
	}
	if len(c.Allow) != 1 {
		t.Errorf("len(Allow) = %d, want 1", len(c.Allow))
	}
	if len(c.LocalDNS) != 1 {
		t.Errorf("len(LocalDNS) = %d, want 1", len(c.LocalDNS))
	}
	if len(c.Clients) != 1 {
		t.Errorf("len(Clients) = %d, want 1", len(c.Clients))
	}
}

func TestLoadMinimalConfig(t *testing.T) {
	cfg := `
targets:
  - name: pihole
    url: http://localhost:80
    password: test
`
	path := writeTestConfig(t, cfg)
	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(c.Targets) != 1 {
		t.Errorf("len(Targets) = %d, want 1", len(c.Targets))
	}
	if len(c.Adlists) != 0 {
		t.Errorf("len(Adlists) = %d, want 0", len(c.Adlists))
	}
}

func TestEnvVarExpansion(t *testing.T) {
	t.Setenv("MY_PASS", "hunter2")

	cfg := `
targets:
  - name: test
    url: http://localhost:80
    password: ${MY_PASS}
`
	path := writeTestConfig(t, cfg)
	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if c.Targets[0].Password != "hunter2" {
		t.Errorf("password = %q, want hunter2", c.Targets[0].Password)
	}
}

func TestEnvVarUndefined(t *testing.T) {
	os.Unsetenv("DEFINITELY_NOT_SET_12345")

	cfg := `
targets:
  - name: test
    url: http://localhost:80
    password: ${DEFINITELY_NOT_SET_12345}
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for undefined env var")
	}
	if !strings.Contains(err.Error(), "DEFINITELY_NOT_SET_12345") {
		t.Errorf("error should mention var name, got: %v", err)
	}
}

func TestDurationParsing(t *testing.T) {
	cfg := `
targets:
  - name: test
    url: http://localhost:80
    password: test
reconcile:
  interval: 5m
`
	path := writeTestConfig(t, cfg)
	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if c.Reconcile.Interval.Duration != 5*time.Minute {
		t.Errorf("interval = %v, want 5m", c.Reconcile.Interval.Duration)
	}
}

func TestDurationDecodeError(t *testing.T) {
	cfg := `
targets:
  - name: test
    url: http://localhost:80
    password: test
reconcile:
  interval:
    - not
    - a
    - string
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for non-string duration")
	}
}

func TestDurationInvalid(t *testing.T) {
	cfg := `
targets:
  - name: test
    url: http://localhost:80
    password: test
reconcile:
  interval: banana
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid duration")
	}
}

func TestValidationInvalidURL(t *testing.T) {
	cfg := `
targets:
  - name: test
    url: not-a-url
    password: test
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
	if !strings.Contains(err.Error(), "invalid URL") {
		t.Errorf("error should mention invalid URL, got: %v", err)
	}
}

func TestValidationFTPURL(t *testing.T) {
	cfg := `
targets:
  - name: test
    url: ftp://localhost:80
    password: test
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for ftp URL")
	}
	if !strings.Contains(err.Error(), "invalid URL") {
		t.Errorf("error should mention invalid URL, got: %v", err)
	}
}

func TestValidationURLNoHost(t *testing.T) {
	cfg := `
targets:
  - name: test
    url: http://
    password: test
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for URL with no host")
	}
}

func TestValidationInvalidDomain(t *testing.T) {
	cfg := `
targets:
  - name: test
    url: http://localhost:80
    password: test
deny:
  - domain: "not a domain!"
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid domain")
	}
	if !strings.Contains(err.Error(), "invalid domain") {
		t.Errorf("error should mention invalid domain, got: %v", err)
	}
}

func TestValidationInvalidCIDR(t *testing.T) {
	cfg := `
targets:
  - name: test
    url: http://localhost:80
    password: test
clients:
  - match: "not-a-cidr"
    groups: []
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid CIDR")
	}
	if !strings.Contains(err.Error(), "invalid IP or CIDR") {
		t.Errorf("error should mention invalid CIDR, got: %v", err)
	}
}

func TestValidationMissingGroupReference(t *testing.T) {
	cfg := `
targets:
  - name: test
    url: http://localhost:80
    password: test
groups:
  - name: real-group
adlists:
  - url: https://example.com/list.txt
    groups: [nonexistent]
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing group reference")
	}
	if !strings.Contains(err.Error(), "undefined group") {
		t.Errorf("error should mention undefined group, got: %v", err)
	}
}

func TestValidationDuplicateAdlistURL(t *testing.T) {
	cfg := `
targets:
  - name: test
    url: http://localhost:80
    password: test
adlists:
  - url: https://example.com/list.txt
  - url: https://example.com/list.txt
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for duplicate adlist URL")
	}
	if !strings.Contains(err.Error(), "duplicate adlist URL") {
		t.Errorf("error should mention duplicate, got: %v", err)
	}
}

func TestValidationDuplicateDenyDomain(t *testing.T) {
	cfg := `
targets:
  - name: test
    url: http://localhost:80
    password: test
deny:
  - domain: ads.example.com
  - domain: ads.example.com
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for duplicate deny domain")
	}
	if !strings.Contains(err.Error(), "duplicate domain") {
		t.Errorf("error should mention duplicate domain, got: %v", err)
	}
}

func TestValidationDuplicateGroupName(t *testing.T) {
	cfg := `
targets:
  - name: test
    url: http://localhost:80
    password: test
groups:
  - name: same
  - name: same
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for duplicate group name")
	}
	if !strings.Contains(err.Error(), "duplicate group name") {
		t.Errorf("error should mention duplicate group, got: %v", err)
	}
}

func TestDefaults(t *testing.T) {
	cfg := `
targets:
  - name: test
    url: http://localhost:80
    password: test
`
	path := writeTestConfig(t, cfg)
	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if c.Metrics.Port != 9099 {
		t.Errorf("default port = %d, want 9099", c.Metrics.Port)
	}
	if c.Metrics.Path != "/metrics" {
		t.Errorf("default path = %q, want /metrics", c.Metrics.Path)
	}
	if c.Metrics.ScrapeInterval.Duration != 30*time.Second {
		t.Errorf("default scrape_interval = %v, want 30s", c.Metrics.ScrapeInterval.Duration)
	}
	if c.Reconcile.Marker != "[forseti]" {
		t.Errorf("default marker = %q, want [forseti]", c.Reconcile.Marker)
	}
	if *c.Reconcile.GravityOnChange != true {
		t.Error("default gravity_on_change = false, want true")
	}
}

func TestValidationNoTargets(t *testing.T) {
	cfg := `
groups:
  - name: test
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for no targets")
	}
	if !strings.Contains(err.Error(), "at least one target") {
		t.Errorf("error should mention targets required, got: %v", err)
	}
}

func TestValidationInvalidMode(t *testing.T) {
	cfg := `
mode: banana
targets:
  - name: test
    url: http://localhost:80
    password: test
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid mode")
	}
	if !strings.Contains(err.Error(), "mode must be") {
		t.Errorf("error should mention mode, got: %v", err)
	}
}

func TestValidationMissingTargetName(t *testing.T) {
	cfg := `
targets:
  - url: http://localhost:80
    password: test
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing target name")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("error should mention name, got: %v", err)
	}
}

func TestValidationMissingTargetURL(t *testing.T) {
	cfg := `
targets:
  - name: test
    password: test
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing target URL")
	}
	if !strings.Contains(err.Error(), "url is required") {
		t.Errorf("error should mention url, got: %v", err)
	}
}

func TestValidationMissingTargetPassword(t *testing.T) {
	cfg := `
targets:
  - name: test
    url: http://localhost:80
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing password")
	}
	if !strings.Contains(err.Error(), "password is required") {
		t.Errorf("error should mention password, got: %v", err)
	}
}

func TestValidationGravityScheduleFields(t *testing.T) {
	cfg := `
targets:
  - name: test
    url: http://localhost:80
    password: test
    gravity:
      schedule: "1 2 3"
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid gravity schedule fields")
	}
	if !strings.Contains(err.Error(), "gravity schedule must have 5 fields") {
		t.Errorf("error should mention 5 fields, got: %v", err)
	}
}

func TestValidationSyncModeRequiresRole(t *testing.T) {
	cfg := `
mode: sync
targets:
  - name: primary
    url: http://localhost:80
    password: test
    role: primary
  - name: replica
    url: http://localhost:81
    password: test
sync:
  primary: primary
  resources: [adlists]
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing role on replica")
	}
	if !strings.Contains(err.Error(), "role is required") {
		t.Errorf("error should mention role, got: %v", err)
	}
}

func TestValidationSyncModeInvalidRole(t *testing.T) {
	cfg := `
mode: sync
targets:
  - name: primary
    url: http://localhost:80
    password: test
    role: primary
  - name: replica
    url: http://localhost:81
    password: test
    role: backup
sync:
  primary: primary
  resources: [adlists]
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid role")
	}
	if !strings.Contains(err.Error(), "role must be") {
		t.Errorf("error should mention role, got: %v", err)
	}
}

func TestValidationSyncNoPrimary(t *testing.T) {
	cfg := `
mode: sync
targets:
  - name: one
    url: http://localhost:80
    password: test
    role: replica
  - name: two
    url: http://localhost:81
    password: test
    role: replica
sync:
  primary: one
  resources: [adlists]
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for no primary")
	}
}

func TestValidationSyncMultiplePrimaries(t *testing.T) {
	cfg := `
mode: sync
targets:
  - name: one
    url: http://localhost:80
    password: test
    role: primary
  - name: two
    url: http://localhost:81
    password: test
    role: primary
sync:
  primary: one
  resources: [adlists]
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for multiple primaries")
	}
}

func TestValidationSyncMissingPrimaryField(t *testing.T) {
	cfg := `
mode: sync
targets:
  - name: one
    url: http://localhost:80
    password: test
    role: primary
  - name: two
    url: http://localhost:81
    password: test
    role: replica
sync:
  resources: [adlists]
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing sync.primary")
	}
	if !strings.Contains(err.Error(), "sync.primary is required") {
		t.Errorf("error should mention sync.primary, got: %v", err)
	}
}

func TestValidationSyncPrimaryPointsToReplica(t *testing.T) {
	cfg := `
mode: sync
targets:
  - name: one
    url: http://localhost:80
    password: test
    role: primary
  - name: two
    url: http://localhost:81
    password: test
    role: replica
sync:
  primary: two
  resources: [adlists]
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error when sync.primary points to a replica")
	}
	if !strings.Contains(err.Error(), "must reference a target with role: primary") {
		t.Errorf("error should mention role: primary, got: %v", err)
	}
}

func TestValidationSyncPrimaryMismatch(t *testing.T) {
	cfg := `
mode: sync
targets:
  - name: one
    url: http://localhost:80
    password: test
    role: primary
  - name: two
    url: http://localhost:81
    password: test
    role: replica
sync:
  primary: nonexistent
  resources: [adlists]
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for primary mismatch")
	}
	if !strings.Contains(err.Error(), "must reference a target with role: primary") {
		t.Errorf("error should mention reference, got: %v", err)
	}
}

func TestValidationSyncNoResources(t *testing.T) {
	cfg := `
mode: sync
targets:
  - name: one
    url: http://localhost:80
    password: test
    role: primary
  - name: two
    url: http://localhost:81
    password: test
    role: replica
sync:
  primary: one
  resources: []
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for empty resources")
	}
	if !strings.Contains(err.Error(), "must list at least one resource") {
		t.Errorf("error should mention resources, got: %v", err)
	}
}

func TestValidationSyncInvalidResource(t *testing.T) {
	cfg := `
mode: sync
targets:
  - name: one
    url: http://localhost:80
    password: test
    role: primary
  - name: two
    url: http://localhost:81
    password: test
    role: replica
sync:
  primary: one
  resources: [invalid_thing]
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid resource")
	}
	if !strings.Contains(err.Error(), "unknown resource") {
		t.Errorf("error should mention unknown resource, got: %v", err)
	}
}

func TestValidationSyncModeValid(t *testing.T) {
	cfg := `
mode: sync
targets:
  - name: primary
    url: http://localhost:80
    password: test
    role: primary
  - name: replica
    url: http://localhost:81
    password: test
    role: replica
sync:
  primary: primary
  resources: [adlists, deny, allow]
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err != nil {
		t.Fatalf("valid sync config should not error: %v", err)
	}
}

func TestValidationEmptyGroupName(t *testing.T) {
	cfg := `
targets:
  - name: test
    url: http://localhost:80
    password: test
groups:
  - name: ""
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for empty group name")
	}
}

func TestValidationEmptyAdlistURL(t *testing.T) {
	cfg := `
targets:
  - name: test
    url: http://localhost:80
    password: test
adlists:
  - url: ""
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for empty adlist URL")
	}
}

func TestValidationEmptyDenyDomain(t *testing.T) {
	cfg := `
targets:
  - name: test
    url: http://localhost:80
    password: test
deny:
  - domain: ""
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for empty deny domain")
	}
}

func TestValidationInvalidAllowDomain(t *testing.T) {
	cfg := `
targets:
  - name: test
    url: http://localhost:80
    password: test
allow:
  - domain: "not a domain!"
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid allow domain")
	}
}

func TestValidationDuplicateAllowDomain(t *testing.T) {
	cfg := `
targets:
  - name: test
    url: http://localhost:80
    password: test
allow:
  - domain: safe.example.com
  - domain: safe.example.com
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for duplicate allow domain")
	}
	if !strings.Contains(err.Error(), "duplicate domain") {
		t.Errorf("error should mention duplicate, got: %v", err)
	}
}

func TestValidationLocalDNSInvalidIP(t *testing.T) {
	cfg := `
targets:
  - name: test
    url: http://localhost:80
    password: test
local_dns:
  - domain: nas.lan
    ip: not-an-ip
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid IP")
	}
	if !strings.Contains(err.Error(), "invalid IP") {
		t.Errorf("error should mention invalid IP, got: %v", err)
	}
}

func TestValidationLocalDNSInvalidDomain(t *testing.T) {
	cfg := `
targets:
  - name: test
    url: http://localhost:80
    password: test
local_dns:
  - domain: "not a domain!"
    ip: 192.168.1.1
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid local DNS domain")
	}
}

func TestValidationClientEmptyMatch(t *testing.T) {
	cfg := `
targets:
  - name: test
    url: http://localhost:80
    password: test
clients:
  - match: ""
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for empty client match")
	}
	if !strings.Contains(err.Error(), "match is required") {
		t.Errorf("error should mention match, got: %v", err)
	}
}

func TestValidationClientUndefinedGroup(t *testing.T) {
	cfg := `
targets:
  - name: test
    url: http://localhost:80
    password: test
clients:
  - match: 192.168.1.1
    groups: [nonexistent]
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for undefined group in client")
	}
	if !strings.Contains(err.Error(), "undefined group") {
		t.Errorf("error should mention undefined group, got: %v", err)
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	path := writeTestConfig(t, "{{{{invalid yaml")
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestDefaultSyncInterval(t *testing.T) {
	cfg := `
mode: sync
targets:
  - name: primary
    url: http://localhost:80
    password: test
    role: primary
  - name: replica
    url: http://localhost:81
    password: test
    role: replica
sync:
  primary: primary
  resources: [adlists]
`
	path := writeTestConfig(t, cfg)
	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if c.Sync.Interval.Duration != 5*time.Minute {
		t.Errorf("default sync interval = %v, want 5m", c.Sync.Interval.Duration)
	}
}

func TestClientWithPlainIP(t *testing.T) {
	cfg := `
targets:
  - name: test
    url: http://localhost:80
    password: test
clients:
  - match: 192.168.1.100
    groups: []
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err != nil {
		t.Fatalf("plain IP should be valid, got: %v", err)
	}
}

func TestLocalDNSPurgeDefault(t *testing.T) {
	cfg := `
targets:
  - name: test
    url: http://localhost:80
    password: test
`
	path := writeTestConfig(t, cfg)
	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if c.Reconcile.LocalDNSPurge {
		t.Error("LocalDNSPurge should default to false")
	}
}

func TestLocalDNSPurgeExplicitTrue(t *testing.T) {
	cfg := `
targets:
  - name: test
    url: http://localhost:80
    password: test
reconcile:
  local_dns_purge: true
`
	path := writeTestConfig(t, cfg)
	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if !c.Reconcile.LocalDNSPurge {
		t.Error("LocalDNSPurge should be true when explicitly set")
	}
}

func TestNegativeReconcileInterval(t *testing.T) {
	cfg := `
targets:
  - name: test
    url: http://localhost:80
    password: test
reconcile:
  interval: -1m
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for negative interval")
	}
	if !strings.Contains(err.Error(), "reconcile.interval must be at least 10s") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSubMinimumReconcileInterval(t *testing.T) {
	cfg := `
targets:
  - name: test
    url: http://localhost:80
    password: test
reconcile:
  interval: 1s
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for sub-minimum interval")
	}
	if !strings.Contains(err.Error(), "reconcile.interval must be at least 10s") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSubMinimumScrapeInterval(t *testing.T) {
	cfg := `
targets:
  - name: test
    url: http://localhost:80
    password: test
metrics:
  scrape_interval: 2s
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for sub-minimum scrape interval")
	}
	if !strings.Contains(err.Error(), "scrape_interval must be at least 5s") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidIntervalsPass(t *testing.T) {
	cfg := `
targets:
  - name: test
    url: http://localhost:80
    password: test
reconcile:
  interval: 30s
metrics:
  scrape_interval: 10s
`
	path := writeTestConfig(t, cfg)
	_, err := Load(path)
	if err != nil {
		t.Fatalf("valid intervals should pass: %v", err)
	}
}

func TestDefaultIntervalPassValidation(t *testing.T) {
	cfg := `
targets:
  - name: test
    url: http://localhost:80
    password: test
`
	path := writeTestConfig(t, cfg)
	c, err := Load(path)
	if err != nil {
		t.Fatalf("default intervals should pass: %v", err)
	}
	if c.Reconcile.Interval.Duration != 0 {
		if c.Reconcile.Interval.Duration < MinReconcileInterval {
			t.Errorf("default reconcile interval %v below minimum", c.Reconcile.Interval.Duration)
		}
	}
	if c.Metrics.ScrapeInterval.Duration < MinScrapeInterval {
		t.Errorf("default scrape interval %v below minimum", c.Metrics.ScrapeInterval.Duration)
	}
}
