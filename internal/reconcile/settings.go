package reconcile

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/st0o0/forseti/internal/config"
)

type SettingsAPI interface {
	GetConfig() (map[string]any, error)
	PatchConfig(path string, value any) error
}

type SettingMapping struct {
	ForsetiPath string
	PiholePath  string
	Value       any
}

type SettingsDiff struct {
	Changes []SettingChange
}

type SettingChange struct {
	Name    string
	Path    string
	Current any
	Desired any
}

func (d SettingsDiff) HasChanges() bool {
	return len(d.Changes) > 0
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func BuildDesiredSettingsList(s *config.Settings) []SettingMapping {
	var mappings []SettingMapping

	if len(s.DNS.Upstream) > 0 {
		mappings = append(mappings, SettingMapping{
			ForsetiPath: "dns.upstream",
			PiholePath:  "dns/upstreams",
			Value:       s.DNS.Upstream,
		})
	}
	if s.DNS.Cache.Size != nil {
		mappings = append(mappings, SettingMapping{
			ForsetiPath: "dns.cache.size",
			PiholePath:  "dns/cache/size",
			Value:       *s.DNS.Cache.Size,
		})
	}
	if s.DNS.Cache.ForceOnDisk != nil {
		mappings = append(mappings, SettingMapping{
			ForsetiPath: "dns.cache.force_on_disk",
			PiholePath:  "dns/cache/optimizer",
			Value:       boolToInt(*s.DNS.Cache.ForceOnDisk),
		})
	}
	if s.DNS.RateLimit.Count != nil {
		mappings = append(mappings, SettingMapping{
			ForsetiPath: "dns.rate_limit.count",
			PiholePath:  "dns/rateLimit/count",
			Value:       *s.DNS.RateLimit.Count,
		})
	}
	if s.DNS.RateLimit.Interval != nil {
		mappings = append(mappings, SettingMapping{
			ForsetiPath: "dns.rate_limit.interval",
			PiholePath:  "dns/rateLimit/interval",
			Value:       *s.DNS.RateLimit.Interval,
		})
	}
	if s.Blocking.Mode != "" {
		mappings = append(mappings, SettingMapping{
			ForsetiPath: "blocking.mode",
			PiholePath:  "dns/blocking/mode",
			Value:       s.Blocking.Mode,
		})
	}
	if s.Privacy.Level != nil {
		mappings = append(mappings, SettingMapping{
			ForsetiPath: "privacy.level",
			PiholePath:  "misc/privacylevel",
			Value:       *s.Privacy.Level,
		})
	}

	// DNS — new fields
	if s.DNS.Port != nil {
		mappings = append(mappings, SettingMapping{
			ForsetiPath: "dns.port",
			PiholePath:  "dns/port",
			Value:       *s.DNS.Port,
		})
	}
	if s.DNS.DomainNeeded != nil {
		mappings = append(mappings, SettingMapping{
			ForsetiPath: "dns.domain_needed",
			PiholePath:  "dns/domainNeeded",
			Value:       boolToInt(*s.DNS.DomainNeeded),
		})
	}
	if s.DNS.BogusPriv != nil {
		mappings = append(mappings, SettingMapping{
			ForsetiPath: "dns.bogus_priv",
			PiholePath:  "dns/bogusPriv",
			Value:       boolToInt(*s.DNS.BogusPriv),
		})
	}
	if s.DNS.DNSSEC != nil {
		mappings = append(mappings, SettingMapping{
			ForsetiPath: "dns.dnssec",
			PiholePath:  "dns/dnssec",
			Value:       boolToInt(*s.DNS.DNSSEC),
		})
	}
	if s.DNS.ListeningMode != "" {
		mappings = append(mappings, SettingMapping{
			ForsetiPath: "dns.listening_mode",
			PiholePath:  "dns/listeningMode",
			Value:       s.DNS.ListeningMode,
		})
	}
	if s.DNS.QueryLogging != nil {
		mappings = append(mappings, SettingMapping{
			ForsetiPath: "dns.query_logging",
			PiholePath:  "dns/queryLogging",
			Value:       boolToInt(*s.DNS.QueryLogging),
		})
	}
	if s.DNS.CNAMEDeepInspect != nil {
		mappings = append(mappings, SettingMapping{
			ForsetiPath: "dns.cname_deep_inspect",
			PiholePath:  "dns/cnameDeepInspect",
			Value:       boolToInt(*s.DNS.CNAMEDeepInspect),
		})
	}
	if s.DNS.ResolveIPv4 != nil {
		mappings = append(mappings, SettingMapping{
			ForsetiPath: "dns.resolve_ipv4",
			PiholePath:  "dns/resolveIPv4",
			Value:       boolToInt(*s.DNS.ResolveIPv4),
		})
	}
	if s.DNS.ResolveIPv6 != nil {
		mappings = append(mappings, SettingMapping{
			ForsetiPath: "dns.resolve_ipv6",
			PiholePath:  "dns/resolveIPv6",
			Value:       boolToInt(*s.DNS.ResolveIPv6),
		})
	}

	// RevServer
	if s.DNS.RevServer.Enabled != nil {
		mappings = append(mappings, SettingMapping{
			ForsetiPath: "dns.rev_server.enabled",
			PiholePath:  "dns/revServer/active",
			Value:       boolToInt(*s.DNS.RevServer.Enabled),
		})
	}
	if s.DNS.RevServer.CIDR != "" {
		mappings = append(mappings, SettingMapping{
			ForsetiPath: "dns.rev_server.cidr",
			PiholePath:  "dns/revServer/cidr",
			Value:       s.DNS.RevServer.CIDR,
		})
	}
	if s.DNS.RevServer.Target != "" {
		mappings = append(mappings, SettingMapping{
			ForsetiPath: "dns.rev_server.target",
			PiholePath:  "dns/revServer/target",
			Value:       s.DNS.RevServer.Target,
		})
	}
	if s.DNS.RevServer.Domain != "" {
		mappings = append(mappings, SettingMapping{
			ForsetiPath: "dns.rev_server.domain",
			PiholePath:  "dns/revServer/domain",
			Value:       s.DNS.RevServer.Domain,
		})
	}

	// Blocking — new fields
	if s.Blocking.Active != nil {
		mappings = append(mappings, SettingMapping{
			ForsetiPath: "blocking.active",
			PiholePath:  "dns/blocking/active",
			Value:       boolToInt(*s.Blocking.Active),
		})
	}
	if s.Blocking.Timer != nil {
		mappings = append(mappings, SettingMapping{
			ForsetiPath: "blocking.timer",
			PiholePath:  "dns/blocking/timer",
			Value:       *s.Blocking.Timer,
		})
	}

	// DHCP
	if s.DHCP.Active != nil {
		mappings = append(mappings, SettingMapping{
			ForsetiPath: "dhcp.active",
			PiholePath:  "dhcp/active",
			Value:       boolToInt(*s.DHCP.Active),
		})
	}
	if s.DHCP.Start != "" {
		mappings = append(mappings, SettingMapping{
			ForsetiPath: "dhcp.start",
			PiholePath:  "dhcp/start",
			Value:       s.DHCP.Start,
		})
	}
	if s.DHCP.End != "" {
		mappings = append(mappings, SettingMapping{
			ForsetiPath: "dhcp.end",
			PiholePath:  "dhcp/end",
			Value:       s.DHCP.End,
		})
	}
	if s.DHCP.Router != "" {
		mappings = append(mappings, SettingMapping{
			ForsetiPath: "dhcp.router",
			PiholePath:  "dhcp/router",
			Value:       s.DHCP.Router,
		})
	}
	if s.DHCP.LeaseTime != nil {
		mappings = append(mappings, SettingMapping{
			ForsetiPath: "dhcp.lease_time",
			PiholePath:  "dhcp/leaseTime",
			Value:       *s.DHCP.LeaseTime,
		})
	}
	if s.DHCP.Domain != "" {
		mappings = append(mappings, SettingMapping{
			ForsetiPath: "dhcp.domain",
			PiholePath:  "dhcp/domain",
			Value:       s.DHCP.Domain,
		})
	}
	if s.DHCP.IPv6 != nil {
		mappings = append(mappings, SettingMapping{
			ForsetiPath: "dhcp.ipv6",
			PiholePath:  "dhcp/ipv6",
			Value:       boolToInt(*s.DHCP.IPv6),
		})
	}
	if s.DHCP.RapidCommit != nil {
		mappings = append(mappings, SettingMapping{
			ForsetiPath: "dhcp.rapid_commit",
			PiholePath:  "dhcp/rapidCommit",
			Value:       boolToInt(*s.DHCP.RapidCommit),
		})
	}

	// Webserver
	if s.Webserver.Port != nil {
		mappings = append(mappings, SettingMapping{
			ForsetiPath: "webserver.port",
			PiholePath:  "webserver/port",
			Value:       *s.Webserver.Port,
		})
	}

	// Misc
	if s.Misc.Nice != nil {
		mappings = append(mappings, SettingMapping{
			ForsetiPath: "misc.nice",
			PiholePath:  "misc/nice",
			Value:       *s.Misc.Nice,
		})
	}
	if s.Misc.DelayStartup != nil {
		mappings = append(mappings, SettingMapping{
			ForsetiPath: "misc.delay_startup",
			PiholePath:  "misc/delay_startup",
			Value:       *s.Misc.DelayStartup,
		})
	}
	if s.Misc.Check.Load != nil {
		mappings = append(mappings, SettingMapping{
			ForsetiPath: "misc.check.load",
			PiholePath:  "misc/check/load",
			Value:       boolToInt(*s.Misc.Check.Load),
		})
	}
	if s.Misc.Check.Disk != nil {
		mappings = append(mappings, SettingMapping{
			ForsetiPath: "misc.check.disk",
			PiholePath:  "misc/check/disk",
			Value:       *s.Misc.Check.Disk,
		})
	}
	if s.Misc.Check.Shmem != nil {
		mappings = append(mappings, SettingMapping{
			ForsetiPath: "misc.check.shmem",
			PiholePath:  "misc/check/shmem",
			Value:       *s.Misc.Check.Shmem,
		})
	}

	return mappings
}

func extractConfigValue(cfg map[string]any, path string) (any, bool) {
	parts := splitPath(path)
	current := any(cfg)
	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = m[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func splitPath(path string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(path); i++ {
		if path[i] == '/' {
			if i > start {
				parts = append(parts, path[start:i])
			}
			start = i + 1
		}
	}
	if start < len(path) {
		parts = append(parts, path[start:])
	}
	return parts
}

func normalizeValue(v any) any {
	switch val := v.(type) {
	case bool:
		return boolToInt(val)
	case float64:
		if val == float64(int(val)) {
			return int(val)
		}
		return val
	default:
		return v
	}
}

func settingsNeedUpdate(current, desired any) bool {
	return fmt.Sprintf("%v", normalizeValue(current)) != fmt.Sprintf("%v", normalizeValue(desired))
}

func DiffSettings(settings *config.Settings, api SettingsAPI) (*SettingsDiff, error) {
	if settings.IsEmpty() {
		return &SettingsDiff{}, nil
	}

	actualConfig, err := api.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("get config: %w", err)
	}

	diff := &SettingsDiff{}
	for _, m := range BuildDesiredSettingsList(settings) {
		actual, found := extractConfigValue(actualConfig, m.PiholePath)
		if !found || settingsNeedUpdate(actual, m.Value) {
			diff.Changes = append(diff.Changes, SettingChange{
				Name:    m.ForsetiPath,
				Path:    m.PiholePath,
				Current: actual,
				Desired: m.Value,
			})
		}
	}

	return diff, nil
}

func ApplySettings(target string, settings *config.Settings, api SettingsAPI) (*SettingsDiff, error) {
	diff, err := DiffSettings(settings, api)
	if err != nil {
		return nil, err
	}

	var errs []error
	for _, change := range diff.Changes {
		slog.Info("applying setting", "target", target, "name", change.Name, "current", change.Current, "desired", change.Desired)
		if err := api.PatchConfig(change.Path, change.Desired); err != nil {
			errs = append(errs, fmt.Errorf("set %s: %w", change.Name, err))
		}
	}

	return diff, errors.Join(errs...)
}
