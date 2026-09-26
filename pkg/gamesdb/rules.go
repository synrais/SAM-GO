package gamesdb

import (
	"path/filepath"
	"strings"

	"github.com/synrais/SAMenu/pkg/config"
	"github.com/synrais/SAMenu/pkg/games"
)

// RuleSet holds the Folders/Files/Extensions/Paths rules from SAMenu.ini
// ([Database.X] or [Attract.X]), sorted out by the system they apply to.
type RuleSet struct {
	bySystem map[string]config.Rules // lowercase system ID -> combined rules
}

// NewRuleSet works out which systems each section applies to. Section names
// can be ALL, a system ID or a group (category or manufacturer).
func NewRuleSet(sections map[string]config.Rules) RuleSet {
	rs := RuleSet{bySystem: make(map[string]config.Rules)}
	for name, rules := range sections {
		var ids map[string]bool
		if strings.EqualFold(name, "all") {
			ids = make(map[string]bool)
			for _, s := range games.Systems {
				ids[strings.ToLower(s.Id)] = true
			}
		} else {
			ids, _ = games.ResolveSystems([]string{name})
		}
		for id := range ids {
			combined := rs.bySystem[id]
			combined.Add(rules)
			rs.bySystem[id] = combined
		}
	}
	return rs
}

// Excludes reports whether a game is ruled out.
func (rs RuleSet) Excludes(f FileInfo) bool {
	rules, ok := rs.bySystem[strings.ToLower(f.SystemId)]
	if !ok {
		return false
	}

	for _, ext := range rules.Extensions {
		if e := strings.TrimPrefix(strings.TrimSpace(ext), "."); e != "" && strings.EqualFold(e, f.Ext) {
			return true
		}
	}

	// Files match the name with or without its extension.
	full := f.FileName()
	for _, pattern := range rules.Files {
		if matchPattern(pattern, f.Name) || matchPattern(pattern, full) {
			return true
		}
	}

	// Folders match the folder names inside the system, as the menu shows
	// them (the MenuPath minus the system name and the file).
	if len(rules.Folders) > 0 {
		parts := strings.Split(f.MenuPath, "/")
		if len(parts) > 2 {
			for _, folder := range parts[1 : len(parts)-1] {
				for _, pattern := range rules.Folders {
					if matchPattern(pattern, folder) {
						return true
					}
				}
			}
		}
	}

	for _, p := range rules.Paths {
		if p = strings.TrimSpace(p); p == "" {
			continue
		}
		p = strings.ToLower(filepath.Clean(p))
		path := strings.ToLower(f.Path)
		if path == p || strings.HasPrefix(path, p+"/") {
			return true
		}
	}

	return false
}

// matchPattern matches a SAMenu.ini pattern against a name, ignoring case:
// "hack" exact, "hack*" starts with, "*hack" ends with, "*hack*" contains.
func matchPattern(pattern, name string) bool {
	p := strings.ToLower(strings.TrimSpace(pattern))
	n := strings.ToLower(name)
	star1, star2 := strings.HasPrefix(p, "*"), strings.HasSuffix(p, "*")
	p = strings.Trim(p, "*")
	if p == "" {
		return false
	}
	switch {
	case star1 && star2:
		return strings.Contains(n, p)
	case star2:
		return strings.HasPrefix(n, p)
	case star1:
		return strings.HasSuffix(n, p)
	default:
		return n == p
	}
}
