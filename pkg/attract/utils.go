package attract

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

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

//
// -----------------------------
// Extra helpers
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

//
// -----------------------------
// Shuffle pool (per system)
// -----------------------------

var usedPools = make(map[string]map[string]bool)

// PickRandomGame chooses a random game without repeats until all for that system are used.
// It also appends the chosen game into history, updates currentIndex, runs it, and resets the timer.
func PickRandomGame(cfg *config.UserConfig, r *rand.Rand) string {
	fmt.Println("[DEBUG][PickRandomGame] called")

	keys := CacheKeys("lists")
	fmt.Println("[DEBUG][PickRandomGame] all list keys:", keys)

	if len(keys) == 0 {
		fmt.Println("[Attract] No gamelists available in memory.")
		return ""
	}

	// Only consider *_gamelist.txt files (exclude History.txt and others)
	var systemKeys []string
	for _, k := range keys {
		if strings.HasSuffix(k, "_gamelist.txt") {
			systemKeys = append(systemKeys, k)
		}
	}
	fmt.Println("[DEBUG][PickRandomGame] filtered systemKeys:", systemKeys)

	if len(systemKeys) == 0 {
		fmt.Println("[Attract] No gamelists found (excluding history).")
		return ""
	}

	listKey := systemKeys[r.Intn(len(systemKeys))]
	fmt.Println("[DEBUG][PickRandomGame] chosen listKey:", listKey)

	lines := GetCache("lists", listKey)
	fmt.Println("[DEBUG][PickRandomGame] entries in list:", len(lines))
	if len(lines) == 0 {
		return ""
	}

	if usedPools[listKey] == nil {
		usedPools[listKey] = make(map[string]bool)
		fmt.Println("[DEBUG][PickRandomGame] created new used pool for", listKey)
	}
	used := usedPools[listKey]

	var unused []string
	for _, line := range lines {
		_, gamePath := utils.ParseLine(line)
		if !used[gamePath] {
			unused = append(unused, gamePath)
		}
	}
	fmt.Println("[DEBUG][PickRandomGame] unused entries:", len(unused))

	if len(unused) == 0 {
		fmt.Println("[DEBUG][PickRandomGame] pool exhausted, resetting for", listKey)
		usedPools[listKey] = make(map[string]bool)
		used = usedPools[listKey]
		for _, line := range lines {
			_, gamePath := utils.ParseLine(line)
			unused = append(unused, gamePath)
		}
	}

	choice := unused[0]
	if cfg.Attract.Random && len(unused) > 1 {
		choice = unused[r.Intn(len(unused))]
	}
	fmt.Println("[DEBUG][PickRandomGame] choice:", choice)

	used[choice] = true

	// update history
	hist := GetCache("lists", "History.txt")
	fmt.Println("[DEBUG][PickRandomGame] history len before:", len(hist))
	hist = append(hist, choice)
	SetCache("lists", "History.txt", hist)
	currentIndex = len(hist) - 1
	fmt.Println("[DEBUG][PickRandomGame] history len after:", len(hist), "currentIndex:", currentIndex)

	fmt.Println("[DEBUG][PickRandomGame] calling Run with:", choice)
	Run([]string{choice})

	// reset timer
	wait := ParsePlayTime(cfg.Attract.PlayTime, r)
	fmt.Println("[DEBUG][PickRandomGame] resetting timer, duration:", wait)
	ResetAttractTimer(wait)

	return choice
}

//
// -----------------------------
// History navigation
// -----------------------------

var currentIndex int = -1

func Next(cfg *config.UserConfig, r *rand.Rand) (string, bool) {
	fmt.Println("[DEBUG][Next] called, currentIndex:", currentIndex)

	hist := GetCache("lists", "History.txt")
	fmt.Println("[DEBUG][Next] history length:", len(hist))

	// Case 1: move forward in history
	if currentIndex >= 0 && currentIndex < len(hist)-1 {
		currentIndex++
		_, path := utils.ParseLine(hist[currentIndex])
		fmt.Println("[DEBUG][Next] moving forward to history index:", currentIndex, "->", path)

		if err := Run([]string{path}); err != nil {
			fmt.Printf("[Attract] Failed to run %s: %v\n", path, err)
			return "", false
		}
		ResetAttractTimer(ParsePlayTime(cfg.Attract.PlayTime, r))
		return path, true
	}

	// Case 2: no forward history
	fmt.Println("[DEBUG][Next] picking random game")
	path := PickRandomGame(cfg, r)
	if path == "" {
		fmt.Println("[Attract] No game available to play.")
		return "", false
	}
	fmt.Println("[DEBUG][Next] PickRandomGame chose:", path)
	// PickRandomGame already resets timer
	return path, true
}

