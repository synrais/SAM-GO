package attract

import (
	"encoding/json"
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/synrais/SAM-GO/pkg/cache"
	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/games"
	"github.com/synrais/SAM-GO/pkg/input"
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
// Extra helpers from attract.go
// -----------------------------

// ParsePlayTime handles "40" or "40-130"
func ParsePlayTime(value string, r *rand.Rand) time.Duration {
	if strings.Contains(value, "-") {
		parts := strings.SplitN(value, "-", 2)
		min, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
		max, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
		if max > min {
			return time.Duration(r.Intn(max-min+1)+min) * time.Second
		}
		return time.Duration(min) * time.Second
	}
	secs, _ := strconv.Atoi(value)
	return time.Duration(secs) * time.Second
}

// Disabled checks if a game should be blocked by rules
func Disabled(systemID string, gamePath string, cfg *config.UserConfig) bool {
	rules, ok := cfg.Disable[systemID]
	if !ok {
		return false
	}

	base := filepath.Base(gamePath)
	ext := filepath.Ext(gamePath)
	dir := filepath.Base(filepath.Dir(gamePath))

	for _, f := range rules.Folders {
		if matchRule(f, dir) {
			return true
		}
	}
	for _, f := range rules.Files {
		if matchRule(f, base) {
			return true
		}
	}
	for _, e := range rules.Extensions {
		if strings.EqualFold(ext, e) {
			return true
		}
	}
	return false
}

// GetSystemsByCategory retrieves systems by category (Console, Handheld, Arcade, etc.)
func GetSystemsByCategory(category string) ([]string, error) {
	var systemIDs []string
	for _, systemID := range games.AllSystems() {
		if strings.EqualFold(systemID.Category, category) {
			systemIDs = append(systemIDs, systemID.Id)
		}
	}
	if len(systemIDs) == 0 {
		return nil, fmt.Errorf("no systems found in category: %s", category)
	}
	return systemIDs, nil
}

// ExpandGroups expands category/group names into system IDs.
func ExpandGroups(systemIDs []string) ([]string, error) {
	var expanded []string
	for _, systemID := range systemIDs {
		trimmed := strings.TrimSpace(systemID)
		if trimmed == "" {
			continue
		}

		if trimmed == "Console" || trimmed == "Handheld" || trimmed == "Arcade" || trimmed == "Computer" {
			groupSystems, err := GetSystemsByCategory(trimmed)
			if err != nil {
				return nil, fmt.Errorf("group not found: %v", trimmed)
			}
			expanded = append(expanded, groupSystems...)
			continue
		}

		if sys, err := games.LookupSystem(trimmed); err == nil {
			expanded = append(expanded, sys.Id)
			continue
		}

		expanded = append(expanded, trimmed)
	}
	return expanded, nil
}

// FilterAllowed applies include/exclude restrictions case-insensitively.
func FilterAllowed(all []string, include, exclude []string) []string {
	var filtered []string
	for _, sys := range all {
		base := strings.TrimSuffix(filepath.Base(sys), "_gamelist.txt")
		if len(include) > 0 {
			if !ContainsInsensitive(include, base) {
				continue
			}
		}
		if ContainsInsensitive(exclude, base) {
			continue
		}
		filtered = append(filtered, sys)
	}
	return filtered
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
	if rules, ok := cfg.Disable[strings.ToLower(systemID)]; ok {
		folders = append(folders, rules.Folders...)
		patterns = append(patterns, rules.Files...)
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
	if systemRules, ok := cfg.Disable[sysKey]; ok {
		rules = append(rules, systemRules.Extensions...)
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

// -----------------------------
// List + Masterlist helpers
// -----------------------------

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func gamelistFilename(systemID string) string {
	return systemID + "_gamelist.txt"
}

func writeGamelist(dir, systemID string, files []string, ramOnly bool) {
	if ramOnly {
		return
	}
	path := filepath.Join(dir, gamelistFilename(systemID))
	_ = os.WriteFile(path, []byte(strings.Join(files, "\n")), 0644)
}

func writeSimpleList(path string, lines []string) {
	_ = os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
}

func removeSystemBlock(master []string, systemID string) []string {
	var out []string
	skip := false
	for _, line := range master {
		if strings.HasPrefix(line, "# SYSTEM: ") {
			if strings.Contains(line, systemID) {
				skip = true
				continue
			}
			skip = false
		}
		if !skip {
			out = append(out, line)
		}
	}
	return out
}

func countGames(master []string) int {
	count := 0
	for _, line := range master {
		if !strings.HasPrefix(line, "# SYSTEM:") {
			count++
		}
	}
	return count
}

func updateGameIndex(systemID string, files []string) {
	unique := utils.DedupeFiles(files)
	for _, f := range unique {
		name, ext := utils.NormalizeEntry(f)
		entry := input.GameEntry{
			Name: name,
			Ext:  ext,
			Path: f,
		}
		input.GameIndex = append(input.GameIndex, entry)
	}
}

// -----------------------------
// Timestamp helpers (modtime version only)
// -----------------------------

type SavedTimestamp struct {
	SystemID string    `json:"system_id"`
	Path     string    `json:"path"`
	ModTime  time.Time `json:"mod_time"`
}

func saveTimestamps(gamelistDir string, timestamps []SavedTimestamp) error {
	data, err := json.MarshalIndent(timestamps, "", "  ")
	if err != nil {
		return fmt.Errorf("[Modtime] Failed to encode timestamps: %w", err)
	}
	path := filepath.Join(gamelistDir, "Modtime")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("[Modtime] Failed to save timestamps: %w", err)
	}
	return nil
}

func loadSavedTimestamps(gamelistDir string) ([]SavedTimestamp, error) {
	path := filepath.Join(gamelistDir, "Modtime")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []SavedTimestamp{}, nil
		}
		return nil, fmt.Errorf("[Modtime] Failed to read file: %w", err)
	}
	var timestamps []SavedTimestamp
	if err := json.Unmarshal(data, &timestamps); err != nil {
		return nil, fmt.Errorf("[Modtime] Failed to parse JSON: %w", err)
	}
	return timestamps, nil
}

func isFolderModified(systemID, path string, saved []SavedTimestamp) (bool, time.Time, error) {
	var latestMod time.Time

	err := filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		mod := info.ModTime()
		if mod.After(latestMod) {
			latestMod = mod
		}
		return nil
	})
	if err != nil {
		if os.IsNotExist(err) {
			return false, time.Time{}, nil
		}
		return false, time.Time{}, fmt.Errorf("[Modtime] Walk failed for %s: %w", path, err)
	}

	for _, ts := range saved {
		if ts.SystemID == systemID && ts.Path == path {
			return latestMod.After(ts.ModTime), latestMod, nil
		}
	}

	return true, latestMod, nil
}

func updateTimestamp(list []SavedTimestamp, systemID, path string, mod time.Time) []SavedTimestamp {
	for i, ts := range list {
		if ts.SystemID == systemID && ts.Path == path {
			list[i].ModTime = mod
			return list
		}
	}
	return append(list, SavedTimestamp{
		SystemID: systemID,
		Path:     path,
		ModTime:  mod,
	})
}
