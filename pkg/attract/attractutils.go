package attract

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/synrais/SAM-GO/pkg/cache"
	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/utils"
)

// ContainsInsensitive checks if list contains item (case-insensitive).
func ContainsInsensitive(list []string, item string) bool {
	for _, v := range list {
		if strings.EqualFold(strings.TrimSpace(v), item) {
			return true
		}
	}
	return false
}

// MatchesSystem checks if system name matches (case-insensitive).
func MatchesSystem(list []string, system string) bool {
	return ContainsInsensitive(list, system)
}

// AllowedFor determines if a system is allowed based on include/exclude lists.
func AllowedFor(system string, include, exclude []string) bool {
	if len(include) > 0 && !MatchesSystem(include, system) {
		return false
	}
	if MatchesSystem(exclude, system) {
		return false
	}
	return true
}

// ReadNameSet reads a file into a set of normalized names.
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

// ReadStaticMap reads a staticlist file mapping name → timestamp.
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

// ParseLine extracts optional <timestamp> from a gamelist line.
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

// ReadStaticTimestamp looks up a game's timestamp in staticlist.
func ReadStaticTimestamp(system, game string) float64 {
	path := filepath.Join("filterlists", system+"_staticlist.txt") // uses default dir
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
func containsInsensitive(list []string, item string) bool {
	for _, v := range list {
		if strings.EqualFold(strings.TrimSpace(v), item) {
			return true
		}
	}
	return false
}

func matchesSystem(list []string, system string) bool {
	return containsInsensitive(list, system)
}

func allowedFor(system string, include, exclude []string) bool {
	if len(include) > 0 && !matchesSystem(include, system) {
		return false
	}
	if matchesSystem(exclude, system) {
		return false
	}
	return true
}

func readNameSet(path string) map[string]struct{} {
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

func readStaticMap(path string) map[string]string {
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

// ApplyFilterlistsDetailed applies all cache-stage filters individually,
// returning the filtered lines + counts for logging.
func ApplyFilterlistsDetailed(gamelistDir, system string, lines []string, cfg *config.UserConfig) ([]string, map[string]int, bool) {
	filterBase := config.FilterlistDir()
	hadLists := false
	counts := map[string]int{"White": 0, "Black": 0, "Static": 0, "Folder": 0, "File": 0}

	// Whitelist
	if cfg.List.UseWhitelist && allowedFor(system,
		cfg.List.WhitelistInclude, cfg.List.WhitelistExclude) {
		whitelist := readNameSet(filepath.Join(filterBase, system+"_whitelist.txt"))
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
	if cfg.List.UseBlacklist && allowedFor(system,
		cfg.List.BlacklistInclude, cfg.List.BlacklistExclude) {
		bl := readNameSet(filepath.Join(filterBase, system+"_blacklist.txt"))
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
	if cfg.List.UseStaticlist && allowedFor(system,
		cfg.List.StaticlistInclude, cfg.List.StaticlistExclude) {
		sm := readStaticMap(filepath.Join(filterBase, system+"_staticlist.txt"))
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

	// Folder filters
	before := len(lines)
	lines = FilterFoldersAndFiles(lines, system, cfg)
	counts["Folder"] = before - len(lines)

	// File filters
	before = len(lines)
	lines = FilterUniqueWithMGL(lines)
	counts["File"] = before - len(lines)

	// Update cache
	cache.SetList(system+"_gamelist.txt", lines)

	return lines, counts, hadLists
}

// ParseLine extracts optional <timestamp> from a gamelist line.
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

// ReadStaticTimestamp returns the timestamp for a game from the staticlist.
func ReadStaticTimestamp(_ string, system, game string) float64 {
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

// matchRule applies human-friendly pattern matching to a candidate string.
func matchRule(rule, candidate string) bool {
	rule = strings.ToLower(strings.TrimSpace(rule))
	candidate = strings.ToLower(strings.TrimSpace(candidate))

	if rule == "" || candidate == "" {
		return false
	}

	// Wildcard contains check
	if strings.HasPrefix(rule, "*") && strings.HasSuffix(rule, "*") && len(rule) > 2 {
		sub := strings.Trim(rule, "*")
		return strings.Contains(candidate, sub)
	}
	// Prefix match
	if strings.HasSuffix(rule, "*") {
		prefix := strings.TrimSuffix(rule, "*")
		return strings.HasPrefix(candidate, prefix)
	}
	// Suffix match
	if strings.HasPrefix(rule, "*") {
		suffix := strings.TrimPrefix(rule, "*")
		return strings.HasSuffix(candidate, suffix)
	}
	// Exact match (ignore extension for candidate if rule lacks one)
	if !strings.Contains(rule, ".") {
		candidate = strings.TrimSuffix(candidate, filepath.Ext(candidate))
	}
	return candidate == rule
}

// -----------------------------
// Filters
// -----------------------------

// FilterUniqueWithMGL filters out duplicate files based on their base name
// (ignores extension) and prioritizes `.mgl` files.
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

// FilterFoldersAndFiles applies Disable.Folders and Disable.Files rules (per-system + global).
func FilterFoldersAndFiles(files []string, systemID string, cfg *config.UserConfig) []string {
	var folders, patterns []string

	// Global rules [Disable.ALL]
	if global, ok := cfg.Disable["all"]; ok {
		folders = append(folders, global.Folders...)
		patterns = append(patterns, global.Files...)
	}
	// System-specific rules
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

		// --- Folder filter (check each segment) ---
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

		// --- File pattern filter ---
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

// FilterExtensions removes files with specific extensions (per-system + global).
func FilterExtensions(files []string, systemID string, cfg *config.UserConfig) []string {
	var rules []string

	// Always normalize systemID to lowercase for lookup
	sysKey := strings.ToLower(systemID)

	// Global rules [Disable.ALL]
	if global, ok := cfg.Disable["all"]; ok {
		rules = append(rules, global.Extensions...)
	}
	// System-specific rules
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

// ApplyFilterlists applies whitelist, blacklist, and staticlist filtering to gamelist files.
func ApplyFilterlists(gamelistDir string, systemID string, files []string, cfg *config.UserConfig) ([]string, bool) {
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
