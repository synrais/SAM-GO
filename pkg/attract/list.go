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
// Timeline overview:
//
// 1. Print startup state (RAM-only vs SD writes).
// 2. Load saved timestamps for change detection.
// 3. Build a system → paths mapping.
// 4. Load masterlist and GameIndex from disk if they exist.
// 5. For each system:
//    a) Check if system folder modified since last scan.
//    b) If modified or no gamelist exists → full rescan:
//       - Gather files from system paths.
//       - Deduplicate.
//       - Apply extension filters.
//       - Apply filterlists (whitelist/blacklist/static).
//       - Write gamelist + update masterlist & GameIndex.
//    c) Else (not modified) → reuse cached list only (never read raw disk files).
// 6. Save updated timestamps.
// 7. Save masterlist + GameIndex back to disk (unless RAM-only).
// 8. Print summary of build results.
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

	// 2. Load saved modtimes for each system folder.
	savedTimestamps, err := loadSavedTimestamps(gamelistDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[List] Error loading saved timestamps: %v\n", err)
		return 0
	}
	updatedTimestamps := savedTimestamps

	// 3. Build system → paths map (some systems may have multiple dirs).
	systemPathMap := make(map[string][]string)
	systemOrder := []string{}
	for _, p := range systemPaths {
		if _, ok := systemPathMap[p.System.Id]; !ok {
			systemOrder = append(systemOrder, p.System.Id)
		}
		systemPathMap[p.System.Id] = append(systemPathMap[p.System.Id], p.Path)
	}

	// 4. Load masterlist and GameIndex if they exist.
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

	// 5. Process each system in order.
	for _, systemId := range systemOrder {
		paths := systemPathMap[systemId]
		sysStart := time.Now()

		gamelistPath := filepath.Join(gamelistDir, gamelistFilename(systemId))
		exists := fileExists(gamelistPath)

		var rawFiles []string
		modified := false

		// 5a. Check timestamps for each system path.
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

		// 5b. If modified or gamelist missing → full rebuild.
		if modified || !exists {
			// Gather raw files from system paths.
			for _, path := range paths {
				files, err := games.GetFiles(systemId, path)
				if err != nil {
					fmt.Fprintf(os.Stderr, "[List] %s\n", err.Error())
					continue
				}
				rawFiles = append(rawFiles, files...)
			}

			// Deduplicate raw file list.
			deduped := utils.DedupeFiles(rawFiles)

			// Extension filter stage.
			beforeDisk := len(deduped)
			diskFiltered := FilterExtensions(deduped, systemId, cfg)
			extRemoved := beforeDisk - len(diskFiltered)

			// Apply filterlists (whitelist, blacklist, static, etc.).
			cacheFiles, counts, _ := ApplyFilterlists(gamelistDir, systemId, diskFiltered, cfg)
			if len(cacheFiles) == 0 {
				emptySystems = append(emptySystems, systemId)
				continue
			}

			// Sort final cache list for consistency.
			sort.Strings(cacheFiles)
			totalGames += len(cacheFiles)

			// Write gamelist to disk (unless RAM-only mode).
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
					-extRemoved, len(rawFiles)-beforeDisk,
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

			// Update masterlist + GameIndex with new results.
			masterList = removeSystemBlock(masterList, systemId)
			masterList = append(masterList, "# SYSTEM: "+systemId+" #")
			masterList = append(masterList, rawFiles...)
			updateGameIndex(systemId, deduped)

		} else {
			// 5c. Reuse cached gamelist.
			// IMPORTANT: no disk fallback here — cache is always the source of truth.
			lines := GetList(gamelistFilename(systemId))

			// Apply extension filter + filterlists to cached list.
			beforeDisk := len(lines)
			diskFiltered := FilterExtensions(lines, systemId, cfg)
			extRemoved := beforeDisk - len(diskFiltered)
			cacheFiles, counts, _ := ApplyFilterlists(gamelistDir, systemId, diskFiltered, cfg)

			totalGames += len(cacheFiles)
			reused++

			if !quiet {
				fmt.Printf(
					"[List] %-12s Disk: %d → %d (Ext:%d) Cache: %d → %d "+
						"(White:%d, Black:%d, Static:%d, Folder:%d, File:%d) (%.2fs) [reused]\n",
					systemId,
					beforeDisk, len(diskFiltered),
					-extRemoved,
					len(diskFiltered), len(cacheFiles),
					counts["White"], counts["Black"], counts["Static"], counts["Folder"], counts["File"],
					time.Since(sysStart).Seconds(),
				)
			}
		}
	}

	// 6. Save updated timestamps for future runs.
	if err := saveTimestamps(gamelistDir, updatedTimestamps); err != nil {
		fmt.Fprintf(os.Stderr, "[List] Failed to save timestamps: %v\n", err)
	}

	// 7. Save masterlist and GameIndex to disk (unless RAM-only).
	SetList("Masterlist.txt", masterList)
	if !cfg.List.RamOnly {
		writeSimpleList(masterPath, masterList)
		if data, err := json.MarshalIndent(GetGameIndex(), "", "  "); err == nil {
			_ = os.WriteFile(indexPath, data, 0644)
		}
	}

	// 8. Print summary and return total game count.
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
