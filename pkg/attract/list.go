package attract

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/games"
	"github.com/synrais/SAM-GO/pkg/utils"
)

// CreateGamelists builds all gamelists from scratch or reuses existing ones.
//
// Process overview:
//   1. Print startup state (RAM-only vs SD writes).
//   2. Load saved folder timestamps for change detection.
//   3. Build system→paths map.
//   4. Load Masterlist + GameIndex from disk (for reuse).
//   5. For each system:
//        a) Check if system folder modified.
//        b) If modified or missing gamelist → full rebuild (dedupe, filter, update Masterlist/GameIndex).
//        c) Else (unchanged) → reuse gamelist: reload from disk and re-filter into cache.
//   6. Save updated timestamps.
//   7. If any fresh/rebuilt → write Masterlist + GameIndex back to disk.
//   8. Print summary.
//
// Returns the total number of games indexed across all systems.
func CreateGamelists(cfg *config.UserConfig,
	gamelistDir string,
	systemPaths []games.PathResult,
	quiet bool) int {

	start := time.Now()
	if !quiet {
		if cfg.List.RamOnly {
			fmt.Println("[List] Building lists in RAM-only mode (no SD writes)...")
		} else {
			fmt.Println("[List] Scanning system folders...")
		}
	}

	totalGames := 0
	fresh, rebuilt, reused := 0, 0, 0
	var emptySystems []string
	anyRebuilt := false

	// 2. Load saved modtimes for each system folder.
	savedTimestamps, err := loadSavedTimestamps(gamelistDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[List] Error loading saved timestamps: %v\n", err)
		return 0
	}
	updatedTimestamps := savedTimestamps

	// 3. Build system → paths map.
	systemPathMap := make(map[string][]string)
	systemOrder := []string{}
	for _, p := range systemPaths {
		if _, ok := systemPathMap[p.System.Id]; !ok {
			systemOrder = append(systemOrder, p.System.Id)
		}
		systemPathMap[p.System.Id] = append(systemPathMap[p.System.Id], p.Path)
	}

	// 4. Load Masterlist + GameIndex from disk into RAM (for reuse).
	masterPath := filepath.Join(gamelistDir, "Masterlist.txt")
	indexPath := filepath.Join(gamelistDir, "GameIndex")

	var masterList []string
	if fileExists(masterPath) {
		masterList, _ = utils.ReadLines(masterPath)
	}
	if fileExists(indexPath) {
		data, _ := os.ReadFile(indexPath)
		var diskIndex []GameEntry
		if err := json.Unmarshal(data, &diskIndex); err == nil {
			ResetGameIndex()
			for _, e := range diskIndex {
				AppendGameIndex(e)
			}
		}
	}

	// 5. Process each system.
	for _, systemId := range systemOrder {
		paths := systemPathMap[systemId]
		sysStart := time.Now()

		gamelistPath := filepath.Join(gamelistDir, gamelistFilename(systemId))
		exists := fileExists(gamelistPath)

		var rawFiles []string
		modified := false

		// 5a. Check folder modtimes.
		for _, path := range paths {
			m, currentMod, err := isFolderModified(systemId, path, savedTimestamps)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[List] Error checking %s: %v\n", path, err)
				continue
			}
			if m {
				modified = true
				updatedTimestamps = updateTimestamp(updatedTimestamps, systemId, path, currentMod)
			}
		}

		// 5b. Fresh/rebuild branch.
		if modified || !exists {
			anyRebuilt = true

			// Gather raw files from system paths.
			for _, path := range paths {
				files, err := games.GetFiles(systemId, path)
				if err != nil {
					fmt.Fprintf(os.Stderr, "[List] %s\n", err.Error())
					continue
				}
				rawFiles = append(rawFiles, files...)
			}

			// Deduplicate.
			deduped := utils.DedupeFiles(rawFiles)

			// Extension filter.
			beforeDisk := len(deduped)
			diskFiltered := FilterExtensions(deduped, systemId, cfg)
			extRemoved := beforeDisk - len(diskFiltered)

			// Apply filterlists.
			cacheFiles, counts, _ := ApplyFilterlists(gamelistDir, systemId, diskFiltered, cfg)
			if len(cacheFiles) == 0 {
				emptySystems = append(emptySystems, systemId)
				continue
			}

			// Sort cache slice + update total.
			sort.Strings(cacheFiles)
			totalGames += len(cacheFiles)

			// Seed cache slice in RAM.
			SetList(gamelistFilename(systemId), cacheFiles)

			// Write gamelist (unless RAM-only).
			writeGamelist(gamelistDir, systemId, diskFiltered, cfg.List.RamOnly)

			if exists && !cfg.List.RamOnly {
				rebuilt++
			} else {
				fresh++
			}

			// Progress logging.
			if !quiet {
				fmt.Printf(
					"[List] %-12s Disk: %d → %d (Ext:%d, Dupes:%d) Cache: %d → %d "+
						"(White:%d, Black:%d, Static:%d, Folder:%d, File:%d) (%.2fs) [%s]\n",
					systemId,
					len(rawFiles), len(diskFiltered),
					extRemoved, len(rawFiles)-beforeDisk,
					len(diskFiltered), len(cacheFiles),
					counts["White"], counts["Black"], counts["Static"], counts["Folder"], counts["File"],
					time.Since(sysStart).Seconds(),
					func() string {
						if exists && !cfg.List.RamOnly {
							return "rebuilt"
						}
						return "fresh"
					}(),
				)
			}

			// Update Masterlist + GameIndex in RAM.
			masterList = removeSystemBlock(masterList, systemId)
			masterList = append(masterList, "# SYSTEM: "+systemId+" #")
			masterList = append(masterList, rawFiles...)
			updateGameIndex(systemId, deduped)

		} else {
			// 5c. Reuse branch: reload gamelist from disk, re-filter into cache.
			lines, err := utils.ReadLines(gamelistPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[List] Failed to read %s: %v\n", gamelistPath, err)
				continue
			}

			beforeDisk := len(lines)
			diskFiltered := FilterExtensions(lines, systemId, cfg)
			extRemoved := beforeDisk - len(diskFiltered)
			cacheFiles, counts, _ := ApplyFilterlists(gamelistDir, systemId, diskFiltered, cfg)

			// Seed cache slice in RAM.
			SetList(gamelistFilename(systemId), cacheFiles)

			totalGames += len(cacheFiles)
			reused++

			if !quiet {
				fmt.Printf(
					"[List] %-12s Disk: %d → %d (Ext:%d) Cache: %d → %d "+
						"(White:%d, Black:%d, Static:%d, Folder:%d, File:%d) (%.2fs) [reused]\n",
					systemId,
					beforeDisk, len(diskFiltered),
					extRemoved,
					len(diskFiltered), len(cacheFiles),
					counts["White"], counts["Black"], counts["Static"], counts["Folder"], counts["File"],
					time.Since(sysStart).Seconds(),
				)
			}
		}
	}

	// 6. Save updated timestamps.
	if err := saveTimestamps(gamelistDir, updatedTimestamps); err != nil {
		fmt.Fprintf(os.Stderr, "[List] Failed to save timestamps: %v\n", err)
	}

	// 7. Only write Masterlist + GameIndex if rebuild/fresh occurred.
	if anyRebuilt {
		SetList("Masterlist.txt", masterList)
		if !cfg.List.RamOnly {
			writeSimpleList(masterPath, masterList)
			if data, err := json.MarshalIndent(GetGameIndex(), "", "  "); err == nil {
				_ = os.WriteFile(indexPath, data, 0644)
			}
		}
	}

	// 8. Summary.
	if !quiet {
		fmt.Printf("[List] Masterlist contains %d titles\n", countGames(masterList))
		fmt.Printf("[List] GameIndex contains %d titles\n", len(GetGameIndex()))

		state := "reused"
		if fresh > 0 || rebuilt > 0 {
			state = "fresh"
		}
		fmt.Printf("[List] %-12s %7d entries [%s]\n", "Masterlist.txt", countGames(masterList), state)
		fmt.Printf("[List] %-12s %7d entries [%s]\n", "GameIndex", len(GetGameIndex()), state)

		taken := time.Since(start).Seconds()
		fmt.Printf("[List] Done: %d games in %.1fs (%d fresh, %d rebuilt, %d reused systems)\n",
			totalGames, taken, fresh, rebuilt, reused)
		if len(emptySystems) > 0 {
			fmt.Printf("[List] Empty systems: %s\n", strings.Join(emptySystems, ", "))
		}
	}

	return totalGames
}
