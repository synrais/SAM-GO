package attract

import (
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
	"github.com/synrais/SAM-GO/pkg/utils"
)

// ---------------------------
// Helpers
// ---------------------------

// Gamelist filename for a given system
func gamelistFilename(systemId string) string {
	return systemId + "_gamelist.txt"
}

// Cache the gamelist, then write it to disk (unless ramOnly is true).
func writeGamelist(gamelistDir string, systemId string, files []string, ramOnly bool) {
	cache.SetList(gamelistFilename(systemId), files)
	if ramOnly {
		return
	}

	var sb strings.Builder
	for _, file := range files {
		sb.WriteString(file)
		sb.WriteByte('\n')
	}
	data := []byte(sb.String())

	gamelistPath := filepath.Join(gamelistDir, gamelistFilename(systemId))
	if err := os.MkdirAll(filepath.Dir(gamelistPath), 0755); err != nil {
		panic(err)
	}
	if err := os.WriteFile(gamelistPath, data, 0644); err != nil {
		panic(err)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func writeSimpleList(path string, files []string) {
	var sb strings.Builder
	for _, f := range files {
		sb.WriteString(f)
		sb.WriteByte('\n')
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		panic(err)
	}
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		panic(err)
	}
}

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

	var globalSearch []string
	var masterList []string
	anyRebuilt := false

	// Load saved timestamps
	savedTimestamps, err := loadSavedTimestamps(gamelistDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[List] Error loading saved timestamps: %v\n", err)
		return 0
	}
	updatedTimestamps := savedTimestamps

	// Group paths by system
	systemPathMap := make(map[string][]string)
	systemOrder := []string{} // stable order
	for _, p := range systemPaths {
		if _, ok := systemPathMap[p.System.Id]; !ok {
			systemOrder = append(systemOrder, p.System.Id)
		}
		systemPathMap[p.System.Id] = append(systemPathMap[p.System.Id], p.Path)
	}

	// Process systems in stable order
	for _, systemId := range systemOrder {
		paths := systemPathMap[systemId]

		sysStart := time.Now()
		gamelistPath := filepath.Join(gamelistDir, gamelistFilename(systemId))
		exists := fileExists(gamelistPath)

		var rawFiles []string
		var systemFiles []string
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

		status := ""
		if modified || !exists {
			// Rebuild gamelist
			for _, path := range paths {
				files, err := games.GetFiles(systemId, path)
				if err != nil {
					fmt.Fprintf(os.Stderr, "[List] %s\n", err.Error())
					continue
				}
				rawFiles = append(rawFiles, files...)
			}

			rawCount := len(rawFiles)

			// Dedup per system
			seen := make(map[string]struct{})
			deduped := make([]string, 0, rawCount)
			for _, f := range rawFiles {
				name, _ := utils.NormalizeEntry(f)
				if _, ok := seen[name]; ok {
					continue
				}
				seen[name] = struct{}{}
				deduped = append(deduped, f)
			}
			dedupedCount := len(deduped)

			// Apply filters
			beforeFilter := dedupedCount
			systemFiles = FilterUniqueWithMGL(deduped)
			systemFiles = FilterExtensions(systemFiles, systemId, cfg)
			systemFiles, hadLists := ApplyFilterlists(gamelistDir, systemId, systemFiles, cfg)
			filteredCount := len(systemFiles)
			filtersRemoved := beforeFilter - filteredCount

			if len(systemFiles) == 0 {
				emptySystems = append(emptySystems, systemId)
				continue
			}

			sort.Strings(systemFiles)
			totalGames += len(systemFiles)

			writeGamelist(gamelistDir, systemId, systemFiles, cfg.List.RamOnly)

			if exists && !cfg.List.RamOnly {
				rebuilt++
				anyRebuilt = true
				status = "rebuilt"
			} else {
				fresh++
				status = "fresh"
			}

			if !quiet {
				if hadLists {
					fmt.Printf("[List] %-15s %5d/%-5d → %-5d entries (-%d filtered) (%.2fs) [%s]\n",
						systemId, dedupedCount, rawCount, filteredCount, filtersRemoved,
						time.Since(sysStart).Seconds(), status,
					)
				} else {
					fmt.Printf("[List] %-15s %5d/%-5d → %-5d entries (no filterlists) (%.2fs) [%s]\n",
						systemId, dedupedCount, rawCount, filteredCount,
						time.Since(sysStart).Seconds(), status,
					)
				}
			}

			// Master = raw, Search = deduped
			masterList = append(masterList, rawFiles...)
			seenSearch := make(map[string]struct{})
			for _, f := range deduped {
				name, _ := utils.NormalizeEntry(f)
				if _, ok := seenSearch[name]; !ok {
					globalSearch = append(globalSearch, f)
					seenSearch[name] = struct{}{}
				}
			}

		} else {
			// Reuse cached list
			lines := cache.GetList(gamelistFilename(systemId))
			if len(lines) == 0 {
				lines, _ = utils.ReadLines(gamelistPath)
				cache.SetList(gamelistFilename(systemId), lines)
			}
			systemFiles = lines
			totalGames += len(systemFiles)
			reused++
			status = "reused"

			if !quiet {
				fmt.Printf("[List] %-15s %5d/%-5d → %-5d entries (%.2fs) [%s]\n",
					systemId, len(systemFiles), len(systemFiles), len(systemFiles),
					time.Since(sysStart).Seconds(), status,
				)
			}
		}
	}

	// Save timestamps once at the end
	if err := saveTimestamps(gamelistDir, updatedTimestamps); err != nil {
		fmt.Fprintf(os.Stderr, "[List] Failed to save timestamps: %v\n", err)
	}

	// Write Search.txt
	if fresh > 0 || rebuilt > 0 {
		sort.Strings(globalSearch)
		cache.SetList("Search.txt", globalSearch)
		if !cfg.List.RamOnly {
			writeSimpleList(filepath.Join(gamelistDir, "Search.txt"), globalSearch)
		}
		if !quiet {
			state := "fresh"
			if anyRebuilt {
				state = "rebuilt"
			}
			fmt.Printf("[List] %-12s %7d entries [%s]\n", "Search.txt", len(globalSearch), state)
		}
	} else if !quiet {
		lines := cache.GetList("Search.txt")
		if len(lines) == 0 {
			lines, _ = utils.ReadLines(filepath.Join(gamelistDir, "Search.txt"))
			cache.SetList("Search.txt", lines)
		}
		fmt.Printf("[List] %-12s %7d entries [reused]\n", "Search.txt", len(lines))
	}

	// Write Masterlist.txt
	if fresh > 0 || rebuilt > 0 {
		cache.SetList("Masterlist.txt", masterList)
		if !cfg.List.RamOnly {
			writeSimpleList(filepath.Join(gamelistDir, "Masterlist.txt"), masterList)
		}
		if !quiet {
			state := "fresh"
			if anyRebuilt {
				state = "rebuilt"
			}
			fmt.Printf("[List] %-12s %7d entries [%s]\n", "Masterlist.txt", len(masterList), state)
		}
	} else if !quiet {
		lines := cache.GetList("Masterlist.txt")
		if len(lines) == 0 {
			lines, _ = utils.ReadLines(filepath.Join(gamelistDir, "Masterlist.txt"))
			cache.SetList("Masterlist.txt", lines)
		}
		fmt.Printf("[List] %-12s %7d entries [reused]\n", "Masterlist.txt", len(lines))
	}

	if !quiet {
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
