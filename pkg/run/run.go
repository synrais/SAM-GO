package run

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/games"
	"github.com/synrais/SAM-GO/pkg/mister"
)

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

	// 🎵 Log "Now Playing"
	fmt.Printf("[RUN] Now Playing %s: (%s)\n", system.Id, LastPlayedName)

	// Launch game
	return mister.LaunchGame(&config.UserConfig{}, system, runPath)
}

