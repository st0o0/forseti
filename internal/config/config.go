package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Mode string

const (
	ModeConfig Mode = "config"
	ModeSync   Mode = "sync"

	MinReconcileInterval = 10 * time.Second
	MinSyncInterval      = 10 * time.Second
	MinScrapeInterval    = 5 * time.Second
)

type Config struct {
	Mode      Mode            `yaml:"mode"`
	LogLevel  string          `yaml:"log_level"`
	LogFormat string          `yaml:"log_format"`
	Metrics   Metrics         `yaml:"metrics"`
	Targets   []Target        `yaml:"targets"`
	Reconcile Reconcile       `yaml:"reconcile"`
	Sync      SyncConfig      `yaml:"sync"`
	Settings  Settings        `yaml:"settings"`
	Groups    []Group         `yaml:"groups"`
	Adlists   []Adlist        `yaml:"adlists"`
	Deny      []DenyEntry     `yaml:"deny"`
	Allow     []AllowEntry    `yaml:"allow"`
	LocalDNS  []LocalDNSEntry `yaml:"local_dns"`
	CNAME     []CNAMEEntry    `yaml:"cname"`
	Clients   []ClientEntry   `yaml:"clients"`
}

type Settings struct {
	DNS       DNSSettings       `yaml:"dns"`
	Blocking  BlockingSettings  `yaml:"blocking"`
	Privacy   PrivacySettings   `yaml:"privacy"`
	DHCP      DHCPSettings      `yaml:"dhcp"`
	Webserver WebserverSettings `yaml:"webserver"`
	Misc      MiscSettings      `yaml:"misc"`
}

func (s Settings) IsEmpty() bool {
	return len(s.DNS.Upstream) == 0 &&
		s.DNS.Cache.Size == nil &&
		s.DNS.Cache.ForceOnDisk == nil &&
		s.DNS.RateLimit.Count == nil &&
		s.DNS.RateLimit.Interval == nil &&
		s.DNS.Port == nil &&
		s.DNS.DomainNeeded == nil &&
		s.DNS.BogusPriv == nil &&
		s.DNS.DNSSEC == nil &&
		s.DNS.ListeningMode == "" &&
		s.DNS.QueryLogging == nil &&
		s.DNS.CNAMEDeepInspect == nil &&
		s.DNS.ResolveIPv4 == nil &&
		s.DNS.ResolveIPv6 == nil &&
		s.DNS.RevServer.Enabled == nil &&
		s.Blocking.Mode == "" &&
		s.Blocking.Active == nil &&
		s.Blocking.Timer == nil &&
		s.Privacy.Level == nil &&
		s.DHCP.Active == nil &&
		s.DHCP.Start == "" &&
		s.DHCP.End == "" &&
		s.DHCP.Router == "" &&
		s.DHCP.LeaseTime == nil &&
		s.DHCP.Domain == "" &&
		s.DHCP.IPv6 == nil &&
		s.DHCP.RapidCommit == nil &&
		s.Webserver.Port == nil &&
		s.Misc.Nice == nil &&
		s.Misc.DelayStartup == nil &&
		s.Misc.Check.Load == nil &&
		s.Misc.Check.Disk == nil &&
		s.Misc.Check.Shmem == nil
}

type DNSSettings struct {
	Upstream         []string          `yaml:"upstream"`
	Cache            CacheSettings     `yaml:"cache"`
	RateLimit        RateLimitSettings `yaml:"rate_limit"`
	Port             *int              `yaml:"port"`
	DomainNeeded     *bool             `yaml:"domain_needed"`
	BogusPriv        *bool             `yaml:"bogus_priv"`
	DNSSEC           *bool             `yaml:"dnssec"`
	ListeningMode    string            `yaml:"listening_mode"`
	QueryLogging     *bool             `yaml:"query_logging"`
	CNAMEDeepInspect *bool             `yaml:"cname_deep_inspect"`
	ResolveIPv4      *bool             `yaml:"resolve_ipv4"`
	ResolveIPv6      *bool             `yaml:"resolve_ipv6"`
	RevServer        RevServerSettings `yaml:"rev_server"`
}

type RevServerSettings struct {
	Enabled *bool  `yaml:"enabled"`
	CIDR    string `yaml:"cidr"`
	Target  string `yaml:"target"`
	Domain  string `yaml:"domain"`
}

