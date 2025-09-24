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

// mergeCounts merges multiple count maps into one.
func mergeCounts(ms ...map[string]int) map[string]int {
	out := map[string]int{}
	for _, m := range ms {
		for k, v := range m {
			out[k] += v
		}
	}
	return out
}

//
// -----------------------------
// Helpers for splitting/normalizing
// -----------------------------

func SplitNTrim(s, sep string, n int) []string {
	parts := strings.SplitN(s, sep, n)
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

//
// -----------------------------
// Stage Filters
// -----------------------------

func Stage1Filters(files []string, systemID string, cfg *config.UserConfig) ([]string, map[string]int) {
	counts := map[string]int{"File": 0}

	beforeExt := len(files)
	filtered := FilterExtensions(files, systemID, cfg)
	counts["File"] = beforeExt - len(filtered)

	return filtered, counts
}

func Stage2Filters(files []string) ([]string, map[string]int) {
	counts := map[string]int{"File": 0}

	beforeMGL := len(files)
	filtered := FilterUniqueWithMGL(files)
	mglRemoved := beforeMGL - len(filtered)

	beforeDedupe := len(filtered)
	filtered = utils.DedupeFiles(filtered)
	dedupeRemoved := beforeDedupe - len(filtered)

	counts["File"] = mglRemoved + dedupeRemoved
	return filtered, counts
}

func Stage3Filters(gamelistDir, systemID string, diskLines []string, cfg *config.UserConfig) ([]string, map[string]int, bool) {
	return ApplyFilterlists(gamelistDir, systemID, diskLines, cfg)
}

//
// -----------------------------
// Include / Exclude filtering
// -----------------------------

func FilterAllowed(all []string, includeRaw, excludeRaw []string) []string {
	include, _ := ExpandGroups(includeRaw)
	exclude, _ := ExpandGroups(excludeRaw)

	var filtered []string
	for _, key := range all {
		systemID := strings.TrimSuffix(key, "_gamelist.txt")

		if len(include) > 0 && !ContainsInsensitive(include, systemID) {
			continue
		}
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

func GamelistFilename(systemID string) string {
	return systemID + "_gamelist.txt"
}

func FileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func WriteLinesIfChanged(path string, lines []string) error {
	content := []byte(strings.Join(lines, "\n") + "\n")
	return WriteFileIfChanged(path, content)
}

func WriteJSONIfChanged(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return WriteFileIfChanged(path, data)
}

func WriteFileIfChanged(path string, data []byte) error {
	f, err := os.Open(path)
	if err == nil {
		defer f.Close()

		oldHash := fnv.New64a()
		if _, err := io.Copy(oldHash, f); err == nil {
			newHash := fnv.New64a()
			newHash.Write(data)

			if bytes.Equal(oldHash.Sum(nil), newHash.Sum(nil)) {
				return nil
			}
		}
	}
	return os.WriteFile(path, data, 0o644)
}

//
// -----------------------------
// Filter helpers
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

//
// -----------------------------
// Filterlist pipeline
// -----------------------------

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

func CountMaster() int {
	all := FlattenCache("master")
	count := 0
	for _, line := range all {
		if !strings.HasPrefix(line, "# SYSTEM:") {
			count++
		}
	}
	return count
}

func UpdateGameIndex(systemID string, files []string) {
	unique := utils.DedupeFiles(files)
	AmendCache("index", systemID, unique)
}

//
// -----------------------------
// Timestamps
// -----------------------------

type SavedTimestamp struct {
	SystemID string    `json:"system_id"`
	Path     string    `json:"path"`
	ModTime  time.Time `json:"mod_time"`
}

func saveTimestamps(gamelistDir string, timestamps []SavedTimestamp) error {
	path := filepath.Join(gamelistDir, "Modtime")
	if err := WriteJSONIfChanged(path, timestamps); err != nil {
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
	return true, latestMod, nil
}

func updateTimestamp(list []SavedTimestamp, systemID, path string, mod time.Time) []SavedTimestamp {
	for i, ts := range list {
		if ts.SystemID == systemID && ts.Path == path {
			list[i].ModTime = mod
			return list
		}
	}
	return append(list, SavedTimestamp{SystemID: systemID, Path: path, ModTime: mod})
}

//
// -----------------------------
// Misc helpers
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

func matchRule(rule, candidate string) bool {
	rule = strings.ToLower(strings.TrimSpace(rule))
	candidate = strings.ToLower(strings.TrimSpace(candidate))

	if rule == "" || candidate == "" {
		return false
	}
	if strings.HasPrefix(rule, "*") && strings.HasSuffix(rule, "*") && len(rule) > 2 {
		return strings.Contains(candidate, strings.Trim(rule, "*"))
	}
	if strings.HasSuffix(rule, "*") {
		return strings.HasPrefix(candidate, strings.TrimSuffix(rule, "*"))
	}
	if strings.HasPrefix(rule, "*") {
		return strings.HasSuffix(candidate, strings.TrimPrefix(rule, "*"))
	}
	if !strings.Contains(rule, ".") {
		candidate = strings.TrimSuffix(candidate, filepath.Ext(candidate))
	}
	return candidate == rule
}

func printListStatus(systemID, action string, diskCount, cacheCount int, counts map[string]int) {
	if counts == nil {
		counts = map[string]int{}
	}
	fmt.Printf("[List] %-12s Disk:%d Cache:%d (White:%d Black:%d Static:%d Folder:%d File:%d) [%s]\n",
		systemID, diskCount, cacheCount,
		counts["White"], counts["Black"], counts["Static"], counts["Folder"], counts["File"], action)
}
