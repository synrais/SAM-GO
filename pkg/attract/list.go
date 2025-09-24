package attract

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/games"
	"github.com/synrais/SAM-GO/pkg/utils"
)

// CreateGamelists builds all gamelists, masterlist, and game index.
func CreateGamelists(cfg *config.UserConfig, gamelistDir string, systemPaths []games.PathResult, quiet bool) int {
	start := time.Now()

	// Ensure gamelistDir exists
	if err := os.MkdirAll(gamelistDir, 0o755); err != nil {
		return 0
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
		fmt.Println("[DEBUG] global reset → clearing in-RAM lists + GameIndex + MasterList")
	}
	ResetAll()

	// preload MasterList
	preStart := time.Now()
	if lines, err := utils.ReadLines(filepath.Join(gamelistDir, "MasterList")); err == nil {
		SetMasterSystem("preload", lines)
		if !quiet {
			fmt.Printf("[DEBUG] preload MasterList → survivors=%d (%.2fs)\n",
				len(lines), time.Since(preStart).Seconds())
		}
	}

	// preload GameIndex
	idxStart := time.Now()
	if lines, err := utils.ReadLines(filepath.Join(gamelistDir, "GameIndex")); err == nil {
		SetIndexSystem("preload", lines)
		if !quiet {
			fmt.Printf("[DEBUG] preload GameIndex → survivors=%d (%.2fs)\n",
				len(lines), time.Since(idxStart).Seconds())
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
			RemoveMasterSystem(system.Id)
			RemoveIndexSystem(system.Id)
			if !quiet {
				fmt.Printf("[DEBUG] %s cleanup complete (%.2fs)\n",
					system.Id, time.Since(cleanStart).Seconds())
			}

			// Stage 1
			s1Start := time.Now()
			stage1, c1 := Stage1Filters(files, system.Id, cfg)
			if !quiet {
				fmt.Printf("[DEBUG] %s Stage1 → survivors=%d (removed=file=%d) (%.2fs)\n",
					system.Id, len(stage1), c1["File"], time.Since(s1Start).Seconds())
			}
			AmendMasterSystem(system.Id, stage1)

			// Stage 2
			s2Start := time.Now()
			stage2, c2 := Stage2Filters(stage1)
			if !quiet {
				fmt.Printf("[DEBUG] %s Stage2 → survivors=%d (removed=file=%d) (%.2fs)\n",
					system.Id, len(stage2), c2["File"], time.Since(s2Start).Seconds())
			}
			AmendIndexSystem(system.Id, stage2)
			_ = WriteLinesIfChanged(gamelistPath, stage2)

			// Stage 3
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
			if !quiet {
				fmt.Printf("[DEBUG] %s REUSED path=%s\n", system.Id, romPath)
			}

			if FileExists(gamelistPath) {
				if len(GetList(GamelistFilename(system.Id))) == 0 {
					if lines, err := utils.ReadLines(gamelistPath); err == nil {
						SetList(GamelistFilename(system.Id), lines)
					}
				}

				stage2, _ := Stage2Filters(GetList(GamelistFilename(system.Id)))
				stage3, _, _ := Stage3Filters(gamelistDir, system.Id, stage2, cfg)
				SetList(GamelistFilename(system.Id), stage3)

				totalGames += len(stage3)
				if !quiet {
					printListStatus(system.Id, "reused", len(stage2), len(stage3), nil)
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

	// 🔥 Apply include/exclude restrictions
	inc, err := ExpandGroups(cfg.Attract.Include)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[Attract] Error expanding include groups: %v\n", err)
		return 0
	}
	exc, err := ExpandGroups(cfg.Attract.Exclude)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[Attract] Error expanding exclude groups: %v\n", err)
		return 0
	}
	allKeys := ListKeys()
	allowed := FilterAllowed(allKeys, inc, exc)

	for _, k := range allKeys {
		systemID := strings.TrimSuffix(k, "_gamelist.txt")
		if !ContainsInsensitive(allowed, systemID) {
			RemoveList(k)
		}
	}

	// global write
	writeStart := time.Now()
	if freshCount > 0 || len(FlattenMaster()) == 0 || len(FlattenIndex()) == 0 {
		_ = WriteLinesIfChanged(filepath.Join(gamelistDir, "MasterList"), FlattenMaster())
		_ = WriteLinesIfChanged(filepath.Join(gamelistDir, "GameIndex"), FlattenIndex())
		_ = saveTimestamps(gamelistDir, newTimestamps)
		if !quiet {
			fmt.Printf("[DEBUG] global write complete (%.2fs)\n", time.Since(writeStart).Seconds())
		}
	}

	if !quiet {
		fmt.Printf("[List] MasterList contains %d titles\n", len(FlattenMaster()))
		fmt.Printf("[List] GameIndex contains %d titles\n", len(FlattenIndex()))
		fmt.Printf("[List] Done in %.1fs (%d fresh, %d reused systems)\n",
			time.Since(start).Seconds(), freshCount, reuseCount)
	}

	if len(FlattenIndex()) == 0 {
		fmt.Println("[Attract] List build failed: no games indexed")
	}
	return totalGames
}