type CacheSettings struct {
	Size        *int  `yaml:"size"`
	ForceOnDisk *bool `yaml:"force_on_disk"`
}

type RateLimitSettings struct {
	Count    *int `yaml:"count"`
	Interval *int `yaml:"interval"`
}

type BlockingSettings struct {
	Mode   string `yaml:"mode"`
	Active *bool  `yaml:"active"`
	Timer  *int   `yaml:"timer"`
}

type PrivacySettings struct {
	Level *int `yaml:"level"`
}

type DHCPSettings struct {
	Active      *bool  `yaml:"active"`
	Start       string `yaml:"start"`
	End         string `yaml:"end"`
	Router      string `yaml:"router"`
	LeaseTime   *int   `yaml:"lease_time"`
	Domain      string `yaml:"domain"`
	IPv6        *bool  `yaml:"ipv6"`
	RapidCommit *bool  `yaml:"rapid_commit"`
}

type WebserverSettings struct {
	Port *int `yaml:"port"`
}

type MiscSettings struct {
	Nice         *int          `yaml:"nice"`
	DelayStartup *int          `yaml:"delay_startup"`
	Check        CheckSettings `yaml:"check"`
}

type CheckSettings struct {
	Load *bool `yaml:"load"`
	Disk *int  `yaml:"disk"`
	Shmem *int `yaml:"shmem"`
}

type SyncConfig struct {
	Interval  Duration `yaml:"interval"`
	Primary   string   `yaml:"primary"`
	Resources []string `yaml:"resources"`
}

type Metrics struct {
	Port           int              `yaml:"port"`
	Path           string           `yaml:"path"`
	ScrapeInterval Duration         `yaml:"scrape_interval"`
	Collectors     CollectorToggles `yaml:"collectors"`
}

type CollectorToggles struct {
	Stats         *bool `yaml:"stats"`
	Upstreams     *bool `yaml:"upstreams"`
	QueryTypes    *bool `yaml:"query_types"`
	Blocking      *bool `yaml:"blocking"`
	Reconcile     *bool `yaml:"reconcile"`
	Gravity       *bool `yaml:"gravity"`
	Sessions      *bool `yaml:"sessions"`
	SettingsDrift *bool `yaml:"settings_drift"`
	DHCP          *bool `yaml:"dhcp"`
}

func (ct CollectorToggles) IsEnabled(name string) bool {
	var p *bool
	switch name {
	case "stats":
		p = ct.Stats
	case "upstreams":
		p = ct.Upstreams
	case "query_types":
		p = ct.QueryTypes
	case "blocking":
		p = ct.Blocking
	case "reconcile":
		p = ct.Reconcile
	case "gravity":
		p = ct.Gravity
	case "sessions":
		p = ct.Sessions
	case "settings_drift":
		p = ct.SettingsDrift
	case "dhcp":
		p = ct.DHCP
	default:
		return true
	}
	return p == nil || *p
}

type Target struct {
	Name     string        `yaml:"name"`
	URL      string        `yaml:"url"`
	Password string        `yaml:"password"`
	Role     string        `yaml:"role"`
	Gravity  GravityConfig `yaml:"gravity"`
	File     string        `yaml:"file"`
}

type TargetOverride struct {
	Settings Settings        `yaml:"settings"`
	Groups   []Group         `yaml:"groups"`
	Adlists  []Adlist        `yaml:"adlists"`
	Deny     []DenyEntry     `yaml:"deny"`
	Allow    []AllowEntry    `yaml:"allow"`
	LocalDNS []LocalDNSEntry `yaml:"local_dns"`
	CNAME    []CNAMEEntry    `yaml:"cname"`
	Clients  []ClientEntry   `yaml:"clients"`
	Exclude  ExcludeBlock    `yaml:"exclude"`
}

type ExcludeBlock struct {
	Groups   []GroupExclude   `yaml:"groups"`
	Adlists  []AdlistExclude  `yaml:"adlists"`
	Deny     []DenyExclude    `yaml:"deny"`
	Allow    []AllowExclude   `yaml:"allow"`
	LocalDNS []LocalDNSExclude `yaml:"local_dns"`
	CNAME    []CNAMEExclude   `yaml:"cname"`
	Clients  []ClientExclude  `yaml:"clients"`
}

