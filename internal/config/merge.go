package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func resolveTargets(cfg *Config, configDir string) ([]ResolvedTarget, error) {
	resolved := make([]ResolvedTarget, 0, len(cfg.Targets))
	for _, t := range cfg.Targets {
		rt := ResolvedTarget{
			Target:   t,
			Settings: cfg.Settings,
			Groups:   cloneSlice(cfg.Groups),
			Adlists:  cloneSlice(cfg.Adlists),
			Deny:     cloneSlice(cfg.Deny),
			Allow:    cloneSlice(cfg.Allow),
			LocalDNS: cloneSlice(cfg.LocalDNS),
			CNAME:    cloneSlice(cfg.CNAME),
			Clients:  cloneSlice(cfg.Clients),
		}

		if t.File != "" {
			override, err := loadOverride(t.File, configDir)
			if err != nil {
				return nil, fmt.Errorf("target %q: %w", t.Name, err)
			}
			mergeOverride(&rt, override)
		}

		if err := validateEffective(&rt); err != nil {
			return nil, fmt.Errorf("target %q: %w", t.Name, err)
		}

		resolved = append(resolved, rt)
	}
	return resolved, nil
}

func loadOverride(file, configDir string) (*TargetOverride, error) {
	path := file
	if !filepath.IsAbs(path) {
		path = filepath.Join(configDir, path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading override file %q: %w", path, err)
	}

	expanded, err := expandEnv(string(data))
	if err != nil {
		return nil, fmt.Errorf("expanding env vars in %q: %w", path, err)
	}

	var raw map[string]any
	if err := yaml.Unmarshal([]byte(expanded), &raw); err != nil {
		return nil, fmt.Errorf("parsing override file %q: %w", path, err)
	}
	for key := range raw {
		switch key {
		case "settings", "groups", "adlists", "deny", "allow", "local_dns", "cname", "clients", "exclude":
		default:
			return nil, fmt.Errorf("override file %q: field %q is not allowed (only settings, content sections, and exclude)", path, key)
		}
	}

	var override TargetOverride
	if err := yaml.Unmarshal([]byte(expanded), &override); err != nil {
		return nil, fmt.Errorf("parsing override file %q: %w", path, err)
	}

	return &override, nil
}

func mergeOverride(rt *ResolvedTarget, o *TargetOverride) {
	rt.Settings = mergeSettings(rt.Settings, o.Settings)

	rt.Groups = append(rt.Groups, o.Groups...)
	rt.Adlists = append(rt.Adlists, o.Adlists...)

	for i := range o.Deny {
		if o.Deny[i].Kind == "" {
			o.Deny[i].Kind = "exact"
		}
	}
	rt.Deny = append(rt.Deny, o.Deny...)

	for i := range o.Allow {
		if o.Allow[i].Kind == "" {
			o.Allow[i].Kind = "exact"
		}
	}
	rt.Allow = append(rt.Allow, o.Allow...)

	rt.LocalDNS = append(rt.LocalDNS, o.LocalDNS...)
	rt.CNAME = append(rt.CNAME, o.CNAME...)
	rt.Clients = append(rt.Clients, o.Clients...)

	applyExcludes(rt, &o.Exclude)
}

func mergeSettings(base, override Settings) Settings {
	merged := base

	if len(override.DNS.Upstream) > 0 {
		merged.DNS.Upstream = override.DNS.Upstream
	}
	if override.DNS.Cache.Size != nil {
		merged.DNS.Cache.Size = override.DNS.Cache.Size
	}
	if override.DNS.Cache.ForceOnDisk != nil {
		merged.DNS.Cache.ForceOnDisk = override.DNS.Cache.ForceOnDisk
	}
	if override.DNS.RateLimit.Count != nil {
		merged.DNS.RateLimit.Count = override.DNS.RateLimit.Count
	}
	if override.DNS.RateLimit.Interval != nil {
		merged.DNS.RateLimit.Interval = override.DNS.RateLimit.Interval
	}
	if override.Blocking.Mode != "" {
		merged.Blocking.Mode = override.Blocking.Mode
	}
	if override.Privacy.Level != nil {
		merged.Privacy.Level = override.Privacy.Level
	}

	return merged
}

func applyExcludes(rt *ResolvedTarget, ex *ExcludeBlock) {
	for _, e := range ex.Groups {
		found := false
		rt.Groups = removeFrom(rt.Groups, func(g Group) bool {
			if g.Name == e.Name {
				found = true
				return true
			}
			return false
		})
		if !found {
			slog.Warn("exclude: group not found in effective config", "name", e.Name, "target", rt.Name)
		}
	}

	for _, e := range ex.Adlists {
		found := false
		rt.Adlists = removeFrom(rt.Adlists, func(a Adlist) bool {
			if a.URL == e.URL {
				found = true
				return true
			}
			return false
		})
		if !found {
			slog.Warn("exclude: adlist not found in effective config", "url", e.URL, "target", rt.Name)
		}
	}

	for _, e := range ex.Deny {
		found := false
		rt.Deny = removeFrom(rt.Deny, func(d DenyEntry) bool {
			if d.Domain == e.Domain {
				found = true
				return true
			}
			return false
		})
		if !found {
			slog.Warn("exclude: deny entry not found in effective config", "domain", e.Domain, "target", rt.Name)
		}
	}

	for _, e := range ex.Allow {
		found := false
		rt.Allow = removeFrom(rt.Allow, func(a AllowEntry) bool {
			if a.Domain == e.Domain {
				found = true
				return true
			}
			return false
		})
		if !found {
			slog.Warn("exclude: allow entry not found in effective config", "domain", e.Domain, "target", rt.Name)
		}
	}

	for _, e := range ex.LocalDNS {
		found := false
		rt.LocalDNS = removeFrom(rt.LocalDNS, func(d LocalDNSEntry) bool {
			if d.Domain == e.Domain {
				found = true
				return true
			}
			return false
		})
		if !found {
			slog.Warn("exclude: local_dns entry not found in effective config", "domain", e.Domain, "target", rt.Name)
		}
	}

	for _, e := range ex.CNAME {
		found := false
		rt.CNAME = removeFrom(rt.CNAME, func(c CNAMEEntry) bool {
			if c.Domain == e.Domain {
				found = true
				return true
			}
			return false
		})
		if !found {
			slog.Warn("exclude: cname entry not found in effective config", "domain", e.Domain, "target", rt.Name)
		}
	}

	for _, e := range ex.Clients {
		found := false
		rt.Clients = removeFrom(rt.Clients, func(c ClientEntry) bool {
			if c.Match == e.Match {
				found = true
				return true
			}
			return false
		})
		if !found {
			slog.Warn("exclude: client not found in effective config", "match", e.Match, "target", rt.Name)
		}
	}
}

func removeFrom[T any](slice []T, match func(T) bool) []T {
	result := make([]T, 0, len(slice))
	for _, item := range slice {
		if !match(item) {
			result = append(result, item)
		}
	}
	return result
}

func cloneSlice[T any](s []T) []T {
	if s == nil {
		return nil
	}
	c := make([]T, len(s))
	copy(c, s)
	return c
}
