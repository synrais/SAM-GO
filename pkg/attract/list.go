package attract

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/games"
)

// -----------------------------
// Gamelist builder
// -----------------------------

// BuildSystemList builds or reuses a gamelist for one system.
func BuildSystemList(cfg *config.UserConfig, system games.System, forceRebuild bool, ramOnly bool) (int, map[string]int, string, error) {
	start := time.Now()
	counts := map[string]int{"White": 0, "Black": 0, "Static": 0, "Folder": 0, "File": 0}

	// Paths
	systemDir := filepath.Join(cfg.Roms.BaseDir, system.Path)
	gamelistDir := config.GamelistDir()
	gamelistFile := filepath.Join(gamelistDir, GamelistFilename(system.Id))

	// Detect if gamelist already exists
	if !forceRebuild && FileExists(gamelistFile) {
		lines, err := ReadLines(gamelistFile)
		if err != nil {
			return 0, counts, "[error]", fmt.Errorf("read gamelist: %w", err)
		}
		SetList(filepath.Base(gamelistFile), lines)
		UpdateGameIndex(system.Id, lines)
		return len(lines), counts, "[reused]", nil
	}

	// Scan ROMs
	var files []string
	err := filepath.Walk(systemDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return 0, counts, "[error]", fmt.Errorf("scan system: %w", err)
	}

	// Apply filters
	files, counts, _ = ApplyFilterlists(gamelistDir, system.Id, files, cfg)

	// Save to RAM + disk
	SetList(GamelistFilename(system.Id), files)
	if !ramOnly {
		if err := WriteLinesIfChanged(gamelistFile, files); err != nil {
			return 0, counts, "[error]", fmt.Errorf("write gamelist: %w", err)
		}
	}

	// Update GameIndex
	UpdateGameIndex(system.Id, files)

	elapsed := time.Since(start).Seconds()
	return len(files), counts, fmt.Sprintf("[built %.2fs]", elapsed), nil
}

// -----------------------------
// Masterlist + Index rebuild
// -----------------------------

// BuildMasterAndIndex builds Masterlist.txt and GameIndex from all systems.
func BuildMasterAndIndex(gamelistDir string, ramOnly bool) (int, int, error) {
	var master []string
	ResetGameIndex()

	// Walk through cached lists
	for _, key := range ListKeys() {
		lines := GetList(key)
		if len(lines) == 0 {
			continue
		}
		systemID := strings.TrimSuffix(key, "_gamelist.txt")

		// Append system header
		master = append(master, "# SYSTEM: "+systemID)
		master = append(master, lines...)

		// Update GameIndex
		UpdateGameIndex(systemID, lines)
	}

	// Count
	gameCount := CountGames(master)
	indexCount := len(GetGameIndex())

	// Write Masterlist + GameIndex only if changed
	if !ramOnly {
		if err := WriteLinesIfChanged(filepath.Join(gamelistDir, "Masterlist.txt"), master); err != nil {
			return 0, 0, fmt.Errorf("write masterlist: %w", err)
		}
		if err := WriteJSONIfChanged(filepath.Join(gamelistDir, "GameIndex"), GetGameIndex()); err != nil {
			return 0, 0, fmt.Errorf("write gameindex: %w", err)
		}
	}

	return gameCount, indexCount, nil
}