type GroupExclude struct {
	Name string `yaml:"name"`
}

type AdlistExclude struct {
	URL string `yaml:"url"`
}

type DenyExclude struct {
	Domain string `yaml:"domain"`
}

type AllowExclude struct {
	Domain string `yaml:"domain"`
}

type LocalDNSExclude struct {
	Domain string `yaml:"domain"`
}

type CNAMEExclude struct {
	Domain string `yaml:"domain"`
}

type ClientExclude struct {
	Match string `yaml:"match"`
}

type GravityConfig struct {
	Schedule string `yaml:"schedule"`
}

type Reconcile struct {
	Interval        Duration `yaml:"interval"`
	Marker          string   `yaml:"marker"`
	GravityOnChange *bool    `yaml:"gravity_on_change"`
	LocalDNSPurge   bool     `yaml:"local_dns_purge"`
	CNAMEPurge      bool     `yaml:"cname_purge"`
}

type Group struct {
	Name    string `yaml:"name"`
	Comment string `yaml:"comment"`
}

type Adlist struct {
	URL     string   `yaml:"url"`
	Comment string   `yaml:"comment"`
	Groups  []string `yaml:"groups"`
}

type DenyEntry struct {
	Domain string `yaml:"domain"`
	Kind   string `yaml:"kind"`
}

type AllowEntry struct {
	Domain string `yaml:"domain"`
	Kind   string `yaml:"kind"`
}

type CNAMEEntry struct {
	Domain string `yaml:"domain"`
	Target string `yaml:"target"`
}

type LocalDNSEntry struct {
	Domain string `yaml:"domain"`
	IP     string `yaml:"ip"`
}

type ClientEntry struct {
	Match   string   `yaml:"match"`
	Comment string   `yaml:"comment"`
	Groups  []string `yaml:"groups"`
}

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	d.Duration = parsed
	return nil
}

type ResolvedTarget struct {
	Target
	Settings Settings
	Groups   []Group
	Adlists  []Adlist
	Deny     []DenyEntry
	Allow    []AllowEntry
	LocalDNS []LocalDNSEntry
	CNAME    []CNAMEEntry
	Clients  []ClientEntry
}

func Load(path string) (*Config, []ResolvedTarget, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("reading config: %w", err)
	}

	expanded, err := expandEnv(string(data))
	if err != nil {
		return nil, nil, fmt.Errorf("expanding env vars: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, nil, fmt.Errorf("parsing config: %w", err)
	}

	applyDefaults(&cfg)

	if err := validate(&cfg); err != nil {
		return nil, nil, fmt.Errorf("validating config: %w", err)
	}

	configDir := filepath.Dir(path)
	resolved, err := resolveTargets(&cfg, configDir)
	if err != nil {
		return nil, nil, fmt.Errorf("resolving targets: %w", err)
	}

	return &cfg, resolved, nil
}

var envVarPattern = regexp.MustCompile(`\$\{([^}]+)}`)

func expandEnv(content string) (string, error) {
	var undefined []string
	result := envVarPattern.ReplaceAllStringFunc(content, func(match string) string {
		varName := envVarPattern.FindStringSubmatch(match)[1]
		val, ok := os.LookupEnv(varName)
		if !ok {
			undefined = append(undefined, varName)
			return match
		}
		return val
	})
	if len(undefined) > 0 {
		return "", fmt.Errorf("undefined environment variables: %s", strings.Join(undefined, ", "))
	}
	return result, nil
}

func applyDefaults(cfg *Config) {
	if cfg.Mode == "" {
		cfg.Mode = ModeConfig
	}
	if cfg.Metrics.Port == 0 {
		cfg.Metrics.Port = 9099
	}
	if cfg.Metrics.Path == "" {
		cfg.Metrics.Path = "/metrics"
	}
	if cfg.Metrics.ScrapeInterval.Duration == 0 {
		cfg.Metrics.ScrapeInterval.Duration = 30 * time.Second
	}
	if cfg.Reconcile.Marker == "" {
		cfg.Reconcile.Marker = "[forseti]"
	}
	if cfg.Reconcile.GravityOnChange == nil {
		t := true
		cfg.Reconcile.GravityOnChange = &t
	}
	if cfg.Sync.Interval.Duration == 0 {
		cfg.Sync.Interval.Duration = 5 * time.Minute
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}
	if cfg.LogFormat == "" {
		cfg.LogFormat = "text"
	}
	for i := range cfg.Deny {
		if cfg.Deny[i].Kind == "" {
			cfg.Deny[i].Kind = "exact"
		}
	}
	for i := range cfg.Allow {
		if cfg.Allow[i].Kind == "" {
			cfg.Allow[i].Kind = "exact"
		}
	}
}

