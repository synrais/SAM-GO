package attract

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/synrais/SAM-GO/pkg/cache"
	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/utils"
)

// -----------------------------
// Case-insensitive helpers
// -----------------------------

func ContainsInsensitive(list []string, item string) bool {
	for _, v := range list {
		if strings.EqualFold(strings.TrimSpace(v), item) {
			return true
		}
	}
	return false
}

func MatchesSystem(list []string, system string) bool {
	return ContainsInsensitive(list, system)
}

func AllowedFor(system string, include, exclude []string) bool {
	if len(include) > 0 && !MatchesSystem(include, system) {
		return false
	}
	if MatchesSystem(exclude, system) {
		return false
	}
	return true
}

// -----------------------------
// Filterlist readers
// -----------------------------

func ReadNameSet(path string) map[string]struct{} {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	set := make(map[string]struct{})
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		name, _ := utils.NormalizeEntry(line)
		set[name] = struct{}{}
	}
	return set
}

func ReadStaticMap(path string) map[string]string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	m := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}
		ts := strings.Trim(parts[0], "<>")
		name, _ := utils.NormalizeEntry(parts[1])
		m[name] = ts
	}
	return m
}

// -----------------------------
// Line helpers
// -----------------------------

func ParseLine(line string) (float64, string) {
	if strings.HasPrefix(line, "<") {
		if idx := strings.Index(line, ">"); idx > 1 {
			tsStr := line[1:idx]
			rest := line[idx+1:]
			ts, _ := strconv.ParseFloat(tsStr, 64)
			return ts, rest
		}
	}
	return 0, line
}

func ReadStaticTimestamp(system, game string) float64 {
	filterBase := config.FilterlistDir()
	path := filepath.Join(filterBase, system+"_staticlist.txt")

	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}
		tsStr := strings.Trim(parts[0], "<>")
		name, _ := utils.NormalizeEntry(parts[1])
		if name == game {
			ts, _ := strconv.ParseFloat(tsStr, 64)
			return ts
		}
	}
	return 0
}

// -----------------------------
// Matching helper
// -----------------------------

func matchRule(rule, candidate string) bool {
	rule = strings.ToLower(strings.TrimSpace(rule))
	candidate = strings.ToLower(strings.TrimSpace(candidate))

	if rule == "" || candidate == "" {
		return false
	}

	if strings.HasPrefix(rule, "*") && strings.HasSuffix(rule, "*") && len(rule) > 2 {
		sub := strings.Trim(rule, "*")
		return strings.Contains(candidate, sub)
	}
	if strings.HasSuffix(rule, "*") {
		prefix := strings.TrimSuffix(rule, "*")
		return strings.HasPrefix(candidate, prefix)
	}
	if strings.HasPrefix(rule, "*") {
		suffix := strings.TrimPrefix(rule, "*")
		return strings.HasSuffix(candidate, suffix)
	}
	if !strings.Contains(rule, ".") {
		candidate = strings.TrimSuffix(candidate, filepath.Ext(candidate))
	}
	return candidate == rule
}

// -----------------------------
// Filters
// -----------------------------

func FilterUniqueWithMGL(files []string) []string {
	chosen := make(map[string]string)
	for _, f := range files {
		base := strings.TrimSuffix(strings.ToLower(filepath.Base(f)), filepath.Ext(f))
		ext := strings.ToLower(filepath.Ext(f))
		if prev, ok := chosen[base]; ok {
			if strings.HasSuffix(prev, ".mgl") {
				continue
			}
			if ext == ".mgl" {
				chosen[base] = f
			}
		} else {
			chosen[base] = f
		}
	}
	result := []string{}
	for _, v := range chosen {
		result = append(result, v)
	}
	return result
}

func FilterFoldersAndFiles(files []string, systemID string, cfg *config.UserConfig) []string {
	var folders, patterns []string

	if global, ok := cfg.Disable["all"]; ok {
		folders = append(folders, global.Folders...)
		patterns = append(patterns, global.Files...)
	}
	if sys, ok := cfg.Disable[strings.ToLower(systemID)]; ok {
		folders = append(folders, sys.Folders...)
		patterns = append(patterns, sys.Files...)
	}

	if len(folders) == 0 && len(patterns) == 0 {
		return files
	}

	var filtered []string
	for _, f := range files {
		base := filepath.Base(f)
		dir := filepath.Dir(f)
		skip := false

		dirParts := strings.Split(dir, string(os.PathSeparator))
		for _, folderRule := range folders {
			for _, seg := range dirParts {
				if matchRule(folderRule, seg) {
					fmt.Printf("[Filters] Skipping %s (folder %s disabled)\n", base, folderRule)
					skip = true
					break
				}
			}
			if skip {
				break
			}
		}
		if skip {
			continue
		}

		for _, fileRule := range patterns {
			if matchRule(fileRule, base) {
				fmt.Printf("[Filters] Skipping %s (pattern %s disabled)\n", base, fileRule)
				skip = true
				break
			}
		}

		if !skip {
			filtered = append(filtered, f)
		}
	}
	return filtered
}

