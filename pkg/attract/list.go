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

	// 🔥 reset RAM caches first
	if !quiet {
		fmt.Println("[DEBUG] Resetting in-RAM caches (lists + GameIndex)")
	}
	ResetAll()

	// preload master + gameindex from disk if present
	master, _ := utils.ReadLines(filepath.Join(gamelistDir, "Masterlist.txt"))
	if len(master) > 0 {
		if !quiet {
			fmt.Printf("[DEBUG] Preloaded Masterlist with %d lines\n", len(master))
		}
	} else if !quiet {
		fmt.Printf("[DEBUG] No Masterlist file found, will build new one\n")
	}

	if lines, err := utils.ReadLines(filepath.Join(gamelistDir, "GameIndex")); err == nil {
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
			fmt.Printf("[DEBUG] Preloaded GameIndex with %d entries\n", count)
		}
	} else if !quiet {
		fmt.Printf("[DEBUG] No GameIndex file found, will build new one\n")
	}

	totalGames := 0
	freshCount := 0
	reuseCount := 0

	for _, sp := range systemPaths {
		system := sp.System
		romPath := sp.Path
		if romPath == "" {
			if !quiet {
				fmt.Printf("[DEBUG] Skipping %s (no ROM path)\n", system.Id)
			}
			continue
		}

		// detect if folder has changed
		modified, latestMod, _ := isFolderModified(system.Id, romPath, savedTimestamps)
		if !quiet {
			fmt.Printf("[DEBUG] System %s folder modified=%v latestMod=%s\n",
				system.Id, modified, latestMod.Format(time.RFC3339))
		}

		// build paths
		gamelistPath := filepath.Join(gamelistDir, GamelistFilename(system.Id))

		if modified || !FileExists(gamelistPath) {
			// -------- Fresh system --------
			if !quiet {
				fmt.Printf("[DEBUG] Processing system %s as FRESH (path=%s)\n", system.Id, romPath)
			}

			files, err := games.GetFiles(system.Id, romPath)
			if err != nil {
				fmt.Printf("[ERROR] %s: %v\n", system.Id, err)
				continue
			}
			if len(files) == 0 {
				if !quiet {
					fmt.Printf("[DEBUG] %s had no valid files, skipping\n", system.Id)
				}
				continue
			}
			if !quiet {
				fmt.Printf("[DEBUG] %s initial file scan returned %d files\n", system.Id, len(files))
			}

			// 🔥 Remove old entries first
			beforeMaster := len(master)
			master = removeSystemFromMaster(master, system.Id)
			if !quiet {
				fmt.Printf("[DEBUG] %s Masterlist cleanup %d → %d lines\n",
					system.Id, beforeMaster, len(master))
			}

			beforeIndex := len(GetGameIndex())
			gameIndex := removeSystemFromGameIndex(GetGameIndex(), system.Id)
			ResetAll() // clear again to repopulate clean
			for _, entry := range gameIndex {
				AppendGameIndex(entry)
			}
			if !quiet {
				fmt.Printf("[DEBUG] %s GameIndex cleanup %d → %d entries\n",
					system.Id, beforeIndex, len(gameIndex))
			}

			// Stage 1 Filters
			stage1, c1 := Stage1Filters(files, system.Id, cfg)
			if !quiet {
				fmt.Printf("[DEBUG] %s Stage1 complete: %d files survive (removed %d)\n",
					system.Id, len(stage1), c1["File"])
			}
			master = append(master, "# SYSTEM: "+system.Id)
			master = append(master, stage1...)

			// Stage 2 Filters
			stage2, c2 := Stage2Filters(stage1)
			if !quiet {
				fmt.Printf("[DEBUG] %s Stage2 complete: %d files survive (removed %d)\n",
					system.Id, len(stage2), c2["File"])
			}
			UpdateGameIndex(system.Id, stage2)

			// 🚨 system gamelist write (fresh only)
			_ = WriteLinesIfChanged(gamelistPath, stage2)
			if !quiet {
				fmt.Printf("[DEBUG] %s gamelist written to %s (Stage2 output)\n", system.Id, gamelistPath)
			}

			// Stage 3 Filters
			stage3, c3, _ := Stage3Filters(gamelistDir, system.Id, stage2, cfg)
			if !quiet {
				fmt.Printf("[DEBUG] %s Stage3 complete: %d files survive (removed: white=%d black=%d static=%d folder=%d file=%d)\n",
					system.Id, len(stage3),
					c3["White"], c3["Black"], c3["Static"], c3["Folder"], c3["File"])
			}
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
			if !quiet {
				fmt.Printf("[DEBUG] Processing system %s as REUSED (path=%s)\n", system.Id, romPath)
			}

			if FileExists(gamelistPath) {
				lines, err := utils.ReadLines(gamelistPath)
				if err == nil {
					if !quiet {
						fmt.Printf("[DEBUG] %s reloaded gamelist with %d entries\n", system.Id, len(lines))
					}
					// reuse disk gamelist, reapply filters for cache only
					stage2, c2 := Stage2Filters(lines)
					if !quiet {
						fmt.Printf("[DEBUG] %s Stage2 (reuse) complete: %d files survive (removed %d)\n",
							system.Id, len(stage2), c2["File"])
					}
					stage3, c3, _ := Stage3Filters(gamelistDir, system.Id, stage2, cfg)
					if !quiet {
						fmt.Printf("[DEBUG] %s Stage3 (reuse) complete: %d files survive (removed: white=%d black=%d static=%d folder=%d file=%d)\n",
							system.Id, len(stage3),
							c3["White"], c3["Black"], c3["Static"], c3["Folder"], c3["File"])
					}
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

	// write master + index ONCE at the end
	if !quiet {
		fmt.Printf("[DEBUG] Writing Masterlist with %d lines\n", len(master))
	}
	_ = WriteLinesIfChanged(filepath.Join(gamelistDir, "Masterlist.txt"), master)

	gi := GetGameIndex()
	giLines := []string{}
	for _, entry := range gi {
		giLines = append(giLines, fmt.Sprintf("%s|%s|%s|%s",
			entry.SystemID, entry.Name, entry.Ext, entry.Path))
	}
	if !quiet {
		fmt.Printf("[DEBUG] Writing GameIndex with %d entries\n", len(gi))
	}
	_ = WriteLinesIfChanged(filepath.Join(gamelistDir, "GameIndex"), giLines)

	// save updated timestamps
	if !quiet {
		fmt.Printf("[DEBUG] Saving %d updated timestamps\n", len(newTimestamps))
	}
	_ = saveTimestamps(gamelistDir, newTimestamps)

	if !quiet {
		fmt.Printf("[List] Masterlist contains %d titles\n", CountGames(master))
		fmt.Printf("[List] GameIndex contains %d titles\n", len(gi))
		fmt.Printf("[List] Done in %.1fs (%d fresh, %d reused systems)\n",
			time.Since(start).Seconds(), freshCount, reuseCount)

		// 🚨 explicit banner for reused behavior
		if reuseCount > 0 {
			fmt.Printf("[DEBUG] Skipped disk writes for %d reused systems\n", reuseCount)
		}
	}

	if len(gi) == 0 {
		fmt.Println("[Attract] List build failed: no games indexed")
	}
	return totalGames
}