var validSyncResources = map[string]bool{
	"adlists": true, "deny": true, "allow": true,
	"local_dns": true, "groups": true, "clients": true,
}

func validate(cfg *Config) error {
	var errs []error

	if cfg.Mode != ModeConfig && cfg.Mode != ModeSync {
		errs = append(errs, fmt.Errorf("mode must be %q or %q, got %q", ModeConfig, ModeSync, cfg.Mode))
	}

	if len(cfg.Targets) == 0 {
		errs = append(errs, errors.New("at least one target is required"))
	}
	for i, t := range cfg.Targets {
		if t.Name == "" {
			errs = append(errs, fmt.Errorf("target[%d]: name is required", i))
		}
		if t.URL == "" {
			errs = append(errs, fmt.Errorf("target[%d]: url is required", i))
		} else {
			u, err := url.Parse(t.URL)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
				errs = append(errs, fmt.Errorf("target[%d]: invalid URL %q (must be http or https)", i, t.URL))
			}
		}
		if t.Password == "" {
			errs = append(errs, fmt.Errorf("target[%d]: password is required", i))
		}
		if t.Gravity.Schedule != "" {
			if fields := strings.Fields(t.Gravity.Schedule); len(fields) != 5 {
				errs = append(errs, fmt.Errorf("target[%d]: gravity schedule must have 5 fields (min hour dom mon dow), got %d", i, len(fields)))
			}
		}
		if cfg.Mode == ModeSync {
			if t.Role == "" {
				errs = append(errs, fmt.Errorf("target[%d]: role is required in sync mode (primary or replica)", i))
			} else if t.Role != "primary" && t.Role != "replica" {
				errs = append(errs, fmt.Errorf("target[%d]: role must be \"primary\" or \"replica\", got %q", i, t.Role))
			}
		}
	}

	if cfg.Mode == ModeSync {
		primaryCount := 0
		for _, t := range cfg.Targets {
			if t.Role == "primary" {
				primaryCount++
			}
		}
		if primaryCount != 1 {
			errs = append(errs, fmt.Errorf("sync mode requires exactly 1 primary target, got %d", primaryCount))
		}
		if cfg.Sync.Primary == "" {
			errs = append(errs, errors.New("sync.primary is required in sync mode"))
		} else {
			found := false
			for _, t := range cfg.Targets {
				if t.Name == cfg.Sync.Primary && t.Role == "primary" {
					found = true
					break
				}
			}
			if !found {
				errs = append(errs, fmt.Errorf("sync.primary %q must reference a target with role: primary", cfg.Sync.Primary))
			}
		}
		if len(cfg.Sync.Resources) == 0 {
			errs = append(errs, errors.New("sync.resources must list at least one resource"))
		}
		for i, r := range cfg.Sync.Resources {
			if !validSyncResources[r] {
				errs = append(errs, fmt.Errorf("sync.resources[%d]: unknown resource %q", i, r))
			}
		}
	}

	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLogLevels[cfg.LogLevel] {
		errs = append(errs, fmt.Errorf("log_level must be debug/info/warn/error, got %q", cfg.LogLevel))
	}
	validLogFormats := map[string]bool{"text": true, "json": true}
	if !validLogFormats[cfg.LogFormat] {
		errs = append(errs, fmt.Errorf("log_format must be text/json, got %q", cfg.LogFormat))
	}

	if settingsErrs := validateSettings(&cfg.Settings); settingsErrs != nil {
		errs = append(errs, settingsErrs)
	}

	if cfg.Reconcile.Interval.Duration > 0 && cfg.Reconcile.Interval.Duration < MinReconcileInterval {
		errs = append(errs, fmt.Errorf("reconcile.interval must be at least %s", MinReconcileInterval))
	} else if cfg.Reconcile.Interval.Duration < 0 {
		errs = append(errs, fmt.Errorf("reconcile.interval must be at least %s", MinReconcileInterval))
	}
	if cfg.Sync.Interval.Duration > 0 && cfg.Sync.Interval.Duration < MinSyncInterval {
		errs = append(errs, fmt.Errorf("sync.interval must be at least %s", MinSyncInterval))
	} else if cfg.Sync.Interval.Duration < 0 {
		errs = append(errs, fmt.Errorf("sync.interval must be at least %s", MinSyncInterval))
	}
	if cfg.Metrics.ScrapeInterval.Duration > 0 && cfg.Metrics.ScrapeInterval.Duration < MinScrapeInterval {
		errs = append(errs, fmt.Errorf("scrape_interval must be at least %s", MinScrapeInterval))
	} else if cfg.Metrics.ScrapeInterval.Duration < 0 {
		errs = append(errs, fmt.Errorf("scrape_interval must be at least %s", MinScrapeInterval))
	}

	groupNames := make(map[string]bool)
	for i, g := range cfg.Groups {
		if g.Name == "" {
			errs = append(errs, fmt.Errorf("group[%d]: name is required", i))
		} else if groupNames[g.Name] {
			errs = append(errs, fmt.Errorf("group[%d]: duplicate group name %q", i, g.Name))
		} else {
			groupNames[g.Name] = true
		}
	}

	adlistURLs := make(map[string]bool)
	for i, a := range cfg.Adlists {
		if a.URL == "" {
			errs = append(errs, fmt.Errorf("adlist[%d]: url is required", i))
		} else if adlistURLs[a.URL] {
			errs = append(errs, fmt.Errorf("adlist[%d]: duplicate adlist URL %q", i, a.URL))
		} else {
			adlistURLs[a.URL] = true
		}
		for _, g := range a.Groups {
			if !groupNames[g] {
				errs = append(errs, fmt.Errorf("adlist[%d]: references undefined group %q", i, g))
			}
		}
	}

	denyDomains := make(map[string]bool)
	for i, d := range cfg.Deny {
		if d.Kind != "exact" && d.Kind != "regex" {
			errs = append(errs, fmt.Errorf("deny[%d]: kind must be \"exact\" or \"regex\", got %q", i, d.Kind))
		} else if d.Kind == "regex" {
			if d.Domain == "" {
				errs = append(errs, fmt.Errorf("deny[%d]: domain is required", i))
			} else if _, err := regexp.Compile(d.Domain); err != nil {
				errs = append(errs, fmt.Errorf("deny[%d]: invalid regex %q: %v", i, d.Domain, err))
			}
		} else {
			if err := validateDomain(d.Domain); err != nil {
				errs = append(errs, fmt.Errorf("deny[%d]: %w", i, err))
			}
		}
		dedupKey := d.Kind + ":" + d.Domain
		if denyDomains[dedupKey] {
			errs = append(errs, fmt.Errorf("deny[%d]: duplicate domain %q", i, d.Domain))
		} else {
			denyDomains[dedupKey] = true
		}
	}

	allowDomains := make(map[string]bool)
	for i, a := range cfg.Allow {
		if a.Kind != "exact" && a.Kind != "regex" {
			errs = append(errs, fmt.Errorf("allow[%d]: kind must be \"exact\" or \"regex\", got %q", i, a.Kind))
		} else if a.Kind == "regex" {
			if a.Domain == "" {
				errs = append(errs, fmt.Errorf("allow[%d]: domain is required", i))
			} else if _, err := regexp.Compile(a.Domain); err != nil {
				errs = append(errs, fmt.Errorf("allow[%d]: invalid regex %q: %v", i, a.Domain, err))
			}
		} else {
			if err := validateDomain(a.Domain); err != nil {
				errs = append(errs, fmt.Errorf("allow[%d]: %w", i, err))
			}
		}
		dedupKey := a.Kind + ":" + a.Domain
		if allowDomains[dedupKey] {
			errs = append(errs, fmt.Errorf("allow[%d]: duplicate domain %q", i, a.Domain))
		} else {
			allowDomains[dedupKey] = true
		}
	}

	for i, dns := range cfg.LocalDNS {
		if err := validateDomain(dns.Domain); err != nil {
			errs = append(errs, fmt.Errorf("local_dns[%d]: %w", i, err))
		}
		if net.ParseIP(dns.IP) == nil {
			errs = append(errs, fmt.Errorf("local_dns[%d]: invalid IP %q", i, dns.IP))
		}
	}

	for i, cn := range cfg.CNAME {
		if err := validateDomain(cn.Domain); err != nil {
			errs = append(errs, fmt.Errorf("cname[%d]: %w", i, err))
		}
		if cn.Target == "" {
			errs = append(errs, fmt.Errorf("cname[%d]: target is required", i))
		} else if err := validateDomain(cn.Target); err != nil {
			errs = append(errs, fmt.Errorf("cname[%d]: target: %w", i, err))
		}
	}

	for i, c := range cfg.Clients {
		if c.Match == "" {
			errs = append(errs, fmt.Errorf("client[%d]: match is required", i))
		} else if ip := net.ParseIP(c.Match); ip == nil {
			if _, _, err := net.ParseCIDR(c.Match); err != nil {
				errs = append(errs, fmt.Errorf("client[%d]: invalid IP or CIDR %q", i, c.Match))
			}
		}
		for _, g := range c.Groups {
			if !groupNames[g] {
				errs = append(errs, fmt.Errorf("client[%d]: references undefined group %q", i, g))
			}
		}
	}

	return errors.Join(errs...)
}

