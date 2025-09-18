package run

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/games"
	"github.com/synrais/SAM-GO/pkg/mister"
)

const nowPlayingFile = "/tmp/Now_Playing.txt"

// Globals to expose last played info
var (
	LastPlayedSystem games.System
	LastPlayedPath   string
	LastPlayedName   string // basename without extension
	LastStartTime    time.Time
)

// GetLastPlayed returns the last system, path, clean name, and start time.
func GetLastPlayed() (system games.System, path, name string, start time.Time) {
	return LastPlayedSystem, LastPlayedPath, LastPlayedName, LastStartTime
}

// internal helper to update globals
func setLastPlayed(system games.System, path string) {
	LastPlayedSystem = system
	LastPlayedPath = path
	LastStartTime = time.Now()

	base := filepath.Base(path)
	LastPlayedName = strings.TrimSuffix(base, filepath.Ext(base))
}

// writeNowPlayingFile writes Now_Playing.txt with 3 lines:
// [System Name] GameName
// SystemID GameName.ext
// /full/path/to/GameName.ext
func writeNowPlayingFile() error {
	// Line 1: Pretty system name + clean game name
	line1 := fmt.Sprintf("[%s] %s", LastPlayedSystem.Name, LastPlayedName)

	// Line 2: System ID + original file basename (with extension)
	base := filepath.Base(LastPlayedPath)
	line2 := fmt.Sprintf("%s %s", LastPlayedSystem.Id, base)

	// Line 3: Full absolute path
	line3 := LastPlayedPath

	content := strings.Join([]string{line1, line2, line3}, "\n")
	return os.WriteFile(nowPlayingFile, []byte(content), 0644)
}

// Run launches a game (normal file path).
func Run(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("Usage: SAM -run <path>")
	}
	runPath := args[0]

	// Match system based on path
	system, _ := games.BestSystemMatch(&config.UserConfig{}, runPath)

	// Update globals
	setLastPlayed(system, runPath)

	// Write Now_Playing.txt
	if err := writeNowPlayingFile(); err != nil {
		fmt.Printf("[RUN] Failed to write Now_Playing.txt: %v\n", err)
	}

	// Log "Now Playing"
	fmt.Printf("[RUN] Now Playing %s: %s\n", system.Name, LastPlayedName)

	// Launch game
	return mister.LaunchGame(&config.UserConfig{}, system, runPath)
}
