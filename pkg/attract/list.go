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
func CreateGamelists(cfg *config.UserConfig, gamelistDir string, forceRebuild bool) error {
	start := time.Now()

	// load saved folder timestamps
	savedTimestamps, _ := loadSavedTimestamps(gamelistDir)
	var newTimestamps []SavedTimestamp

	// reset cache before building
	ResetAll()
	ResetGameIndex()

	allSystems := games.AllSystems()
	freshCount := 0
	rebuildCount := 0
	reuseCount := 0

	for _, system := range allSystems {
		romPath := system.Path
		if romPath == "" {
			fmt.Printf("[List] %s skipped (no path)\n", system.Id)
			continue
		}

		// detect if folder has changed
		modified, latestMod, _ := isFolderModified(system.Id, romPath, savedTimestamps)

		// decide action
		action := "reused"
		if forceRebuild {
			action = "rebuilt"
		} else if modified {
			action = "fresh"
		}

		// build paths
		gamelistPath := filepath.Join(gamelistDir, GamelistFilename(system.Id))

		switch action {
		case "fresh", "rebuilt":
			// scan filesystem for games
			files := utils.ScanForGames(romPath)

			// filter extensions
			files = FilterExtensions(files, system.Id, cfg)

			// apply filterlists (whitelist/blacklist/static)
			files, counts, _ := ApplyFilterlists(gamelistDir, system.Id, files, cfg)

			// write gamelist (if changed)
			_ = WriteLinesIfChanged(gamelistPath, files)

			// seed cache
			SetList(GamelistFilename(system.Id), files)

			// update index
			UpdateGameIndex(system.Id, files)

			if action == "fresh" {
				freshCount++
			} else {
				rebuildCount++
			}

			fmt.Printf("[List] %s Disk:%d Cache:%d (White:%d Black:%d Static:%d Folder:%d File:%d) [%s]\n",
				system.Id, len(files), len(files),
				counts["White"], counts["Black"], counts["Static"], counts["Folder"], counts["File"], action)

			// update timestamp
			newTimestamps = updateTimestamp(newTimestamps, system.Id, romPath, latestMod)

		case "reused":
			// load existing gamelist
			if FileExists(gamelistPath) {
				lines, err := utils.ReadLines(gamelistPath)
				if err == nil {
					// apply filterlists to keep consistent
					lines, counts, _ := ApplyFilterlists(gamelistDir, system.Id, lines, cfg)

					// seed cache
					SetList(GamelistFilename(system.Id), lines)

					// update index
					UpdateGameIndex(system.Id, lines)

					fmt.Printf("[List] %s Cache:%d (White:%d Black:%d Static:%d Folder:%d File:%d) [reused]\n",
						system.Id, len(lines),
						counts["White"], counts["Black"], counts["Static"], counts["Folder"], counts["File"])
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
	for _, sys := range allSystems {
		list := GetList(GamelistFilename(sys.Id))
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

	fmt.Printf("[List] Masterlist contains %d titles\n", countGames(master))
	fmt.Printf("[List] GameIndex contains %d titles\n", len(gi))
	fmt.Printf("[List] Done in %.1fs (%d fresh, %d rebuilt, %d reused systems)\n",
		time.Since(start).Seconds(), freshCount, rebuildCount, reuseCount)

	if len(gi) == 0 {
		return fmt.Errorf("[Attract] List build failed: no games indexed")
	}
	return nil
}