func Back(cfg *config.UserConfig, r *rand.Rand) (string, bool) {
	hist := GetCache("lists", "History.txt")

	if currentIndex > 0 {
		currentIndex--
		_, path := utils.ParseLine(hist[currentIndex])
		Run([]string{path})
		ResetAttractTimer(ParsePlayTime(cfg.Attract.PlayTime, r))
		return path, true
	}
	return "", false
}

//
// -----------------------------
// Global Attract Timer
// -----------------------------

var (
	attractTimer   *time.Timer
	attractTimerMu sync.Mutex
)

func ResetAttractTimer(d time.Duration) {
	attractTimerMu.Lock()
	defer attractTimerMu.Unlock()

	if attractTimer != nil {
		if !attractTimer.Stop() {
			select {
			case <-attractTimer.C:
			default:
			}
		}
	}
	attractTimer = time.NewTimer(d)
}

func AttractTimerChan() <-chan time.Time {
	attractTimerMu.Lock()
	defer attractTimerMu.Unlock()
	if attractTimer != nil {
		return attractTimer.C
	}
	return nil
}

//
// -----------------------------
// Game Runner / Now Playing
// -----------------------------

const nowPlayingFile = "/tmp/Now_Playing.txt"

var (
	LastPlayedPath string
	LastStartTime  time.Time

	lastSystemPtr unsafe.Pointer // *games.System
	lastNamePtr   unsafe.Pointer // *string
)

func setLastPlayed(system games.System, path string) {
	LastPlayedPath = path
	LastStartTime = time.Now()

	base := filepath.Base(path)
	name := strings.TrimSuffix(base, filepath.Ext(base))

	// Store atomically
	atomic.StorePointer(&lastSystemPtr, unsafe.Pointer(&system))
	atomic.StorePointer(&lastNamePtr, unsafe.Pointer(&name))
}

func getLastPlayed() (games.System, string) {
	sysPtr := (*games.System)(atomic.LoadPointer(&lastSystemPtr))
	namePtr := (*string)(atomic.LoadPointer(&lastNamePtr))
	if sysPtr != nil && namePtr != nil {
		return *sysPtr, *namePtr
	}
	return games.System{}, ""
}

func writeNowPlayingFile() error {
	system, name := getLastPlayed()
	line1 := fmt.Sprintf("[%s] %s", system.Name, name)
	base := filepath.Base(LastPlayedPath)
	line2 := fmt.Sprintf("%s %s", system.Id, base)
	line3 := LastPlayedPath
	content := strings.Join([]string{line1, line2, line3}, "\n")
	return os.WriteFile(nowPlayingFile, []byte(content), 0644)
}

func Run(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("Usage: SAM -run <path>")
	}
	runPath := args[0]

	fmt.Println("[DEBUG][Run] called with args:", args)
	fmt.Println("[DEBUG][Run] runPath:", runPath)

	// ⚡ We don’t need BestSystemMatch anymore for LaunchGenericFile
	// But we still update LastPlayed + NowPlaying so the metadata is correct.
	system, _ := games.BestSystemMatch(&config.UserConfig{}, runPath)
	setLastPlayed(system, runPath)
	fmt.Println("[DEBUG][Run] LastPlayedPath set to:", LastPlayedPath)

	if err := writeNowPlayingFile(); err != nil {
		fmt.Printf("[RUN] Failed to write Now_Playing.txt: %v\n", err)
	} else {
		fmt.Println("[DEBUG][Run] Now_Playing.txt updated")
	}

	_, name := getLastPlayed()
	fmt.Printf("[RUN] Now Playing %s: %s\n", system.Name, name)

	// 🔹 Switch to LaunchGenericFile
	err := mister.LaunchGenericFile(&config.UserConfig{}, runPath)
	if err != nil {
		fmt.Printf("[RUN] LaunchGenericFile failed for %s: %v\n", runPath, err)
	} else {
		fmt.Println("[DEBUG][Run] LaunchGenericFile succeeded for", runPath)
	}

	return err
}