func FilterExtensions(files []string, systemID string, cfg *config.UserConfig) []string {
	var rules []string
	sysKey := strings.ToLower(systemID)

	if global, ok := cfg.Disable["all"]; ok {
		rules = append(rules, global.Extensions...)
	}
	if sys, ok := cfg.Disable[sysKey]; ok {
		rules = append(rules, sys.Extensions...)
	}

	if len(rules) == 0 {
		return files
	}

	extMap := make(map[string]struct{})
	for _, e := range rules {
		e = strings.ToLower(e)
		if !strings.HasPrefix(e, ".") {
			e = "." + e
		}
		extMap[e] = struct{}{}
	}

	var filtered []string
	for _, f := range files {
		ext := strings.ToLower(filepath.Ext(f))
		if _, skip := extMap[ext]; skip {
			fmt.Printf("[Filters] Skipping %s (extension %s disabled)\n", filepath.Base(f), ext)
			continue
		}
		filtered = append(filtered, f)
	}
	return filtered
}

// -----------------------------
// Filterlist runners
// -----------------------------

// Detailed version with per-category counts
func ApplyFilterlistsDetailed(gamelistDir, system string, lines []string, cfg *config.UserConfig) ([]string, map[string]int, bool) {
	filterBase := config.FilterlistDir()
	hadLists := false
	counts := map[string]int{"White": 0, "Black": 0, "Static": 0, "Folder": 0, "File": 0}

	// Whitelist
	if cfg.List.UseWhitelist && AllowedFor(system, cfg.List.WhitelistInclude, cfg.List.WhitelistExclude) {
		whitelist := ReadNameSet(filepath.Join(filterBase, system+"_whitelist.txt"))
		if whitelist != nil {
			hadLists = true
			var kept []string
			for _, l := range lines {
				name, _ := utils.NormalizeEntry(l)
				if _, ok := whitelist[name]; ok {
					kept = append(kept, l)
				} else {
					counts["White"]++
				}
			}
			lines = kept
		}
	}

	// Blacklist
	if cfg.List.UseBlacklist && AllowedFor(system, cfg.List.BlacklistInclude, cfg.List.BlacklistExclude) {
		bl := ReadNameSet(filepath.Join(filterBase, system+"_blacklist.txt"))
		if bl != nil {
			hadLists = true
			var kept []string
			for _, l := range lines {
				name, _ := utils.NormalizeEntry(l)
				if _, bad := bl[name]; bad {
					counts["Black"]++
					continue
				}
				kept = append(kept, l)
			}
			lines = kept
		}
	}

	// Staticlist
	if cfg.List.UseStaticlist && AllowedFor(system, cfg.List.StaticlistInclude, cfg.List.StaticlistExclude) {
		sm := ReadStaticMap(filepath.Join(filterBase, system+"_staticlist.txt"))
		if sm != nil {
			hadLists = true
			for i, l := range lines {
				name, _ := utils.NormalizeEntry(l)
				if ts, ok := sm[name]; ok {
					lines[i] = "<" + ts + ">" + l
					counts["Static"]++
				}
			}
		}
	}

	before := len(lines)
	lines = FilterFoldersAndFiles(lines, system, cfg)
	counts["Folder"] = before - len(lines)

	before = len(lines)
	lines = FilterUniqueWithMGL(lines)
	counts["File"] = before - len(lines)

	cache.SetList(system+"_gamelist.txt", lines)
	return lines, counts, hadLists
}

// Simpler version: just apply filters, no per-category counts
func ApplyFilterlists(gamelistDir, systemID string, files []string, cfg *config.UserConfig) ([]string, bool) {
	filterBase := config.FilterlistDir()
	hadLists := false

	// Whitelist
	if cfg.List.UseWhitelist {
		whitelistPath := filepath.Join(filterBase, systemID+"_whitelist.txt")
		if f, err := os.Open(whitelistPath); err == nil {
			hadLists = true
			defer f.Close()
			whitelist := make(map[string]struct{})
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				name, _ := utils.NormalizeEntry(scanner.Text())
				if name != "" {
					whitelist[name] = struct{}{}
				}
			}
			var kept []string
			for _, file := range files {
				name, _ := utils.NormalizeEntry(filepath.Base(file))
				if _, ok := whitelist[name]; ok {
					kept = append(kept, file)
				}
			}
			files = kept
		}
	}

	// Blacklist
	if cfg.List.UseBlacklist {
		blacklistPath := filepath.Join(filterBase, systemID+"_blacklist.txt")
		if f, err := os.Open(blacklistPath); err == nil {
			hadLists = true
			defer f.Close()
			blacklist := make(map[string]struct{})
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				name, _ := utils.NormalizeEntry(scanner.Text())
				if name != "" {
					blacklist[name] = struct{}{}
				}
			}
			var kept []string
			for _, file := range files {
				name, _ := utils.NormalizeEntry(filepath.Base(file))
				if _, bad := blacklist[name]; !bad {
					kept = append(kept, file)
				}
			}
			files = kept
		}
	}

	// Staticlist
	if cfg.List.UseStaticlist {
		staticPath := filepath.Join(filterBase, systemID+"_staticlist.txt")
		if f, err := os.Open(staticPath); err == nil {
			hadLists = true
			defer f.Close()
			staticMap := make(map[string]string)
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" {
					continue
				}
				parts := strings.SplitN(line, " ", 2)
				if len(parts) != 2 {
					continue
				}
				ts := strings.Trim(parts[0], "<>")
				name, _ := utils.NormalizeEntry(parts[1])
				staticMap[name] = ts
			}
			for i, f := range files {
				name, _ := utils.NormalizeEntry(filepath.Base(f))
				if ts, ok := staticMap[name]; ok {
					files[i] = "<" + ts + ">" + f
				}
			}
		}
	}

	return files, hadLists
}
