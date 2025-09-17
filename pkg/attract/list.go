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

// Gamelist filename for a given system
func gamelistFilename(systemId string) string {
	return systemId + "_gamelist.txt"
}

// Create gamelists, checking for modifications and handling caching
func createGamelists(cfg *config.UserConfig,
	gamelistDir string,
	systemPaths map[string][]string,
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

	// Load saved timestamps
	savedTimestamps, err := loadSavedTimestamps(gamelistDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[List] Error loading saved timestamps: %v\n", err)
		return 0
	}
	updatedTimestamps := savedTimestamps

	// Process each system
	for systemId, paths := range systemPaths {
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

		status := ""
		if modified || !exists {
			// Step 1: Scan system → raw files
			for _, path := range paths {
				files, err := games.GetFiles(systemId, path)
				if err != nil {
					fmt.Fprintf(os.Stderr, "[List] %s\n", err.Error())
					continue
				}
				rawFiles = append(rawFiles, files...)
			}
			rawCount := len(rawFiles)

			// Step 2: Masterlist = raw
			masterList = append(masterList, rawFiles...)

			// Step 3: Dedup per system
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

			// Write per-system gamelist file (deduped, unfiltered)
			sort.Strings(deduped)
			totalGames += dedupedCount
			writeGamelist(gamelistDir, systemId, deduped, cfg.List.RamOnly)

			// Step 4: Add to search (deduped contribution)
			globalSearch = append(globalSearch, deduped...)

			// Step 5: Apply filters only for cache copy
			filtered := FilterUniqueWithMGL(deduped)
			filtered = FilterExtensions(filtered, systemId, cfg)
			filtered, hadLists := ApplyFilterlists(gamelistDir, systemId, filtered, cfg)
			cache.SetList(gamelistFilename(systemId), filtered)

			if exists && !cfg.List.RamOnly {
				rebuilt++
				status = "rebuilt"
			} else {
				fresh++
				status = "fresh"
			}

			if !quiet {
				if hadLists {
					fmt.Printf("[List] %-12s %5d/%-5d → %-5d entries (filters applied in cache) (%.2fs) [%s]\n",
						systemId, dedupedCount, rawCount, len(filtered),
						time.Since(sysStart).Seconds(), status,
					)
				} else {
					fmt.Printf("[List] %-12s %5d/%-5d → %-5d entries (no filterlists) (%.2fs) [%s]\n",
						systemId, dedupedCount, rawCount, len(filtered),
						time.Since(sysStart).Seconds(), status,
					)
				}
			}

		} else {
			// Reuse cached list (final filtered)
			lines := cache.GetList(gamelistFilename(systemId))
			if len(lines) == 0 {
				lines, _ = utils.ReadLines(gamelistPath)
				cache.SetList(gamelistFilename(systemId), lines)
			}
			totalGames += len(lines)
			reused++
			status = "reused"

			// Masterlist += raw (but here we only have deduped, so append anyway)
			masterList = append(masterList, lines...)
			globalSearch = append(globalSearch, lines...)

			if !quiet {
				fmt.Printf("[List] %-12s %5d/%-5d → %-5d entries (cache reused) (%.2fs) [%s]\n",
					systemId, len(lines), len(lines), len(lines),
					time.Since(sysStart).Seconds(), status,
				)
			}
		}
	}

	// Save updated timestamps
	if err := saveTimestamps(gamelistDir, updatedTimestamps); err != nil {
		fmt.Fprintf(os.Stderr, "[List] Failed to save timestamps: %v\n", err)
	}

	// Write Search.txt (deduped per system, collected globally)
	if fresh > 0 || rebuilt > 0 {
		sort.Strings(globalSearch)
		cache.SetList("Search.txt", globalSearch)
		if !cfg.List.RamOnly {
			writeSimpleList(filepath.Join(gamelistDir, "Search.txt"), globalSearch)
		}
		if !quiet {
			fmt.Printf("[List] %-12s %7d entries [fresh]\n", "Search.txt", len(globalSearch))
		}
	} else if !quiet {
		fmt.Printf("[List] %-12s %7d entries [reused]\n", "Search.txt", len(globalSearch))
	}

	// Write Masterlist.txt (raw, never deduped, never filtered)
	if fresh > 0 || rebuilt > 0 {
		cache.SetList("Masterlist.txt", masterList)
		if !cfg.List.RamOnly {
			writeSimpleList(filepath.Join(gamelistDir, "Masterlist.txt"), masterList)
		}
		if !quiet {
			fmt.Printf("[List] %-12s %7d entries [fresh]\n", "Masterlist.txt", len(masterList))
		}
	} else if !quiet {
		fmt.Printf("[List] %-12s %7d entries [reused]\n", "Masterlist.txt", len(masterList))
	}

	// Summary
	if !quiet {
		taken := time.Since(start).Seconds()
		fmt.Printf("[List] Done: %d games in %.1fs (%d fresh, %d rebuilt, %d reused)\n",
			totalGames, taken, fresh, rebuilt, reused)
		if len(emptySystems) > 0 {
			fmt.Printf("[List] Empty systems: %s\n", strings.Join(emptySystems, ", "))
		}
	}

	return totalGames
}

// helper for writing plain lists
func writeSimpleList(path string, lines []string) {
	var sb strings.Builder
	for _, line := range lines {
		sb.WriteString(line)
		sb.WriteByte('\n')
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		panic(err)
	}
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		panic(err)
	}
}

// Entry point for this tool when called from SAM
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
	systemPathsMap := make(map[string][]string)
	for _, p := range systemPaths {
		systemPathsMap[p.System.Id] = append(systemPathsMap[p.System.Id], p.Path)
	}

	total := createGamelists(cfg, *gamelistDir, systemPathsMap, *quiet)

	if total == 0 {
		return fmt.Errorf("[List] No games indexed")
	}
	return nil
}
