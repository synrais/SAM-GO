package attract

import (
	"encoding/json"
	"flag"
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

// ---------------------------
// Main createGamelists logic
// ---------------------------

func createGamelists(cfg *config.UserConfig,
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

	savedTimestamps, err := loadSavedTimestamps(gamelistDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[List] Error loading saved timestamps: %v\n", err)
		return 0
	}
	updatedTimestamps := savedTimestamps

	systemPathMap := make(map[string][]string)
	systemOrder := []string{}
	for _, p := range systemPaths {
		if _, ok := systemPathMap[p.System.Id]; !ok {
			systemOrder = append(systemOrder, p.System.Id)
		}
		systemPathMap[p.System.Id] = append(systemPathMap[p.System.Id], p.Path)
	}

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

	for _, systemId := range systemOrder {
		paths := systemPathMap[systemId]

		sysStart := time.Now()
		gamelistPath := filepath.Join(gamelistDir, gamelistFilename(systemId))
		exists := fileExists(gamelistPath)

		var rawFiles []string
		modified := false

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
			for _, path := range paths {
				files, err := games.GetFiles(systemId, path)
				if err != nil {
					fmt.Fprintf(os.Stderr, "[List] %s\n", err.Error())
					continue
				}
				rawFiles = append(rawFiles, files...)
			}

			// Dedup per system
			seen := make(map[string]struct{})
			deduped := make([]string, 0, len(rawFiles))
			for _, f := range rawFiles {
				name, _ := utils.NormalizeEntry(f)
				if _, ok := seen[name]; ok {
					continue
				}
				seen[name] = struct{}{}
				deduped = append(deduped, f)
			}

			// Disk stage
			beforeDisk := len(deduped)
			diskFiltered := FilterExtensions(deduped, systemId, cfg)
			extRemoved := beforeDisk - len(diskFiltered)

			// Cache stage
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

			// Update Masterlist + GameIndex
			masterList = removeSystemBlock(masterList, systemId)
			masterList = append(masterList, "# SYSTEM: "+systemId+" #")
			masterList = append(masterList, rawFiles...)
			newIndex := make([]input.GameEntry, 0, len(input.GameIndex))
			for _, e := range input.GameIndex {
				if !strings.Contains(e.Path, "/"+systemId+"/") {
					newIndex = append(newIndex, e)
				}
			}
			input.GameIndex = newIndex
			seenSearch := make(map[string]struct{})
			for _, f := range deduped {
				name, ext := utils.NormalizeEntry(f)
				if name == "" {
					continue
				}
				if _, ok := seenSearch[name]; ok {
					continue
				}
				input.GameIndex = append(input.GameIndex, input.GameEntry{
					Name: name,
					Ext:  ext,
					Path: f,
				})
				seenSearch[name] = struct{}{}
			}

		} else {
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

	// Save timestamps
	if err := saveTimestamps(gamelistDir, updatedTimestamps); err != nil {
		fmt.Fprintf(os.Stderr, "[List] Failed to save timestamps: %v\n", err)
	}

	// Save Masterlist + GameIndex
	cache.SetList("Masterlist.txt", masterList)
	if !cfg.List.RamOnly {
		writeSimpleList(masterPath, masterList)
		if data, err := json.MarshalIndent(input.GameIndex, "", "  "); err == nil {
			_ = os.WriteFile(indexPath, data, 0644)
		}
	}

	// Summary
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

// ---------------------------
// CLI entry point
// ---------------------------

func RunList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("[List] Failed to detect SAM install path: %w", err)
	}
	baseDir := filepath.Dir(exePath)

	defaultOut := filepath.Join(baseDir, "SAM_Gamelists")
	gamelistDir := fs.String("o", defaultOut, "gamelist files directory")

	filter := fs.String("s", "all", "list of systems to index (comma separated)")
	quiet := fs.Bool("q", false, "suppress all status output")
	detect := fs.Bool("d", false, "list active system folders")
	ramOnly := fs.Bool("ramonly", false, "build lists in RAM only (do not write to SD)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, _ := config.LoadUserConfig("SAM", &config.UserConfig{})

	if *ramOnly {
		cfg.List.RamOnly = true
		if !*quiet {
			fmt.Println("[List] RamOnly mode enabled via CLI")
		}
	}

	var systems []games.System
	if *filter == "all" {
		if len(cfg.List.Exclude) > 0 {
			systems = games.AllSystemsExcept(cfg.List.Exclude)
		} else {
			systems = games.AllSystems()
		}
	} else {
		for _, filterId := range strings.Split(*filter, ",") {
			filterId = strings.TrimSpace(filterId)
			system, err := games.LookupSystem(filterId)
			if err != nil {
				continue
			}
			systems = append(systems, *system)
		}
	}

	if *detect {
		results := games.GetActiveSystemPaths(cfg, systems)
		for _, r := range results {
			fmt.Printf("%s:%s\n", strings.ToLower(r.System.Id), r.Path)
		}
		return nil
	}

	systemPaths := games.GetSystemPaths(cfg, systems)
	total := createGamelists(cfg, *gamelistDir, systemPaths, *quiet)

	if total == 0 {
		return fmt.Errorf("[List] No games indexed")
	}
	return nil
}
