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
func CreateGamelists(cfg *config.UserConfig, gamelistDir string, systemPaths []games.PathResult, quiet bool) int {
	start := time.Now()

	// load saved folder timestamps
	savedTimestamps, _ := loadSavedTimestamps(gamelistDir)
	var newTimestamps []SavedTimestamp

	// reset RAM caches first
	if !quiet {
		fmt.Println("[DEBUG] global reset → survivors=0 (removed: all)")
	}
	ResetAll()

	// preload master + gameindex from disk if present
	masterPath := filepath.Join(gamelistDir, "Masterlist.txt")
	indexPath := filepath.Join(gamelistDir, "GameIndex")
	masterMissing := false
	indexMissing := false

	master, err := utils.ReadLines(masterPath)
	if err != nil {
		master = []string{}
		masterMissing = true
		if !quiet {
			fmt.Println("[DEBUG] global preload Masterlist → survivors=0 (removed: missing)")
		}
	} else if !quiet {
		fmt.Printf("[DEBUG] global preload Masterlist → survivors=%d (removed: 0)\n", len(master))
	}

	if lines, err := utils.ReadLines(indexPath); err == nil {
		count := 0
		for _, l := range lines {
			parts := SplitNTrim(l, "|", 4)
			if len(parts) == 4 {
				AppendGameIndex(GameEntry{
					SystemID: parts[0],
					Name:     parts[1],
					Ext:      parts[2],
					Path:     parts[3],
				})
				count++
			}
		}
		if !quiet {
			fmt.Printf("[DEBUG] global preload GameIndex → survivors=%d (removed: 0)\n", count)
		}
	} else {
		indexMissing = true
		if !quiet {
			fmt.Println("[DEBUG] global preload GameIndex → survivors=0 (removed: missing)")
		}
	}

	totalGames := 0
	freshCount := 0
	reuseCount := 0

	for _, sp := range systemPaths {
		system := sp.System
		romPath := sp.Path
		if romPath == "" {
			if !quiet {
				fmt.Printf("[DEBUG] %s skipped → survivors=0 (removed: no-path)\n", system.Id)
			}
			continue
		}

		// detect if folder has changed
		modified, latestMod, _ := isFolderModified(system.Id, romPath, savedTimestamps)

		// build paths
		gamelistPath := filepath.Join(gamelistDir, GamelistFilename(system.Id))

		if modified || !FileExists(gamelistPath) {
			// -------- Fresh system --------
			files, err := games.GetFiles(system.Id, romPath)
			if err != nil {
				fmt.Printf("[ERROR] %s scan failed: %v\n", system.Id, err)
				continue
			}
			if len(files) == 0 {
				if !quiet {
					fmt.Printf("[DEBUG] %s skipped → survivors=0 (removed: no-valid-files)\n", system.Id)
				}
				continue
			}

			// remove old entries first
			beforeMaster := len(master)
			master = removeSystemFromMaster(master, system.Id)
			removedMaster := beforeMaster - len(master)
			if !quiet {
				fmt.Printf("[DEBUG] %s cleanup Masterlist → survivors=%d (removed: %d)\n",
					system.Id, len(master), removedMaster)
			}

			beforeIndex := len(GetGameIndex())
			gameIndex := removeSystemFromGameIndex(GetGameIndex(), system.Id)
			ResetAll()
			for _, entry := range gameIndex {
				AppendGameIndex(entry)
			}
			removedIndex := beforeIndex - len(gameIndex)
			if !quiet {
				fmt.Printf("[DEBUG] %s cleanup GameIndex → survivors=%d (removed: %d)\n",
					system.Id, len(gameIndex), removedIndex)
			}

			// Stage 1
			stage1, c1 := Stage1Filters(files, system.Id, cfg)
			if !quiet {
				fmt.Printf("[DEBUG] %s Stage1 → survivors=%d (removed: file=%d)\n",
					system.Id, len(stage1), c1["File"])
			}
			master = append(master, "# SYSTEM: "+system.Id)
			master = append(master, stage1...)

			// Stage 2
			stage2, c2 := Stage2Filters(stage1)
			if !quiet {
				fmt.Printf("[DEBUG] %s Stage2 → survivors=%d (removed: file=%d)\n",
					system.Id, len(stage2), c2["File"])
			}
			UpdateGameIndex(system.Id, stage2)
			_ = WriteLinesIfChanged(gamelistPath, stage2)
			if !quiet {
				fmt.Printf("[DEBUG] %s gamelist write → survivors=%d (removed: 0)\n",
					system.Id, len(stage2))
			}

			// Stage 3
			stage3, c3, _ := Stage3Filters(gamelistDir, system.Id, stage2, cfg)
			if !quiet {
				fmt.Printf("[DEBUG] %s Stage3 → survivors=%d (removed: white=%d black=%d static=%d folder=%d file=%d)\n",
					system.Id, len(stage3),
					c3["White"], c3["Black"], c3["Static"], c3["Folder"], c3["File"])
			}
			SetList(GamelistFilename(system.Id), stage3)

			totalGames += len(stage3)
			freshCount++
			newTimestamps = updateTimestamp(newTimestamps, system.Id, romPath, latestMod)

		} else {
			// -------- Reused system --------
			if FileExists(gamelistPath) {
				lines, err := utils.ReadLines(gamelistPath)
				if err == nil {
					// Stage 2 reuse
					stage2, c2 := Stage2Filters(lines)
					if !quiet {
						fmt.Printf("[DEBUG] %s reuse Stage2 → survivors=%d (removed: file=%d)\n",
							system.Id, len(stage2), c2["File"])
					}
					// Stage 3 reuse
					stage3, c3, _ := Stage3Filters(gamelistDir, system.Id, stage2, cfg)
					if !quiet {
						fmt.Printf("[DEBUG] %s reuse Stage3 → survivors=%d (removed: white=%d black=%d static=%d folder=%d file=%d)\n",
							system.Id, len(stage3),
							c3["White"], c3["Black"], c3["Static"], c3["Folder"], c3["File"])
					}
					SetList(GamelistFilename(system.Id), stage3)
					totalGames += len(stage3)
				} else {
					fmt.Printf("[WARN] %s reuse failed to read gamelist: %v\n", system.Id, err)
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

	// write master + index ONCE at the end if needed
	gi := GetGameIndex()

	if freshCount > 0 || masterMissing || indexMissing {
		if !quiet {
			fmt.Printf("[DEBUG] global write Masterlist → survivors=%d (removed: 0)\n", len(master))
			fmt.Printf("[DEBUG] global write GameIndex → survivors=%d (removed: 0)\n", len(gi))
			fmt.Printf("[DEBUG] global write Timestamps → survivors=%d (removed: 0)\n", len(newTimestamps))
		}
		_ = WriteLinesIfChanged(masterPath, master)

		giLines := []string{}
		for _, entry := range gi {
			giLines = append(giLines, fmt.Sprintf("%s|%s|%s|%s",
				entry.SystemID, entry.Name, entry.Ext, entry.Path))
		}
		_ = WriteLinesIfChanged(indexPath, giLines)
		_ = saveTimestamps(gamelistDir, newTimestamps)
	} else if !quiet {
		fmt.Println("[DEBUG] global write skipped → survivors=0 (removed: no-fresh no-missing)")
	}

	// summary in same format
	if !quiet {
		fmt.Printf("[DEBUG] global summary Masterlist → survivors=%d (removed: 0)\n", CountGames(master))
		fmt.Printf("[DEBUG] global summary GameIndex → survivors=%d (removed: 0)\n", len(gi))
		fmt.Printf("[DEBUG] global summary run → survivors=%d (removed: fresh=%d reused=%d)\n",
			totalGames, freshCount, reuseCount)
		fmt.Printf("[DEBUG] global summary time → survivors=%.1fs (removed: 0)\n",
			time.Since(start).Seconds())
	}

	if len(gi) == 0 {
		fmt.Println("[Attract] List build failed: no games indexed")
	}
	return totalGames
}