var (
	validBlockingModes   = map[string]bool{"NULL": true, "IP": true, "NXDOMAIN": true}
	validListeningModes  = map[string]bool{"local": true, "all": true, "bind": true}
)

func validateSettings(s *Settings) error {
	var errs []error

	// DNS
	if s.DNS.Cache.Size != nil && *s.DNS.Cache.Size < 0 {
		errs = append(errs, fmt.Errorf("settings.dns.cache.size must be non-negative, got %d", *s.DNS.Cache.Size))
	}
	if s.DNS.RateLimit.Count != nil && *s.DNS.RateLimit.Count < 0 {
		errs = append(errs, fmt.Errorf("settings.dns.rate_limit.count must be non-negative, got %d", *s.DNS.RateLimit.Count))
	}
	if s.DNS.RateLimit.Interval != nil && *s.DNS.RateLimit.Interval < 0 {
		errs = append(errs, fmt.Errorf("settings.dns.rate_limit.interval must be non-negative, got %d", *s.DNS.RateLimit.Interval))
	}
	if s.DNS.Port != nil && (*s.DNS.Port < 1 || *s.DNS.Port > 65535) {
		errs = append(errs, fmt.Errorf("settings.dns.port must be 1-65535, got %d", *s.DNS.Port))
	}
	if s.DNS.ListeningMode != "" && !validListeningModes[s.DNS.ListeningMode] {
		errs = append(errs, fmt.Errorf("settings.dns.listening_mode must be local/all/bind, got %q", s.DNS.ListeningMode))
	}

	// Blocking
	if s.Blocking.Mode != "" && !validBlockingModes[s.Blocking.Mode] {
		errs = append(errs, fmt.Errorf("settings.blocking.mode must be NULL/IP/NXDOMAIN, got %q", s.Blocking.Mode))
	}
	if s.Blocking.Timer != nil && *s.Blocking.Timer < 0 {
		errs = append(errs, fmt.Errorf("settings.blocking.timer must be non-negative, got %d", *s.Blocking.Timer))
	}

	// Privacy
	if s.Privacy.Level != nil && (*s.Privacy.Level < 0 || *s.Privacy.Level > 4) {
		errs = append(errs, fmt.Errorf("settings.privacy.level must be 0-4, got %d", *s.Privacy.Level))
	}

	// DHCP
	dhcpPartial := s.DHCP.Start != "" || s.DHCP.End != "" || s.DHCP.Router != ""
	if dhcpPartial && (s.DHCP.Start == "" || s.DHCP.End == "" || s.DHCP.Router == "") {
		errs = append(errs, fmt.Errorf("settings.dhcp: start, end, and router must all be set together"))
	}
	if s.DHCP.Start != "" && net.ParseIP(s.DHCP.Start) == nil {
		errs = append(errs, fmt.Errorf("settings.dhcp.start: invalid IP %q", s.DHCP.Start))
	}
	if s.DHCP.End != "" && net.ParseIP(s.DHCP.End) == nil {
		errs = append(errs, fmt.Errorf("settings.dhcp.end: invalid IP %q", s.DHCP.End))
	}
	if s.DHCP.Router != "" && net.ParseIP(s.DHCP.Router) == nil {
		errs = append(errs, fmt.Errorf("settings.dhcp.router: invalid IP %q", s.DHCP.Router))
	}
	if s.DHCP.LeaseTime != nil && *s.DHCP.LeaseTime < 0 {
		errs = append(errs, fmt.Errorf("settings.dhcp.lease_time must be non-negative, got %d", *s.DHCP.LeaseTime))
	}

	// Webserver
	if s.Webserver.Port != nil && (*s.Webserver.Port < 1 || *s.Webserver.Port > 65535) {
		errs = append(errs, fmt.Errorf("settings.webserver.port must be 1-65535, got %d", *s.Webserver.Port))
	}

	// Misc
	if s.Misc.DelayStartup != nil && *s.Misc.DelayStartup < 0 {
		errs = append(errs, fmt.Errorf("settings.misc.delay_startup must be non-negative, got %d", *s.Misc.DelayStartup))
	}
	if s.Misc.Check.Disk != nil && (*s.Misc.Check.Disk < 0 || *s.Misc.Check.Disk > 100) {
		errs = append(errs, fmt.Errorf("settings.misc.check.disk must be 0-100, got %d", *s.Misc.Check.Disk))
	}
	if s.Misc.Check.Shmem != nil && (*s.Misc.Check.Shmem < 0 || *s.Misc.Check.Shmem > 100) {
		errs = append(errs, fmt.Errorf("settings.misc.check.shmem must be 0-100, got %d", *s.Misc.Check.Shmem))
	}

	return errors.Join(errs...)
}

