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
