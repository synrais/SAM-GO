package attract

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/synrais/SAM-GO/pkg/cache"
	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/games"
	"github.com/synrais/SAM-GO/pkg/input"
	"github.com/synrais/SAM-GO/pkg/utils"
)

// CreateGamelists scans, deduplicates, filters and writes gamelists.
// Called by higher-level packages (e.g. attract) — no CLI logic here.
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

	// load saved modtimes
	savedTimestamps, err := loadSavedTimestamps(gamelistDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[List] Error loading saved timestamps: %v\n", err)
		return 0
	}
	updatedTimestamps := savedTimestamps

	// build system→paths map
	systemPathMap := make(map[string][]string)
	systemOrder := []string{}
	for _, p := range systemPaths {
		if _, ok := systemPathMap[p.System.Id]; !ok {
			systemOrder = append(systemOrder, p.System.Id)
		}
		systemPathMap[p.System.Id] = append(systemPathMap[p.System.Id], p.Path)
	}

	// load master + index
	masterPath := filepath.Join(gamelistDir, "Masterlist.txt")
	indexPath := filepath.Join(gamelistDir, "GameIndex")
	var masterList []string
	if fileExists(masterPath) {
		masterList, _ = utils.ReadLines(masterPath)
	}
	if fileExists(indexPath) {
		data, _ := os.ReadFile(indexPath)
		_ = json.Unmarshal(data, &input.GameIndex)
	}

	// process each system
	for _, systemId := range systemOrder {
		paths := systemPathMap[systemId]
		sysStart := time.Now()

		gamelistPath := filepath.Join(gamelistDir, gamelistFilename(systemId))
		exists := fileExists(gamelistPath)

		var rawFiles []string
		modified := false

		// check timestamps
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

		if modified || !exists {
			// rescan files
			for _, path := range paths {
				files, err := games.GetFiles(systemId, path)
				if err != nil {
					fmt.Fprintf(os.Stderr, "[List] %s\n", err.Error())
					continue
				}
				rawFiles = append(rawFiles, files...)
			}

			// dedup
			deduped := dedupeFiles(rawFiles)

			// disk stage
			beforeDisk := len(deduped)
			diskFiltered := FilterExtensions(deduped, systemId, cfg)
			extRemoved := beforeDisk - len(diskFiltered)

			// cache stage
			cacheFiles, counts, _ := ApplyFilterlistsDetailed(gamelistDir, systemId, diskFiltered, cfg)
			if len(cacheFiles) == 0 {
				emptySystems = append(emptySystems, systemId)
				continue
			}

			sort.Strings(cacheFiles)
			totalGames += len(cacheFiles)

			writeGamelist(gamelistDir, systemId, diskFiltered, cfg.List.RamOnly)

			if exists && !cfg.List.RamOnly {
				rebuilt++
			} else {
				fresh++
			}

			if !quiet {
				fmt.Printf(
					"[List] %-12s Disk: %d → %d (Ext:%d, Dupes:%d) Cache: %d → %d (White:%d, Black:%d, Static:%d, Folder:%d, File:%d) (%.2fs) [%s]\n",
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

			// update masterlist + game index
			masterList = removeSystemBlock(masterList, systemId)
			masterList = append(masterList, "# SYSTEM: "+systemId+" #")
			masterList = append(masterList, rawFiles...)
			updateGameIndex(systemId, deduped)

		} else {
			// reuse cached list
			lines := cache.GetList(gamelistFilename(systemId))
			if len(lines) == 0 {
				lines, _ = utils.ReadLines(gamelistPath)
				cache.SetList(gamelistFilename(systemId), lines)
			}

			beforeDisk := len(lines)
			diskFiltered := FilterExtensions(lines, systemId, cfg)
			extRemoved := beforeDisk - len(diskFiltered)
			cacheFiles, counts, _ := ApplyFilterlistsDetailed(gamelistDir, systemId, diskFiltered, cfg)

			totalGames += len(cacheFiles)
			reused++

			if !quiet {
				fmt.Printf(
					"[List] %-12s Disk: %d → %d (Ext:%d) Cache: %d → %d (White:%d, Black:%d, Static:%d, Folder:%d, File:%d) (%.2fs) [reused]\n",
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

	// save timestamps
	if err := saveTimestamps(gamelistDir, updatedTimestamps); err != nil {
		fmt.Fprintf(os.Stderr, "[List] Failed to save timestamps: %v\n", err)
	}

	// save masterlist + index
	cache.SetList("Masterlist.txt", masterList)
	if !cfg.List.RamOnly {
		writeSimpleList(masterPath, masterList)
		if data, err := json.MarshalIndent(input.GameIndex, "", "  "); err == nil {
			_ = os.WriteFile(indexPath, data, 0644)
		}
	}

	// summary
	if !quiet {
		fmt.Printf("[List] Masterlist contains %d titles\n", countGames(masterList))
		fmt.Printf("[List] GameIndex contains %d titles\n", len(input.GameIndex))

		state := "reused"
		if fresh > 0 || rebuilt > 0 {
			state = "fresh"
		}
		fmt.Printf("[List] %-12s %7d entries [%s]\n", "Masterlist.txt", countGames(masterList), state)
		fmt.Printf("[List] %-12s %7d entries [%s]\n", "GameIndex", len(input.GameIndex), state)

		taken := time.Since(start).Seconds()
		fmt.Printf("[List] Done: %d games in %.1fs (%d fresh, %d rebuilt, %d reused systems)\n",
			totalGames, taken, fresh, rebuilt, reused)
		if len(emptySystems) > 0 {
			fmt.Printf("[List] Empty systems: %s\n", strings.Join(emptySystems, ", "))
		}
	}

	return totalGames
}
