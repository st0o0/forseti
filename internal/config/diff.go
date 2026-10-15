package config

import (
	"reflect"
	"sort"
	"strings"
)

// ReloadDiff summarizes what changed between two configs on reload.
type ReloadDiff struct {
	// Changed lists top-level config sections (by YAML key) that differ.
	Changed []string
	// Added and Removed list target names.
	Added   []string
	Removed []string
}

// Diff compares two configs section by section. Passwords take part in the
// comparison but are never part of the result.
func Diff(old, cur *Config) ReloadDiff {
	var d ReloadDiff
	ov, nv := reflect.ValueOf(*old), reflect.ValueOf(*cur)
	t := ov.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		key := strings.Split(f.Tag.Get("yaml"), ",")[0]
		if key == "" || key == "-" {
			continue
		}
		if !reflect.DeepEqual(ov.Field(i).Interface(), nv.Field(i).Interface()) {
			d.Changed = append(d.Changed, key)
		}
	}

	oldNames := make(map[string]bool, len(old.Targets))
	for _, tg := range old.Targets {
		oldNames[tg.Name] = true
	}
	newNames := make(map[string]bool, len(cur.Targets))
	for _, tg := range cur.Targets {
		newNames[tg.Name] = true
		if !oldNames[tg.Name] {
			d.Added = append(d.Added, tg.Name)
		}
	}
	for name := range oldNames {
		if !newNames[name] {
			d.Removed = append(d.Removed, name)
		}
	}
	sort.Strings(d.Added)
	sort.Strings(d.Removed)
	return d
}