func validateEffective(rt *ResolvedTarget) error {
	var errs []error

	if settingsErrs := validateSettings(&rt.Settings); settingsErrs != nil {
		errs = append(errs, settingsErrs)
	}

	groupNames := make(map[string]bool)
	for i, g := range rt.Groups {
		if g.Name == "" {
			errs = append(errs, fmt.Errorf("group[%d]: name is required", i))
		} else if groupNames[g.Name] {
			errs = append(errs, fmt.Errorf("group[%d]: duplicate group name %q", i, g.Name))
		} else {
			groupNames[g.Name] = true
		}
	}

	adlistURLs := make(map[string]bool)
	for i, a := range rt.Adlists {
		if a.URL == "" {
			errs = append(errs, fmt.Errorf("adlist[%d]: url is required", i))
		} else if adlistURLs[a.URL] {
			errs = append(errs, fmt.Errorf("adlist[%d]: duplicate adlist URL %q", i, a.URL))
		} else {
			adlistURLs[a.URL] = true
		}
		for _, g := range a.Groups {
			if !groupNames[g] {
				errs = append(errs, fmt.Errorf("adlist[%d]: references undefined group %q", i, g))
			}
		}
	}

	denyDomains := make(map[string]bool)
	for i, d := range rt.Deny {
		dedupKey := d.Kind + ":" + d.Domain
		if denyDomains[dedupKey] {
			errs = append(errs, fmt.Errorf("deny[%d]: duplicate domain %q", i, d.Domain))
		} else {
			denyDomains[dedupKey] = true
		}
	}

	allowDomains := make(map[string]bool)
	for i, a := range rt.Allow {
		dedupKey := a.Kind + ":" + a.Domain
		if allowDomains[dedupKey] {
			errs = append(errs, fmt.Errorf("allow[%d]: duplicate domain %q", i, a.Domain))
		} else {
			allowDomains[dedupKey] = true
		}
	}

	for i, c := range rt.Clients {
		for _, g := range c.Groups {
			if !groupNames[g] {
				errs = append(errs, fmt.Errorf("client[%d]: references undefined group %q", i, g))
			}
		}
	}

	return errors.Join(errs...)
}

var domainPattern = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)

func validateDomain(domain string) error {
	if domain == "" {
		return errors.New("domain is required")
	}
	if !domainPattern.MatchString(domain) {
		return fmt.Errorf("invalid domain %q", domain)
	}
	return nil
}
