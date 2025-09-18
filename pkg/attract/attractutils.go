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
	if input.GameIndex == nil {
		input.GameIndex = make(map[string][]string)
	}
	input.GameIndex[systemID] = utils.DedupeFiles(files)
}

// -----------------------------
// Timestamp helpers
// -----------------------------

type systemTimestamps map[string]map[string]time.Time

func loadSavedTimestamps(dir string) (systemTimestamps, error) {
	path := filepath.Join(dir, "timestamps.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return make(systemTimestamps), nil
	}
	var ts systemTimestamps
	if err := json.Unmarshal(data, &ts); err != nil {
		return make(systemTimestamps), err
	}
	return ts, nil
}

func saveTimestamps(dir string, ts systemTimestamps) error {
	path := filepath.Join(dir, "timestamps.json")
	data, err := json.MarshalIndent(ts, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func isFolderModified(systemID, path string, ts systemTimestamps) (bool, time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, time.Time{}, err
	}
	modTime := info.ModTime()
	prev := ts[systemID][path]
	return modTime.After(prev), modTime, nil
}

func updateTimestamp(ts systemTimestamps, systemID, path string, modTime time.Time) systemTimestamps {
	if ts[systemID] == nil {
		ts[systemID] = make(map[string]time.Time)
	}
	ts[systemID][path] = modTime
	return ts
}

// SavedTimestamp holds last modified time of a system path.
type SavedTimestamp struct {
	SystemID string    `json:"system_id"`
	Path     string    `json:"path"`
	ModTime  time.Time `json:"mod_time"`
}

// saveTimestamps writes all tracked system path modtimes to disk.
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

// loadSavedTimestamps loads all saved modtimes from disk.
// Returns an empty slice if no file exists.
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

// isFolderModified checks if a folder or any subfolder was modified
// since the last saved timestamp. Only considers directories, not files.
func isFolderModified(systemID, path string, saved []SavedTimestamp) (bool, time.Time, error) {
	var latestMod time.Time

	err := filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			// skip problematic subdirs
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

	// Compare against saved record
	for _, ts := range saved {
		if ts.SystemID == systemID && ts.Path == path {
			return latestMod.After(ts.ModTime), latestMod, nil
		}
	}

	// No record yet → treat as modified
	return true, latestMod, nil
}

// updateTimestamp updates or inserts a system path record with the given modtime.
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
