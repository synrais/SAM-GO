package attract

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/utils"
)

// SplitNTrim splits s into at most n substrings separated by sep,
// trims spaces from each substring, and returns the slice.
func SplitNTrim(s, sep string, n int) []string {
	parts := strings.SplitN(s, sep, n)
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

//
// -----------------------------
// Master/GameIndex helpers
// -----------------------------

// removeSystemFromMaster removes the entire block for a given system from Masterlist slice.
func removeSystemFromMaster(master []string, systemID string) []string {
	out := []string{}
	skip := false
	for _, line := range master {
		if len(line) > 9 && line[:9] == "# SYSTEM:" {
			if line == "# SYSTEM: "+systemID {
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

// removeSystemFromGameIndex removes all entries for a given system from GameIndex slice.
func removeSystemFromGameIndex(entries []GameEntry, systemID string) []GameEntry {
	out := []GameEntry{}
	for _, e := range entries {
		if e.SystemID != systemID {
			out = append(out, e)
		}
	}
	return out
}

// mergeCounts merges three sets of filter counts into a single summary map.
func mergeCounts(c1, c2, c3 map[string]int) map[string]int {
	out := map[string]int{}
	for _, c := range []map[string]int{c1, c2, c3} {
		for k, v := range c {
			out[k] += v
		}
	}
	return out
}

//
// -----------------------------
// Stage Filters
// -----------------------------

// Stage1Filters applies structural filters.
// - Extension filtering only.
// Returns stage1 lines (disk-ready) and counts (File = extensions removed).
func Stage1Filters(files []string, systemID string, cfg *config.UserConfig) ([]string, map[string]int) {
	counts := map[string]int{"File": 0}

	// Extensions only
	beforeExt := len(files)
	filtered := FilterExtensions(files, systemID, cfg)
	extRemoved := beforeExt - len(filtered)

	counts["File"] = extRemoved
	return filtered, counts
}

// Stage2Filters applies deduplication filters.
// - .mgl precedence (FilterUniqueWithMGL)
// - Normalized name deduplication (utils.DedupeFiles)
// Returns the diskLines and counts (File = mglRemoved + dedupeRemoved).
func Stage2Filters(files []string) ([]string, map[string]int) {
    counts := map[string]int{"File": 0}

	// .mgl precedence	
    beforeMGL := len(files)
    filtered := FilterUniqueWithMGL(files)
    mglRemoved := beforeMGL - len(filtered)
	
	// Normalized dedup
    beforeDedupe := len(filtered)
    filtered = utils.DedupeFiles(filtered)
    dedupeRemoved := beforeDedupe - len(filtered)

    counts["File"] = mglRemoved + dedupeRemoved
    return filtered, counts
}

// Stage3Filters applies semantic filterlists.
// - whitelist, blacklist, staticlist
// - folder/file rules
// Returns cacheLines, counts, and hadLists flag.
func Stage3Filters(gamelistDir, systemID string, diskLines []string, cfg *config.UserConfig) ([]string, map[string]int, bool) {
	return ApplyFilterlists(gamelistDir, systemID, diskLines, cfg)
}

// Attract include exclude filter helper
// FilterAllowed applies include/exclude restrictions case-insensitively
// to in-RAM gamelist filenames (like "nes_gamelist.txt").
// It returns the list of allowed system IDs (not filenames).
func FilterAllowed(all []string, includeRaw, excludeRaw []string) []string {
    include, _ := ExpandGroups(includeRaw)
    exclude, _ := ExpandGroups(excludeRaw)

    var filtered []string
    for _, key := range all {
        systemID := strings.TrimSuffix(key, "_gamelist.txt")

        // include check
        if len(include) > 0 && !ContainsInsensitive(include, systemID) {
            continue
        }
        // exclude check
        if ContainsInsensitive(exclude, systemID) {
            continue
        }

        filtered = append(filtered, systemID)
    }
    return filtered
}

//
// -----------------------------
// File helpers
// -----------------------------

// GamelistFilename returns the standard gamelist filename for a system.
func GamelistFilename(systemID string) string {
	return systemID + "_gamelist.txt"
}

// FileExists reports whether the given path exists and is not a directory.
func FileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// WriteLinesIfChanged writes the given lines to disk when content differs.
func WriteLinesIfChanged(path string, lines []string) error {
	content := []byte(strings.Join(lines, "\n") + "\n")
	return WriteFileIfChanged(path, content)
}

// WriteJSONIfChanged writes v marshalled as JSON when content differs.
func WriteJSONIfChanged(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return WriteFileIfChanged(path, data)
}

// WriteFileIfChanged writes the provided data only if it differs from the existing file.
// Uses a streaming hash instead of loading entire file into RAM.
func WriteFileIfChanged(path string, data []byte) error {
	f, err := os.Open(path)
	if err == nil {
		defer f.Close()

		// Hash existing file
		oldHash := fnv.New64a()
		if _, err := io.Copy(oldHash, f); err == nil {
			// Hash new data
			newHash := fnv.New64a()
			newHash.Write(data)

			if bytes.Equal(oldHash.Sum(nil), newHash.Sum(nil)) {
				// identical → no write
				return nil
			}
		}
	}
	// file missing or different → write fresh
	return os.WriteFile(path, data, 0o644)
}

//
// -----------------------------
// Filter helpers
// -----------------------------

// FilterUniqueWithMGL ensures .mgl takes precedence when duplicates exist.
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

// FilterFoldersAndFiles drops files matching disabled folder/file rules.
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

// FilterExtensions drops files with disabled extensions.
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

//
// -----------------------------
// Filterlist pipeline
// -----------------------------

// ApplyFilterlists applies whitelist/blacklist/staticlist and updates counters.
func ApplyFilterlists(gamelistDir, systemID string, lines []string, cfg *config.UserConfig) ([]string, map[string]int, bool) {
	filterBase := config.FilterlistDir()
	hadLists := false
	counts := map[string]int{"White": 0, "Black": 0, "Static": 0, "Folder": 0, "File": 0}

	if cfg.List.UseWhitelist && AllowedFor(systemID, cfg.List.WhitelistInclude, cfg.List.WhitelistExclude) {
		whitelist := ReadNameSet(filepath.Join(filterBase, systemID+"_whitelist.txt"))
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

	if cfg.List.UseBlacklist && AllowedFor(systemID, cfg.List.BlacklistInclude, cfg.List.BlacklistExclude) {
		bl := ReadNameSet(filepath.Join(filterBase, systemID+"_blacklist.txt"))
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

	if cfg.List.UseStaticlist && AllowedFor(systemID, cfg.List.StaticlistInclude, cfg.List.StaticlistExclude) {
		sm := ReadStaticMap(filepath.Join(filterBase, systemID+"_staticlist.txt"))
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
	lines = FilterFoldersAndFiles(lines, systemID, cfg)
	counts["Folder"] = before - len(lines)

	return lines, counts, hadLists
}

//
// -----------------------------
// Index + Master helpers
// -----------------------------

// CountGames counts all non-#SYSTEM lines in masterlist.
func CountGames(master []string) int {
	count := 0
	for _, line := range master {
		if !strings.HasPrefix(line, "# SYSTEM:") {
			count++
		}
	}
	return count
}

// UpdateGameIndex builds GameEntry objects and pushes them to the cache.
func UpdateGameIndex(systemID string, files []string) {
	unique := utils.DedupeFiles(files)
	for _, f := range unique {
		name, ext := utils.NormalizeEntry(f)
		entry := GameEntry{
			SystemID: systemID,
			Name:     name,
			Ext:      ext,
			Path:     f,
		}
		AppendGameIndex(entry)
	}
}

//
// -----------------------------
// Timestamps
// -----------------------------

// SavedTimestamp tracks last-modified info for system folders.
type SavedTimestamp struct {
	SystemID string    `json:"system_id"`
	Path     string    `json:"path"`
	ModTime  time.Time `json:"mod_time"`
}

// saveTimestamps writes the cached mod times to disk when the data changes.
func saveTimestamps(gamelistDir string, timestamps []SavedTimestamp) error {
	path := filepath.Join(gamelistDir, "Modtime")
	if err := WriteJSONIfChanged(path, timestamps); err != nil {
		return fmt.Errorf("[Modtime] Failed to save timestamps: %w", err)
	}
	return nil
}

// loadSavedTimestamps reads JSON mod time cache from disk.
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

// isFolderModified checks if the root folder was modified since the saved timestamp.
func isFolderModified(systemID, path string, saved []SavedTimestamp) (bool, time.Time, error) {
    info, err := os.Stat(path)
    if err != nil {
        return false, time.Time{}, fmt.Errorf("[Modtime] Failed to stat %s: %w", path, err)
    }
    latestMod := info.ModTime()

    for _, ts := range saved {
        if ts.SystemID == systemID && ts.Path == path {
            return latestMod.After(ts.ModTime), latestMod, nil
        }
    }

    // No record for this system → treat as modified
    return true, latestMod, nil
}

// updateTimestamp updates or adds an entry to the SavedTimestamp list.
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

//
// -----------------------------
// Misc helpers
// -----------------------------

// ContainsInsensitive checks if list contains item, ignoring case/whitespace.
func ContainsInsensitive(list []string, item string) bool {
	for _, v := range list {
		if strings.EqualFold(strings.TrimSpace(v), item) {
			return true
		}
	}
	return false
}

// MatchesSystem is a wrapper for ContainsInsensitive, for system IDs.
func MatchesSystem(list []string, system string) bool {
	return ContainsInsensitive(list, system)
}

// AllowedFor checks include/exclude rules for a system ID.
func AllowedFor(system string, include, exclude []string) bool {
	if len(include) > 0 && !MatchesSystem(include, system) {
		return false
	}
	if MatchesSystem(exclude, system) {
		return false
	}
	return true
}

// ReadNameSet loads a filter list file into a set of normalized names.
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

// ReadStaticMap loads staticlist.txt into a name→timestamp map.
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

// ReadStaticTimestamp returns the static timestamp for a game if present.
func ReadStaticTimestamp(systemID, game string) float64 {
	filterBase := config.FilterlistDir()
	path := filepath.Join(filterBase, systemID+"_staticlist.txt")

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

// matchRule applies glob-like rules (*foo*, foo*, *bar).
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

// printListStatus standardises log output for fresh/reused system processing.
func printListStatus(systemID, action string, diskCount, cacheCount int, counts map[string]int) {
	if counts == nil {
		counts = map[string]int{}
	}
	fmt.Printf("[List] %-12s Disk:%d Cache:%d (White:%d Black:%d Static:%d Folder:%d File:%d) [%s]\n",
		systemID, diskCount, cacheCount,
		counts["White"], counts["Black"], counts["Static"], counts["Folder"], counts["File"], action)
}
