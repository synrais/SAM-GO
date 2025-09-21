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
//
//	cfg         - user config
//	gamelistDir - folder for lists
//	systemPaths - results of games discovery
//	quiet       - suppress output if true
func CreateGamelists(cfg *config.UserConfig, gamelistDir string, systemPaths []games.PathResult, quiet bool) int {
	start := time.Now()

	// load saved folder timestamps
	savedTimestamps, _ := loadSavedTimestamps(gamelistDir)
	var newTimestamps []SavedTimestamp

	// reset cache before building
	ResetAll()
	ResetGameIndex()

	totalGames := 0
	freshCount := 0
	rebuildCount := 0
	reuseCount := 0

	rawLists := make(map[string][]string)

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

		// decide action
		action := "reused"
		if modified {
			action = "fresh"
		}

		// build paths
		gamelistPath := filepath.Join(gamelistDir, GamelistFilename(system.Id))

		switch action {
		case "fresh":
			// scan filesystem for games
			files, err := games.GetFiles(system.Id, romPath)
			if err != nil {
				fmt.Printf("[List] %s\n", err.Error())
				continue
			}

			diskLines, cacheLines, counts, _ := BuildSystemLists(gamelistDir, system.Id, files, cfg)

			totalScanned += len(files)
			totalDisk += len(diskLines)
			totalCache += len(cacheLines)

			// write gamelist (if changed)
			_ = WriteLinesIfChanged(gamelistPath, diskLines)

			// seed cache
			SetList(GamelistFilename(system.Id), cacheLines)

			// record raw lines for later master build and index
			rawLists[system.Id] = diskLines

			// update index from disk lines (extension + dedupe only)
			UpdateGameIndex(system.Id, diskLines)

			totalGames += len(cacheLines)
			freshCount++

			if !quiet {
				printListStatus(system.Id, "fresh", len(files), len(files), counts)
			}

			// update timestamp
			newTimestamps = updateTimestamp(newTimestamps, system.Id, romPath, latestMod)

		case "reused":
			// load existing gamelist
			if FileExists(gamelistPath) {
				lines, err := utils.ReadLines(gamelistPath)
				if err == nil {
					diskLines, cacheLines, counts, _ := BuildSystemLists(gamelistDir, system.Id, lines, cfg)

					totalDisk += len(diskLines)
					totalCache += len(cacheLines)
					
					// ensure on-disk list matches normalized disk copy
					_ = WriteLinesIfChanged(gamelistPath, diskLines)

					// seed cache with fully filtered copy
					SetList(GamelistFilename(system.Id), cacheLines)

					// record raw copy for master/index generation
					rawLists[system.Id] = diskLines

					// update index with disk lines
					UpdateGameIndex(system.Id, diskLines)

					totalGames += len(cacheLines)

					if !quiet {
						printListStatus(system.Id, "reused", len(diskLines), len(cacheLines), counts)
					}
				} else {
					fmt.Printf("[WARN] Could not reload gamelist for %s: %v\n", system.Id, err)
				}
			}
			reuseCount++
			// keep old timestamp
			for _, ts := range savedTimestamps {
				if ts.SystemID == system.Id {
					newTimestamps = append(newTimestamps, ts)
				}
			}
		}
	}

	// build masterlist + index
	master := []string{}
	for _, sp := range systemPaths {
		sys := sp.System
		list := rawLists[sys.Id]
		if len(list) == 0 {
			continue
		}
		master = append(master, "# SYSTEM: "+sys.Id)
		master = append(master, list...)
	}

	// write masterlist + index only if changed
	_ = WriteLinesIfChanged(filepath.Join(gamelistDir, "Masterlist.txt"), master)

	gi := GetGameIndex()
	giLines := []string{}
	for _, entry := range gi {
		giLines = append(giLines, fmt.Sprintf("%s|%s|%s|%s",
			entry.SystemID, entry.Name, entry.Ext, entry.Path))
	}
	_ = WriteLinesIfChanged(filepath.Join(gamelistDir, "GameIndex"), giLines)

	// save updated timestamps (only if changed)
	_ = saveTimestamps(gamelistDir, newTimestamps)

	if !quiet {
		fmt.Printf("[List] Masterlist contains %d titles\n", CountGames(master))
		fmt.Printf("[List] GameIndex contains %d titles\n", len(gi))
		fmt.Printf("[List] Totals Scanned:%d Disk:%d Cache:%d\n", totalScanned, totalDisk, totalCache)
		fmt.Printf("[List] Done in %.1fs (%d fresh, %d rebuilt, %d reused systems)\n",
			time.Since(start).Seconds(), freshCount, rebuildCount, reuseCount)
	}

	if len(gi) == 0 {
		fmt.Println("[Attract] List build failed: no games indexed")
	}
	return totalGames
}
