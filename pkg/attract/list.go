package attract

import (
	"fmt"
	"path/filepath"
	"time"
	"os"

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

	// Ensure gamelistDir exists before any file I/O
	if err := os.MkdirAll(gamelistDir, 0o755); err != nil {
    	return 0 // or handle error more aggressively
	}
	
	// load saved folder timestamps
	tsStart := time.Now()
	savedTimestamps, _ := loadSavedTimestamps(gamelistDir)
	if !quiet {
		fmt.Printf("[DEBUG] preload timestamps → survivors=%d (%.2fs)\n",
			len(savedTimestamps), time.Since(tsStart).Seconds())
	}
	var newTimestamps []SavedTimestamp

	// reset RAM caches
	if !quiet {
		fmt.Println("[DEBUG] global reset → clearing in-RAM lists + GameIndex")
	}
	ResetAll()

	// preload master + gameindex from disk if present
	preStart := time.Now()
	master, _ := utils.ReadLines(filepath.Join(gamelistDir, "Masterlist.txt"))
	if !quiet {
		fmt.Printf("[DEBUG] preload Masterlist → survivors=%d (%.2fs)\n",
			len(master), time.Since(preStart).Seconds())
	}

	idxStart := time.Now()
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
			fmt.Printf("[DEBUG] preload GameIndex → survivors=%d (%.2fs)\n",
				count, time.Since(idxStart).Seconds())
		}
	} else if !quiet {
		fmt.Printf("[DEBUG] preload GameIndex → missing (%.2fs)\n", time.Since(idxStart).Seconds())
	}

	totalGames := 0
	freshCount := 0
	reuseCount := 0

	for _, sp := range systemPaths {
		sysStart := time.Now()
		system := sp.System
		romPath := sp.Path
		if romPath == "" {
			if !quiet {
				fmt.Printf("[DEBUG] %s skipped → no ROM path (%.2fs)\n",
					system.Id, time.Since(sysStart).Seconds())
			}
			continue
		}

		// detect if folder has changed
		modStart := time.Now()
		modified, latestMod, _ := isFolderModified(system.Id, romPath, savedTimestamps)
		if !quiet {
			fmt.Printf("[DEBUG] %s modtime check → modified=%v latest=%s (%.2fs)\n",
				system.Id, modified, latestMod.Format(time.RFC3339), time.Since(modStart).Seconds())
		}

		// build gamelist path
		gamelistPath := filepath.Join(gamelistDir, GamelistFilename(system.Id))

		if modified || !FileExists(gamelistPath) {
			// -------- Fresh system --------
			if !quiet {
				fmt.Printf("[DEBUG] %s FRESH scan → path=%s\n", system.Id, romPath)
			}

			scanStart := time.Now()
			files, err := games.GetFiles(system.Id, romPath)
			if err != nil {
				fmt.Printf("[ERROR] %s scan failed → %v\n", system.Id, err)
				continue
			}
			if len(files) == 0 {
				if !quiet {
					fmt.Printf("[DEBUG] %s scan empty → skipping (%.2fs)\n",
						system.Id, time.Since(scanStart).Seconds())
				}
				continue
			}
			if !quiet {
				fmt.Printf("[DEBUG] %s initial scan → survivors=%d (%.2fs)\n",
					system.Id, len(files), time.Since(scanStart).Seconds())
			}

			// cleanup old entries
			cleanStart := time.Now()
			beforeMaster := len(master)
			master = removeSystemFromMaster(master, system.Id)
			beforeIndex := len(GetGameIndex())
			gameIndex := removeSystemFromGameIndex(GetGameIndex(), system.Id)
			ResetAll()
			for _, entry := range gameIndex {
				AppendGameIndex(entry)
			}
			if !quiet {
				fmt.Printf("[DEBUG] %s cleanup → Master:%d→%d GameIndex:%d→%d (%.2fs)\n",
					system.Id, beforeMaster, len(master), beforeIndex, len(gameIndex), time.Since(cleanStart).Seconds())
			}

			// Stage 1
			s1Start := time.Now()
			stage1, c1 := Stage1Filters(files, system.Id, cfg)
			if !quiet {
				fmt.Printf("[DEBUG] %s Stage1 → survivors=%d (removed=file=%d) (%.2fs)\n",
					system.Id, len(stage1), c1["File"], time.Since(s1Start).Seconds())
			}
			master = append(master, "# SYSTEM: "+system.Id)
			master = append(master, stage1...)

			// Stage 2
			s2Start := time.Now()
			stage2, c2 := Stage2Filters(stage1)
			if !quiet {
				fmt.Printf("[DEBUG] %s Stage2 → survivors=%d (removed=file=%d) (%.2fs)\n",
					system.Id, len(stage2), c2["File"], time.Since(s2Start).Seconds())
			}
			UpdateGameIndex(system.Id, stage2)
			_ = WriteLinesIfChanged(gamelistPath, stage2)
			if !quiet {
				fmt.Printf("[DEBUG] %s gamelist write → survivors=%d (%s) (%.2fs)\n",
					system.Id, len(stage2), gamelistPath, time.Since(s2Start).Seconds())
			}

			// Stage 3
			s3Start := time.Now()
			stage3, c3, _ := Stage3Filters(gamelistDir, system.Id, stage2, cfg)
			if !quiet {
				fmt.Printf("[DEBUG] %s Stage3 → survivors=%d (removed:white=%d black=%d static=%d folder=%d file=%d) (%.2fs)\n",
					system.Id, len(stage3),
					c3["White"], c3["Black"], c3["Static"], c3["Folder"], c3["File"],
					time.Since(s3Start).Seconds())
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
				fmt.Printf("[DEBUG] %s REUSED path=%s\n", system.Id, romPath)
			}

			if FileExists(gamelistPath) {
				// preload into RAM if missing
				preloadStart := time.Now()
				if len(GetList(GamelistFilename(system.Id))) == 0 {
					if lines, err := utils.ReadLines(gamelistPath); err == nil {
						SetList(GamelistFilename(system.Id), lines)
						if !quiet {
							fmt.Printf("[DEBUG] %s preload gamelist → survivors=%d (from disk) (%.2fs)\n",
								system.Id, len(lines), time.Since(preloadStart).Seconds())
						}
					}
				}

				// Stage 2
				s2Start := time.Now()
				stage2, c2 := Stage2Filters(GetList(GamelistFilename(system.Id)))
				if !quiet {
					fmt.Printf("[DEBUG] %s reuse Stage2 → survivors=%d (removed=file=%d) (%.2fs)\n",
						system.Id, len(stage2), c2["File"], time.Since(s2Start).Seconds())
				}

				// Stage 3
				s3Start := time.Now()
				stage3, c3, _ := Stage3Filters(gamelistDir, system.Id, stage2, cfg)
				if !quiet {
					fmt.Printf("[DEBUG] %s reuse Stage3 → survivors=%d (removed:white=%d black=%d static=%d folder=%d file=%d) (%.2fs)\n",
						system.Id, len(stage3),
						c3["White"], c3["Black"], c3["Static"], c3["Folder"], c3["File"],
						time.Since(s3Start).Seconds())
				}
				SetList(GamelistFilename(system.Id), stage3)

				counts := mergeCounts(map[string]int{}, c2, c3)
				totalGames += len(stage3)
				if !quiet {
					printListStatus(system.Id, "reused", len(stage2), len(stage3), counts)
				}
			}
			reuseCount++
			for _, ts := range savedTimestamps {
				if ts.SystemID == system.Id {
					newTimestamps = append(newTimestamps, ts)
				}
			}
		}
		if !quiet {
			fmt.Printf("[DEBUG] %s done (%.2fs)\n", system.Id, time.Since(sysStart).Seconds())
		}
	}

	// global write
	writeStart := time.Now()
	if freshCount > 0 || master == nil || GetGameIndex() == nil {
		if !quiet {
			fmt.Printf("[DEBUG] global write Masterlist → survivors=%d\n", len(master))
		}
		_ = WriteLinesIfChanged(filepath.Join(gamelistDir, "Masterlist.txt"), master)

		gi := GetGameIndex()
		giLines := []string{}
		for _, entry := range gi {
			giLines = append(giLines, fmt.Sprintf("%s|%s|%s|%s",
				entry.SystemID, entry.Name, entry.Ext, entry.Path))
		}
		if !quiet {
			fmt.Printf("[DEBUG] global write GameIndex → survivors=%d\n", len(gi))
		}
		_ = WriteLinesIfChanged(filepath.Join(gamelistDir, "GameIndex"), giLines)

		if !quiet {
			fmt.Printf("[DEBUG] global write timestamps → survivors=%d\n", len(newTimestamps))
		}
		_ = saveTimestamps(gamelistDir, newTimestamps)
		if !quiet {
			fmt.Printf("[DEBUG] global write complete (%.2fs)\n", time.Since(writeStart).Seconds())
		}
	} else if !quiet {
		fmt.Printf("[DEBUG] global write skipped → nothing written (%.2fs)\n", time.Since(writeStart).Seconds())
	}

	if !quiet {
		fmt.Printf("[List] Masterlist contains %d titles\n", CountGames(master))
		fmt.Printf("[List] GameIndex contains %d titles\n", len(GetGameIndex()))
		fmt.Printf("[List] Done in %.1fs (%d fresh, %d reused systems)\n",
			time.Since(start).Seconds(), freshCount, reuseCount)
	}

	if len(GetGameIndex()) == 0 {
		fmt.Println("[Attract] List build failed: no games indexed")
	}
	return totalGames
}
