package attract

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/games"
	"github.com/synrais/SAM-GO/pkg/mister"
	"github.com/synrais/SAM-GO/pkg/utils"
)

//
// -----------------------------
// System/group helpers
// -----------------------------
//

// GetSystemsByCategory retrieves systems by category (Console, Handheld, Arcade, etc.).
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

//
// -----------------------------
// Filters
// -----------------------------
//

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

		// folder rules
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

		// file rules
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
// Filterlist runners
// -----------------------------
//

// ApplyFilterlists applies whitelist/blacklist/staticlist and updates counters.
func ApplyFilterlists(gamelistDir, systemID string, lines []string, cfg *config.UserConfig) ([]string, map[string]int, bool) {
	filterBase := config.FilterlistDir()
	hadLists := false
	counts := map[string]int{"White": 0, "Black": 0, "Static": 0, "Folder": 0, "File": 0}

	// Whitelist
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

	// Blacklist
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

	// Staticlist
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

	// Folders/files filters
	before := len(lines)
	lines = FilterFoldersAndFiles(lines, systemID, cfg)
	counts["Folder"] = before - len(lines)

	// Deduplication / .mgl precedence
	before = len(lines)
	lines = FilterUniqueWithMGL(lines)
	counts["File"] = before - len(lines)

	SetList(systemID+"_gamelist.txt", lines)
	return lines, counts, hadLists
}

//
// -----------------------------
// List + Masterlist helpers
// -----------------------------
//

// fileExists checks if path exists and is not a dir.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// gamelistFilename returns standard system gamelist filename.
func gamelistFilename(systemID string) string {
	return systemID + "_gamelist.txt"
}

// writeGamelist saves a gamelist (unless RAM-only).
func writeGamelist(dir, systemID string, files []string, ramOnly bool) {
	if ramOnly {
		return
	}
	path := filepath.Join(dir, gamelistFilename(systemID))
	_ = os.WriteFile(path, []byte(strings.Join(files, "\n")), 0644)
}

// writeSimpleList saves a text file list.
func writeSimpleList(path string, lines []string) {
	_ = os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
}

// removeSystemBlock strips existing # SYSTEM: block for a system.
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

// countGames counts all non-#SYSTEM lines in masterlist.
func countGames(master []string) int {
	count := 0
	for _, line := range master {
		if !strings.HasPrefix(line, "# SYSTEM:") {
			count++
		}
	}
	return count
}

// updateGameIndex builds GameEntry objects and pushes to cache.
func updateGameIndex(systemID string, files []string) {
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
// Timestamp helpers
// -----------------------------
//

// SavedTimestamp tracks last-modified info for system folders.
type SavedTimestamp struct {
	SystemID string    `json:"system_id"`
	Path     string    `json:"path"`
	ModTime  time.Time `json:"mod_time"`
}

// saveTimestamps writes JSON modtime cache to disk.
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

// loadSavedTimestamps reads JSON modtime cache from disk.
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

// isFolderModified checks if any subfolder was modified since saved timestamp.
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

// updateTimestamp updates or adds entry to SavedTimestamp list.
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
// Case-insensitive helpers
// -----------------------------
//

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

//
// -----------------------------
// Filterlist readers
// -----------------------------
//

// ReadNameSet loads a filterlist file into a set of normalized names.
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

//
// -----------------------------
// Line helpers
// -----------------------------
//

// ParseLine extracts <timestamp> prefix and remainder.
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

//
// -----------------------------
// Matching helper
// -----------------------------
//

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

//
// -----------------------------
// Extra helpers from attract.go
// -----------------------------
//

// ParsePlayTime handles "40" or "40-130" style configs.
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

// Disabled checks if a game should be blocked by disable rules.
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

// PickRandomGame chooses a random game from the available files.
func PickRandomGame(cfg *config.UserConfig, r *rand.Rand) string {
	files, _ := filepath.Glob(filepath.Join(config.GamelistDir(), "*_gamelist.txt"))
	if len(files) == 0 {
		return ""
	}

	// Pick random gamelist
	listKey := files[r.Intn(len(files))]
	lines, err := utils.ReadLines(listKey)
	if err != nil || len(lines) == 0 {
		return ""
	}

	// Pick random entry
	index := 0
	if cfg.Attract.Random {
		index = r.Intn(len(lines))
	}
	_, gamePath := utils.ParseLine(lines[index])

	return gamePath
}

//
// -----------------------------
// History navigation + Timer reset
// -----------------------------
//

var currentIndex int = -1

func resetTimer(timer *time.Timer, d time.Duration) {
	if timer == nil {
		return
	}
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(d)
}

// Next moves forward in history if possible, otherwise picks a random game.
func Next(timer *time.Timer, cfg *config.UserConfig, r *rand.Rand) (string, bool) {
	hist := GetList("History.txt")

	if currentIndex >= 0 && currentIndex < len(hist)-1 {
		currentIndex++
		path := hist[currentIndex]
		Run([]string{path})
		resetTimer(timer, ParsePlayTime(cfg.Attract.PlayTime, r))
		return path, true
	}

	path := PickRandomGame(cfg, r)
	if path == "" {
		fmt.Println("[Attract] No game available to play.")
		return "", false
	}

	hist = append(hist, path)
	SetList("History.txt", hist)
	currentIndex = len(hist) - 1

	Run([]string{path})
	resetTimer(timer, ParsePlayTime(cfg.Attract.PlayTime, r))
	return path, true
}

// Back moves backward in history.
func Back(timer *time.Timer, cfg *config.UserConfig, r *rand.Rand) (string, bool) {
	hist := GetList("History.txt")
	if currentIndex > 0 {
		currentIndex--
		path := hist[currentIndex]
		Run([]string{path})
		resetTimer(timer, ParsePlayTime(cfg.Attract.PlayTime, r))
		return path, true
	}
	return "", false
}

//
// -----------------------------
// Game Runner / Now Playing
// -----------------------------
//

const nowPlayingFile = "/tmp/Now_Playing.txt"

var (
	LastPlayedSystem games.System
	LastPlayedPath   string
	LastPlayedName   string
	LastStartTime    time.Time
)

func GetLastPlayed() (system games.System, path, name string, start time.Time) {
	return LastPlayedSystem, LastPlayedPath, LastPlayedName, LastStartTime
}

func setLastPlayed(system games.System, path string) {
	LastPlayedSystem = system
	LastPlayedPath = path
	LastStartTime = time.Now()

	base := filepath.Base(path)
	LastPlayedName = strings.TrimSuffix(base, filepath.Ext(base))
}

func writeNowPlayingFile() error {
	line1 := fmt.Sprintf("[%s] %s", LastPlayedSystem.Name, LastPlayedName)
	base := filepath.Base(LastPlayedPath)
	line2 := fmt.Sprintf("%s %s", LastPlayedSystem.Id, base)
	line3 := LastPlayedPath
	content := strings.Join([]string{line1, line2, line3}, "\n")
	return os.WriteFile(nowPlayingFile, []byte(content), 0644)
}

func Run(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("Usage: SAM -run <path>")
	}
	runPath := args[0]

	system, _ := games.BestSystemMatch(&config.UserConfig{}, runPath)
	setLastPlayed(system, runPath)

	if err := writeNowPlayingFile(); err != nil {
		fmt.Printf("[RUN] Failed to write Now_Playing.txt: %v\n", err)
	}

	fmt.Printf("[RUN] Now Playing %s: %s\n", system.Name, LastPlayedName)
	return mister.LaunchGame(&config.UserConfig{}, system, runPath)
}
