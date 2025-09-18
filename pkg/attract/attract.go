package attract

import (
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
	"github.com/synrais/SAM-GO/pkg/history"
	"github.com/synrais/SAM-GO/pkg/input"
	"github.com/synrais/SAM-GO/pkg/run"
	"github.com/synrais/SAM-GO/pkg/staticdetector"
	"github.com/synrais/SAM-GO/pkg/utils"
)

// parsePlayTime handles "40" or "40-130"
func parsePlayTime(value string, r *rand.Rand) time.Duration {
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

// matchesPattern checks if string matches a wildcard (*foo*, bar*, *baz)
func matchesPattern(s, pattern string) bool {
	p := strings.ToLower(pattern)
	s = strings.ToLower(s)

	if strings.HasPrefix(p, "*") && strings.HasSuffix(p, "*") {
		return strings.Contains(s, strings.Trim(p, "*"))
	}
	if strings.HasPrefix(p, "*") {
		return strings.HasSuffix(s, strings.TrimPrefix(p, "*"))
	}
	if strings.HasSuffix(p, "*") {
		return strings.HasPrefix(s, strings.TrimSuffix(p, "*"))
	}
	return s == p
}

// disabled checks if a game should be blocked by rules
func disabled(system string, gamePath string, cfg *config.UserConfig) bool {
	rules, ok := cfg.Disable[system]
	if !ok {
		return false
	}

	base := filepath.Base(gamePath)
	ext := filepath.Ext(gamePath)
	dir := filepath.Base(filepath.Dir(gamePath))

	for _, f := range rules.Folders {
		if matchesPattern(dir, f) {
			return true
		}
	}
	for _, f := range rules.Files {
		if matchesPattern(base, f) {
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

// getSystemsByCategory retrieves systems by category (Console, Handheld, Arcade, etc.)
func getSystemsByCategory(category string) ([]string, error) {
	var systems []string
	for _, sys := range games.AllSystems() {
		if strings.EqualFold(sys.Category, category) {
			systems = append(systems, sys.Id)
		}
	}
	if len(systems) == 0 {
		return nil, fmt.Errorf("no systems found in category: %s", category)
	}
	return systems, nil
}

// expandGroups expands category/group names (Console, Handheld, Arcade, Computer) into system IDs.
func expandGroups(list []string) ([]string, error) {
	var expanded []string
	for _, item := range list {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if trimmed == "Console" || trimmed == "Handheld" || trimmed == "Arcade" || trimmed == "Computer" {
			groupSystems, err := getSystemsByCategory(trimmed)
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

// filterAllowed applies include/exclude restrictions case-insensitively.
func filterAllowed(all []string, include, exclude []string) []string {
	var filtered []string
	for _, sys := range all {
		base := strings.TrimSuffix(filepath.Base(sys), "_gamelist.txt")
		if len(include) > 0 {
			match := false
			for _, s := range include {
				if strings.EqualFold(strings.TrimSpace(s), base) {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}
		skip := false
		for _, s := range exclude {
			if strings.EqualFold(strings.TrimSpace(s), base) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		filtered = append(filtered, sys)
	}
	return filtered
}

// Run executes attract mode using the provided config and args.
func Run(cfg *config.UserConfig, args []string) {
	attractCfg := cfg.Attract

	// Ensure gamelists are built using CreateGamelists
	systemPaths := games.GetSystemPaths(cfg, games.AllSystems())
	if CreateGamelists(cfg, config.GamelistDir(), systemPaths, false) == 0 {
		fmt.Fprintln(os.Stderr, "[Attract] List build failed: no games indexed")
	}

	// Load lists into cache with filters
	for _, system := range games.AllSystems() {
		files, _ := filepath.Glob(filepath.Join(config.GamelistDir(), "*_"+system.Id+"_gamelist.txt"))
		for _, f := range files {
			lines, err := utils.ReadLines(f)
			if err != nil {
				continue
			}
			lines, counts, _ := ApplyFilterlistsDetailed(config.GamelistDir(), system.Id, lines, cfg)
			cache.SetList(filepath.Base(f), lines)
			if counts["White"] > 0 || counts["Black"] > 0 || counts["Static"] > 0 || counts["Folder"] > 0 || counts["File"] > 0 {
				fmt.Printf("[Attract] %s - White: %d, Black: %d, Static: %d, Folder: %d, File: %d\n",
					system.Id, counts["White"], counts["Black"], counts["Static"], counts["Folder"], counts["File"])
			}
		}
	}

	// control channels
	skipCh := make(chan struct{}, 1)
	backCh := make(chan struct{}, 1)

	// parse extra flags
	silent := false
	for _, a := range args {
		if a == "-s" || a == "--silent" {
			silent = true
		}
	}

	// Start static detector → only feeds into skipCh
	if attractCfg.UseStaticDetector {
		go func() {
			for ev := range staticdetector.Stream(cfg, skipCh) {
				if !silent {
					fmt.Printf("[Attract] %s\n", ev)
				}
			}
		}()
	}

	// Hook user inputs → back = backCh, next = skipCh
	if cfg.InputDetector.Mouse || cfg.InputDetector.Keyboard || cfg.InputDetector.Joystick {
		input.RelayInputs(cfg,
			func() { // Back
				select {
				case backCh <- struct{}{}:
				default:
				}
			},
			func() { // Next
				select {
				case skipCh <- struct{}{}:
				default:
				}
			},
		)
	}

	// Collect gamelists
	allKeys := cache.ListKeys()
	var allFiles []string
	for _, k := range allKeys {
		if strings.HasSuffix(k, "_gamelist.txt") {
			allFiles = append(allFiles, k)
		}
	}

	// Expand groups in include/exclude
	include, err := expandGroups(attractCfg.Include)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[Attract] Error expanding include groups: %v\n", err)
		os.Exit(1)
	}
	exclude, err := expandGroups(attractCfg.Exclude)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[Attract] Error expanding exclude groups: %v\n", err)
		os.Exit(1)
	}

	files := filterAllowed(allFiles, include, exclude)
	if len(files) == 0 {
		fmt.Println("[Attract] No gamelists found in cache")
		os.Exit(1)
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	if !silent {
		fmt.Println("[Attract] Running. Ctrl-C to exit.")
	}

	// Game loop
	playGame := func(gamePath, systemID string, ts float64) {
	Launch:
		for {
			name := filepath.Base(gamePath)
			name = strings.TrimSuffix(name, filepath.Ext(name))
			if !silent {
				fmt.Printf("[Attract] %s - %s <%s>\n",
					time.Now().Format("15:04:05"), name, gamePath)
			}
			run.Run([]string{gamePath})

			// base playtime
			wait := parsePlayTime(attractCfg.PlayTime, r)
			if ts > 0 {
				skipDuration := time.Duration(ts*float64(time.Second)) +
					time.Duration(cfg.List.SkipafterStatic)*time.Second
				if skipDuration < wait {
					wait = skipDuration
				}
			}

			deadline := time.Now().Add(wait)
			for time.Now().Before(deadline) {
				if input.IsSearching() {
					time.Sleep(100 * time.Millisecond)
					deadline = deadline.Add(100 * time.Millisecond)
					continue
				}
				remaining := time.Until(deadline)
				select {
				case <-time.After(remaining):
					if next, err := history.PlayNext(); err == nil && next != "" {
						_ = history.SetNowPlaying(next)
						gamePath = next
						systemID, ts = "", 0
						continue Launch
					}
					return
				case <-skipCh:
					if !silent {
						fmt.Println("[Attract] Skipped")
					}
					if next, err := history.PlayNext(); err == nil && next != "" {
						_ = history.SetNowPlaying(next)
						gamePath = next
						systemID, ts = "", 0
						continue Launch
					}
					return
				case <-backCh:
					if prev, err := history.PlayBack(); err == nil && prev != "" {
						_ = history.SetNowPlaying(prev)
						gamePath = prev
						systemID, ts = "", 0
						continue Launch
					}
				}
			}
			if next, err := history.PlayNext(); err == nil && next != "" {
				_ = history.SetNowPlaying(next)
				gamePath = next
				systemID, ts = "", 0
				continue Launch
			}
			return
		}
	}

	// Main attract loop
	for {
		select {
		case <-backCh:
			if prev, err := history.PlayBack(); err == nil && prev != "" {
				_ = history.SetNowPlaying(prev)
				playGame(prev, "", 0)
				continue
			}
		case <-skipCh:
			if next, err := history.PlayNext(); err == nil && next != "" {
				_ = history.SetNowPlaying(next)
				playGame(next, "", 0)
				continue
			}
		default:
		}

		if len(files) == 0 {
			if !silent {
				fmt.Println("[Attract] All systems exhausted — refreshing from cache")
			}
			cache.ResetAll()
			allKeys = cache.ListKeys()
			allFiles = nil
			for _, k := range allKeys {
				if strings.HasSuffix(k, "_gamelist.txt") {
					allFiles = append(allFiles, k)
				}
			}
			files = filterAllowed(allFiles, include, exclude)
			if len(files) == 0 {
				fmt.Println("[Attract] No gamelists even after reset, exiting.")
				return
			}
		}

		listKey := files[r.Intn(len(files))]
		lines := cache.GetList(listKey)
		if len(lines) == 0 {
			var newFiles []string
			for _, f := range files {
				if f != listKey {
					newFiles = append(newFiles, f)
				}
			}
			files = newFiles
			continue
		}

		index := 0
		if attractCfg.Random {
			index = r.Intn(len(lines))
		}
		ts, gamePath := utils.ParseLine(lines[index])
		systemID := strings.TrimSuffix(filepath.Base(listKey), "_gamelist.txt")

		if disabled(systemID, gamePath, cfg) {
			lines = append(lines[:index], lines[index+1:]...)
			cache.SetList(listKey, lines)
			continue
		}

		_ = history.Play(gamePath)
		playGame(gamePath, systemID, ts)

		lines = append(lines[:index], lines[index+1:]...)
		cache.SetList(listKey, lines)
	}
}
