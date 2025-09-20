package attract

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

var (
	LastPlayedSystem games.System
	LastPlayedPath   string
	LastPlayedName   string
	LastStartTime    time.Time
)

// GetLastPlayed returns info about last launched game.
func GetLastPlayed() (system games.System, path, name string, start time.Time) {
	return LastPlayedSystem, LastPlayedPath, LastPlayedName, LastStartTime
}

// setLastPlayed updates in-memory state for last played game.
func setLastPlayed(system games.System, path string) {
	LastPlayedSystem = system
	LastPlayedPath = path
	LastStartTime = time.Now()

	base := filepath.Base(path)
	LastPlayedName = strings.TrimSuffix(base, filepath.Ext(base))
}

// writeNowPlayingFile saves details to /tmp/Now_Playing.txt.
func writeNowPlayingFile() error {
	line1 := fmt.Sprintf("[%s] %s", LastPlayedSystem.Name, LastPlayedName)
	base := filepath.Base(LastPlayedPath)
	line2 := fmt.Sprintf("%s %s", LastPlayedSystem.Id, base)
	line3 := LastPlayedPath
	content := strings.Join([]string{line1, line2, line3}, "\n")
	return os.WriteFile(nowPlayingFile, []byte(content), 0644)
}

// Run launches a game through MiSTer.
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
