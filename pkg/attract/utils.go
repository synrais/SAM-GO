package attract

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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

// PickRandomGame chooses a random game from the allowed in-RAM gamelists.
func PickRandomGame(cfg *config.UserConfig, r *rand.Rand) string {
    if len(allowedLists) == 0 {
        fmt.Println("[Attract] No allowed gamelists available.")
        return ""
    }

    // Pick random gamelist from allowedLists
    listKey := allowedLists[r.Intn(len(allowedLists))]
    lines := GetList(listKey)
    if len(lines) == 0 {
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
// History navigation + Ticker reset
// -----------------------------
//

var currentIndex int = -1

// Next moves forward in history if possible, otherwise picks a random game.
// Always resets the global attract ticker.
func Next(cfg *config.UserConfig, r *rand.Rand) (string, bool) {
	hist := GetList("History.txt")

	// Move forward in history if possible
	if currentIndex >= 0 && currentIndex < len(hist)-1 {
		currentIndex++
		path := hist[currentIndex]
		Run([]string{path})
		resetGlobalTicker(cfg, r)
		return path, true
	}

	// Otherwise pick a new random game
	path := PickRandomGame(cfg, r)
	if path == "" {
		fmt.Println("[Attract] No game available to play.")
		return "", false
	}

	hist = append(hist, path)
	SetList("History.txt", hist)
	currentIndex = len(hist) - 1

	Run([]string{path})
	resetGlobalTicker(cfg, r)
	return path, true
}

// Back moves backward in history if possible.
// Always resets the global attract ticker.
func Back(cfg *config.UserConfig, r *rand.Rand) (string, bool) {
	hist := GetList("History.txt")

	if currentIndex > 0 {
		currentIndex--
		path := hist[currentIndex]
		Run([]string{path})
		resetGlobalTicker(cfg, r)
		return path, true
	}

	// Nothing to go back to
	return "", false
}

// resetGlobalTicker resets the singleton attract ticker.
func resetGlobalTicker(cfg *config.UserConfig, r *rand.Rand) {
	wait := ParsePlayTime(cfg.Attract.PlayTime, r)
	ResetAttractTicker(wait)
}

// Allowed gamelists after include/exclude filtering
var allowedLists []string

// FilterAllowed applies include/exclude restrictions case-insensitively
// for in-RAM gamelist keys (like "nes_gamelist.txt").
// Also updates the global allowedLists used by PickRandomGame.
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
        filtered = append(filtered, key)
    }

    // 🔹 update the global
    allowedLists = filtered
    return filtered
}

//
// -----------------------------
// Global Attract Ticker
// -----------------------------
//

var (
	attractTicker     *time.Ticker
	attractTickerStop chan struct{}
	attractTickerMu   sync.Mutex
)

// ResetAttractTicker stops any existing ticker and starts a new one.
func ResetAttractTicker(d time.Duration) {
	attractTickerMu.Lock()
	defer attractTickerMu.Unlock()

	// Stop old ticker if running
	if attractTicker != nil {
		attractTicker.Stop()
		if attractTickerStop != nil {
			close(attractTickerStop)
		}
	}

	attractTicker = time.NewTicker(d)
	attractTickerStop = make(chan struct{})
}

// AttractTickerChan returns the current ticker channel.
func AttractTickerChan() <-chan time.Time {
	attractTickerMu.Lock()
	defer attractTickerMu.Unlock()
	if attractTicker != nil {
		return attractTicker.C
	}
	return nil
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

// Run launches a game or redirects to a custom loader if registered.
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

	// 🔹 Custom loader hook
	if handled, err := TryCustomLoader(system, runPath); handled {
		return err
	}

	// Default core loader
	return mister.LaunchGame(&config.UserConfig{}, system, runPath)
}
