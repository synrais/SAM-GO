package attract

import (
	"fmt"
	"os"
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

	// reset cache before building
	ResetAll()
	ResetGameIndex()

	// preload existing Masterlist + GameIndex if they exist
	master := []string{}
	masterPath := filepath.Join(gamelistDir, "Masterlist.txt")
	if FileExists(masterPath) {
		if lines, err := utils.ReadLines(masterPath); err == nil {
			master = append(master, lines...)
			SetList("Masterlist.txt", lines)
		}
	}
	giPath := filepath.Join(gamelistDir, "GameIndex")
	if FileExists(giPath) {
		if lines, err := utils.ReadLines(giPath); err == nil {
			for _, line := range lines {
				parts := utils.SplitNTrim(line, "|", 4)
				if len(parts) == 4 {
					entry := GameEntry{
						SystemID: parts[0],
						Name:     parts[1],
						Ext:      parts[2],
						Path:     parts[3],
					}
					AppendGameIndex(entry)
				}
			}
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

		gamelistPath := filepath.Join(gamelistDir, GamelistFilename(system.Id))

		switch action {
		case "fresh":
			// Stage 1 filters
			files, err := games.GetFiles(system.Id, romPath)
			if err != nil {
				fmt.Printf("[List] %s\n", err.Error())
				continue
			}
			stage1, counts1 := Stage1Filters(files, system.Id, cfg)
			if len(stage1) > 0 {
				master = append(master, "# SYSTEM: "+system.Id)
				master = append(master, stage1...)
			}

			// Stage 2 filters
			stage2, counts2 := Stage2Filters(stage1, system.Id)
			_ = WriteLinesIfChanged(gamelistPath, stage2)
			UpdateGameIndex(system.Id, stage2)

			// Stage 3 filters
			stage3, counts3, _ := Stage3Filters(gamelistDir, system.Id, stage2, cfg)
			SetList(GamelistFilename(system.Id), stage3)

			// stats + counts
			totalGames += len(stage3)
			freshCount++
			if !quiet {
				merged := mergeCounts(counts1, counts2, counts3)
				printListStatus(system.Id, "fresh", len(files), len(stage3), merged)
			}

			// update timestamp
			newTimestamps = updateTimestamp(newTimestamps, system.Id, romPath, latestMod)

		case "reused":
			// must have a gamelist on disk
			if FileExists(gamelistPath) {
				lines, err := utils.ReadLines(gamelistPath)
				if err == nil {
					stage1, counts1 := Stage1Filters(lines, system.Id, cfg)
					stage2, counts2 := Stage2Filters(stage1, system.Id)
					_ = WriteLinesIfChanged(gamelistPath, stage2)
					stage3, counts3, _ := Stage3Filters(gamelistDir, system.Id, stage2, cfg)
					SetList(GamelistFilename(system.Id), stage3)

					totalGames += len(stage3)
					if !quiet {
						merged := mergeCounts(counts1, counts2, counts3)
						printListStatus(system.Id, "reused", len(stage2), len(stage3), merged)
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

	// write Masterlist
	_ = WriteLinesIfChanged(masterPath, master)
	if len(master) > 0 {
		SetList("Masterlist.txt", master)
	}

	// write GameIndex
	gi := GetGameIndex()
	giLines := []string{}
	for _, entry := range gi {
		giLines = append(giLines, fmt.Sprintf("%s|%s|%s|%s",
			entry.SystemID, entry.Name, entry.Ext, entry.Path))
	}
	_ = WriteLinesIfChanged(giPath, giLines)

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

// mergeCounts merges three sets of filter counts
func mergeCounts(c1, c2, c3 map[string]int) map[string]int {
	out := map[string]int{}
	for _, c := range []map[string]int{c1, c2, c3} {
		for k, v := range c {
			out[k] += v
		}
	}
	return out
}
