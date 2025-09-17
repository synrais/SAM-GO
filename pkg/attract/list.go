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

// Write the gamelist to disk and cache it
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

// Check if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
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

	var searchList []string   // flat list of deduped entries per system
	var masterList []string   // flat list of all entries across all systems

	// Load saved timestamps
	savedTimestamps, err := loadSavedTimestamps(gamelistDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[List] Error loading saved timestamps: %v\n", err)
		return 0
	}
	updatedTimestamps := savedTimestamps

	// Process systems
	for systemId, paths := range systemPaths {
		sysStart := time.Now()
		gamelistPath := filepath.Join(gamelistDir, gamelistFilename(systemId))
		exists := fileExists(gamelistPath)

		var systemFiles []string
		modified := false

		// Check modification timestamps
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
				systemFiles = append(systemFiles, files...)
			}

			// Apply baseline filters (MGL + extensions)
			systemFiles = FilterUniqueWithMGL(systemFiles)
			systemFiles = FilterExtensions(systemFiles, systemId, cfg)

			if len(systemFiles) == 0 {
				emptySystems = append(emptySystems, systemId)
				continue
			}

			sort.Strings(systemFiles)
			totalGames += len(systemFiles)

			// --- Add to Masterlist + Search ---
			// Masterlist gets everything (raw, not deduped across systems)
			masterList = append(masterList, systemFiles...)

			// Dedup *within this system only* for Search
			seen := make(map[string]struct{})
			for _, f := range systemFiles {
				base := strings.ToLower(filepath.Base(f))
				if _, ok := seen[base]; ok {
					continue
				}
				seen[base] = struct{}{}
				searchList = append(searchList, f)
			}

			// Write gamelist file (deduped systemFiles)
			writeGamelist(gamelistDir, systemId, systemFiles, cfg.List.RamOnly)

			// Apply filterlists to the cached copy only (after writing)
			systemFiles = ApplyFilterlists(gamelistDir, systemId, systemFiles, cfg)
			cache.SetList(gamelistFilename(systemId), systemFiles)

			if exists && !cfg.List.RamOnly {
				rebuilt++
				status = "rebuilt"
			} else {
				fresh++
				status = "fresh"
			}
		} else {
			// Reuse cached list
			lines := cache.GetList(gamelistFilename(systemId))
			if len(lines) == 0 {
				lines, _ = utils.ReadLines(gamelistPath)
				cache.SetList(gamelistFilename(systemId), lines)
			}
			totalGames += len(lines)

			// Masterlist + Search still need to be updated from existing list
			masterList = append(masterList, lines...)

			seen := make(map[string]struct{})
			for _, f := range lines {
				base := strings.ToLower(filepath.Base(f))
				if _, ok := seen[base]; ok {
					continue
				}
				seen[base] = struct{}{}
				searchList = append(searchList, f)
			}

			reused++
			status = "reused"
		}

		if !quiet {
			fmt.Printf("[List] %-12s %5d entries (%.2fs) [%s]\n",
				systemId, len(systemFiles), time.Since(sysStart).Seconds(), status)
		}
	}

	// Save updated timestamps once at the end
	if err := saveTimestamps(gamelistDir, updatedTimestamps); err != nil {
		fmt.Fprintf(os.Stderr, "[List] Failed to save timestamps: %v\n", err)
	}

	// --- Build Search.txt ---
	if fresh > 0 || rebuilt > 0 {
		sort.Strings(searchList)
		cache.SetList("Search.txt", searchList)
		if !cfg.List.RamOnly {
			var sb strings.Builder
			for _, s := range searchList {
				sb.WriteString(s)
				sb.WriteByte('\n')
			}
			searchPath := filepath.Join(gamelistDir, "Search.txt")
			if err := os.MkdirAll(filepath.Dir(searchPath), 0755); err != nil {
				panic(err)
			}
			if err := os.WriteFile(searchPath, []byte(sb.String()), 0644); err != nil {
				panic(err)
			}
		}
		if !quiet {
			fmt.Printf("[List] Built Search list with %d entries\n", len(searchList))
		}
	}

	// --- Build Masterlist.txt ---
	if fresh > 0 || rebuilt > 0 {
		sort.Strings(masterList)
		cache.SetList("Masterlist.txt", masterList)
		if !cfg.List.RamOnly {
			var sb strings.Builder
			for _, m := range masterList {
				sb.WriteString(m)
				sb.WriteByte('\n')
			}
			masterPath := filepath.Join(gamelistDir, "Masterlist.txt")
			if err := os.MkdirAll(filepath.Dir(masterPath), 0755); err != nil {
				panic(err)
			}
			if err := os.WriteFile(masterPath, []byte(sb.String()), 0644); err != nil {
				panic(err)
			}
		}
		if !quiet {
			fmt.Printf("[List] Built Masterlist with %d entries\n", len(masterList))
		}
	}

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
