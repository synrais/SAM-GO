package attract

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/games"
	"github.com/synrais/SAM-GO/pkg/utils"
)

// CreateGamelists builds all gamelists, masterlist, and game index.
//
// Args:
//   cfg         - user config
//   gamelistDir - folder for lists
//   systemPaths - results of games discovery
//   quiet       - suppress output if true
func CreateGamelists(cfg *config.UserConfig, gamelistDir string, systemPaths []games.PathResult, quiet bool) int {
	start := time.Now()

	// load saved folder timestamps
	savedTimestamps, _ := loadSavedTimestamps(gamelistDir)
	var newTimestamps []SavedTimestamp

	// preload master + gameindex from disk if present
	master, _ := utils.ReadLines(filepath.Join(gamelistDir, "Masterlist.txt"))
	gameIndex := []GameEntry{}
	if lines, err := utils.ReadLines(filepath.Join(gamelistDir, "GameIndex")); err == nil {
		for _, l := range lines {
			parts := utils.SplitNTrim(l, "|", 4)
			if len(parts) == 4 {
				gameIndex = append(gameIndex, GameEntry{
					SystemID: parts[0],
					Name:     parts[1],
					Ext:      parts[2],
					Path:     parts[3],
				})
			}
		}
	}

	// reset RAM caches
	ResetAll()

	totalGames := 0
	freshCount := 0
	reuseCount := 0

	for _, sp := range systemPaths {
		system := sp.System
		romPath := sp.Path
		if romPath == "" {
			if !quiet {
				fmt.Printf("[List] %s skipped (no path)\n", system.Id)
			}
			continue
		}

		// detect if folder has changed
		modified, latestMod, _ := isFolderModified(system.Id, romPath, savedTimestamps)

		// build paths
		gamelistPath := filepath.Join(gamelistDir, GamelistFilename(system.Id))

		if modified {
			// -------- Fresh system --------
			files, err := games.GetFiles(system.Id, romPath)
			if err != nil {
				fmt.Printf("[List] %s\n", err.Error())
				continue
			}

			// Stage 1 Filters
			stage1, c1 := Stage1Filters(files, system.Id, cfg)
			master = append(master, "# SYSTEM: "+system.Id)
			master = append(master, stage1...)

			// Stage 2 Filters
			stage2, c2 := Stage2Filters(stage1, system.Id)
			UpdateGameIndex(system.Id, stage2)
			_ = WriteLinesIfChanged(gamelistPath, stage2)

			// Stage 3 Filters
			stage3, c3, _ := Stage3Filters(gamelistDir, system.Id, stage2, cfg)
			SetList(GamelistFilename(system.Id), stage3)

			counts := mergeCounts(c1, c2, c3)
			totalGames += len(stage3)
			freshCount++

			if !quiet {
				printListStatus(system.Id, "fresh", len(files), len(stage3), counts)
			}

			newTimestamps = updateTimestamp(newTimestamps, system.Id, romPath, latestMod)

		} else {
			// -------- Reused system --------
			if FileExists(gamelistPath) {
				lines, err := utils.ReadLines(gamelistPath)
				if err == nil {
					// reuse disk gamelist, reapply filters for cache only
					stage2, c2 := Stage2Filters(lines, system.Id)
					stage3, c3, _ := Stage3Filters(gamelistDir, system.Id, stage2, cfg)
					SetList(GamelistFilename(system.Id), stage3)

					counts := mergeCounts(map[string]int{}, c2, c3)
					totalGames += len(stage3)

					if !quiet {
						printListStatus(system.Id, "reused", len(stage2), len(stage3), counts)
					}
				} else {
					fmt.Printf("[WARN] Could not reload gamelist for %s: %v\n", system.Id, err)
				}
			}
			reuseCount++
			for _, ts := range savedTimestamps {
				if ts.SystemID == system.Id {
					newTimestamps = append(newTimestamps, ts)
				}
			}
		}
	}

	// write master + index
	_ = WriteLinesIfChanged(filepath.Join(gamelistDir, "Masterlist.txt"), master)

	gi := GetGameIndex()
	giLines := []string{}
	for _, entry := range gi {
		giLines = append(giLines, fmt.Sprintf("%s|%s|%s|%s",
			entry.SystemID, entry.Name, entry.Ext, entry.Path))
	}
	_ = WriteLinesIfChanged(filepath.Join(gamelistDir, "GameIndex"), giLines)

	// save updated timestamps
	_ = saveTimestamps(gamelistDir, newTimestamps)

	if !quiet {
		fmt.Printf("[List] Masterlist contains %d titles\n", CountGames(master))
		fmt.Printf("[List] GameIndex contains %d titles\n", len(gi))
		fmt.Printf("[List] Done in %.1fs (%d fresh, %d reused systems)\n",
			time.Since(start).Seconds(), freshCount, reuseCount)
	}

	if len(gi) == 0 {
		fmt.Println("[Attract] List build failed: no games indexed")
	}
	return totalGames
}
