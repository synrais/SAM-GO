package attract

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/games"
	"github.com/synrais/SAM-GO/pkg/utils"
)

// -----------------------------
// Filters Pipeline
// -----------------------------

// Stage1Filters applies structural filtering:
// - Extension filtering
// Returns: cleaned file list + counts.
func Stage1Filters(files []string, systemID string, cfg *config.UserConfig) ([]string, map[string]int) {
	counts := map[string]int{"File": 0}

	// Filter by extensions
	filtered := FilterExtensions(files, systemID, cfg)

	return filtered, counts
}

// Stage2Filters applies deduplication rules:
// - .mgl precedence
// - Normalized deduplication
// Returns: diskLines + counts.
func Stage2Filters(files []string) ([]string, map[string]int) {
	counts := map[string]int{"File": 0}

	// Dedup .mgl precedence
	beforeMGL := len(files)
	files = FilterUniqueWithMGL(files)
	mglRemoved := beforeMGL - len(files)

	// Dedup normalized names
	beforeDedupe := len(files)
	files = utils.DedupeFiles(files)
	dedupeRemoved := beforeDedupe - len(files)

	counts["File"] = mglRemoved + dedupeRemoved
	return files, counts
}

// Stage3Filters applies semantic filterlists:
// - whitelist
// - blacklist
// - staticlist
// - folder/file rules
// Returns: cacheLines + counts + flag if lists were applied.
func Stage3Filters(gamelistDir, systemID string, diskLines []string, cfg *config.UserConfig) ([]string, map[string]int, bool) {
	return ApplyFilterlists(gamelistDir, systemID, diskLines, cfg)
}

// -----------------------------
// Main entrypoint
// -----------------------------

// CreateGamelists builds all gamelists, masterlist, and game index.
//
// Args:
//	cfg         - user config
//	gamelistDir - folder for lists
//	systemPaths - results of games discovery
//	quiet       - suppress output if true
func CreateGamelists(cfg *config.UserConfig, gamelistDir string, systemPaths []games.PathResult, quiet bool) int {
	start := time.Now()

	// load saved folder timestamps
	savedTimestamps, _ := loadSavedTimestamps(gamelistDir)
	var newTimestamps []SavedTimestamp

	// reset cache before building (lists + index together now)
	ResetAll()

	totalGames := 0
	freshCount := 0
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

			// Stage1: extensions only
			stage1, _ := Stage1Filters(files, system.Id, cfg)

			// Stage2: dedupe rules
			diskLines, counts2 := Stage2Filters(stage1)

			// Write gamelist to disk after Stage2
			_ = WriteLinesIfChanged(gamelistPath, diskLines)

			// Update masterlist in memory
			rawLists[system.Id] = diskLines

			// Update GameIndex from diskLines
			UpdateGameIndex(system.Id, diskLines)

			// Stage3: semantic filterlists
			cacheLines, counts3, _ := Stage3Filters(gamelistDir, system.Id, diskLines, cfg)

			// Seed cache with filtered copy
			SetList(GamelistFilename(system.Id), cacheLines)

			totalGames += len(cacheLines)
			freshCount++

			if !quiet {
				// merge counts
				merged := map[string]int{
					"File":   counts2["File"] + counts3["File"],
					"White":  counts3["White"],
					"Black":  counts3["Black"],
					"Static": counts3["Static"],
					"Folder": counts3["Folder"],
				}
				printListStatus(system.Id, "fresh", len(files), len(cacheLines), merged)
			}

			// update timestamp
			newTimestamps = updateTimestamp(newTimestamps, system.Id, romPath, latestMod)

		case "reused":
			// ensure gamelist file exists
			if FileExists(gamelistPath) {
				lines, err := utils.ReadLines(gamelistPath)
				if err == nil {
					// Stage1
					stage1, _ := Stage1Filters(lines, system.Id, cfg)

					// Stage2
					diskLines, counts2 := Stage2Filters(stage1)

					// Write gamelist to disk (noop if unchanged)
					_ = WriteLinesIfChanged(gamelistPath, diskLines)

					// Stage3
					cacheLines, counts3, _ := Stage3Filters(gamelistDir, system.Id, diskLines, cfg)

					// Seed cache with fully filtered copy
					SetList(GamelistFilename(system.Id), cacheLines)

					// ⚠️ Do NOT update rawLists or GameIndex in reused mode
					// Disk already has Masterlist + GameIndex
					totalGames += len(cacheLines)

					if !quiet {
						merged := map[string]int{
							"File":   counts2["File"] + counts3["File"],
							"White":  counts3["White"],
							"Black":  counts3["Black"],
							"Static": counts3["Static"],
							"Folder": counts3["Folder"],
						}
						printListStatus(system.Id, "reused", len(diskLines), len(cacheLines), merged)
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

	// build masterlist (only if fresh systems appended to rawLists)
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

	// write masterlist if fresh systems contributed
	if len(master) > 0 {
		_ = WriteLinesIfChanged(filepath.Join(gamelistDir, "Masterlist.txt"), master)
		// 🔥 seed into cache
		SetList("Masterlist.txt", master)
	} else {
		// 🔥 reused-only run: reload from disk if exists
		if FileExists(filepath.Join(gamelistDir, "Masterlist.txt")) {
			lines, err := utils.ReadLines(filepath.Join(gamelistDir, "Masterlist.txt"))
			if err == nil {
				SetList("Masterlist.txt", lines)
				master = lines
			}
		}
	}

	// write GameIndex
	gi := GetGameIndex()
	if len(gi) > 0 {
		giLines := []string{}
		for _, entry := range gi {
			giLines = append(giLines, fmt.Sprintf("%s|%s|%s|%s",
				entry.SystemID, entry.Name, entry.Ext, entry.Path))
		}
		_ = WriteLinesIfChanged(filepath.Join(gamelistDir, "GameIndex"), giLines)
	} else {
		// 🔥 reused-only run: reload index from disk if exists
		ReloadGameIndexFromDisk(filepath.Join(gamelistDir, "GameIndex"))
		gi = GetGameIndex()
	}

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
