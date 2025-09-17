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

	var globalSearch []string
	masterlist := make(map[string][]string)

	// Load saved timestamps from the Modtime file
	savedTimestamps, err := loadSavedTimestamps(gamelistDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[List] Error loading saved timestamps: %v\n", err)
		return 0
	}

	// Iterate over each system and process it
	for systemId, paths := range systemPaths {

		// Apply Attract Include/Exclude filters
		if len(cfg.Attract.Include) > 0 {
			allowed := false
			for _, inc := range cfg.Attract.Include {
				if strings.EqualFold(systemId, inc) {
					allowed = true
					break
				}
			}
			if !allowed {
				if !quiet {
					fmt.Printf("[List] Skipping %s (not in Attract Include list)\n", systemId)
				}
				continue
			}
		}
		for _, exc := range cfg.Attract.Exclude {
			if strings.EqualFold(systemId, exc) {
				if !quiet {
					fmt.Printf("[List] Skipping %s (in Attract Exclude list)\n", systemId)
				}
				continue
			}
		}

		sysStart := time.Now()
		gamelistPath := filepath.Join(gamelistDir, gamelistFilename(systemId))
		exists := fileExists(gamelistPath)

		var systemFiles []string

		// Check if the system folder has been modified (timestamp check)
		for _, path := range paths {
			modified, err := checkAndHandleModifiedFolder(systemId, path, gamelistDir, savedTimestamps)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[List] Error checking folder modification: %v\n", err)
				continue
			}

			if modified || !exists {
				// Rebuild the gamelist
				for _, path := range paths {
					files, err := games.GetFiles(systemId, path)
					if err != nil {
						fmt.Fprintln(os.Stderr, "[List] "+err.Error())
						continue
					}
					systemFiles = append(systemFiles, files...)
				}

				// Use the filter functions from filters.go
				systemFiles = FilterUniqueWithMGL(systemFiles)
				systemFiles = FilterExtensions(systemFiles, systemId, cfg)

				// Deduplicate
				seenSys := make(map[string]struct{})
				deduped := systemFiles[:0]
				for _, f := range systemFiles {
					base := strings.ToLower(filepath.Base(f))
					if _, ok := seenSys[base]; ok {
						continue
					}
					seenSys[base] = struct{}{}
					deduped = append(deduped, f)
				}
				systemFiles = deduped

				if len(systemFiles) == 0 {
					emptySystems = append(emptySystems, systemId)
					continue
				}

				// Apply filterlists
				systemFiles = ApplyFilterlists(gamelistDir, systemId, systemFiles, cfg)

				sort.Strings(systemFiles)
				totalGames += len(systemFiles)

				writeGamelist(gamelistDir, systemId, systemFiles, cfg.List.RamOnly)

				if exists && !cfg.List.RamOnly {
					rebuilt++
				} else {
					fresh++
				}
			} else {
				// Reuse
				lines := cache.GetList(gamelistFilename(systemId))
				if len(lines) == 0 {
					lines, _ = utils.ReadLines(gamelistPath)
					cache.SetList(gamelistFilename(systemId), lines)
				}
				totalGames += len(lines)
				reused++
			}
		}

		// Add files to search and masterlist
		for _, f := range systemFiles {
			masterlist[systemId] = append(masterlist[systemId], f)
			clean := utils.StripTimestamp(f)
			base := strings.TrimSpace(clean)
			if base != "" {
				globalSearch = append(globalSearch, base)
			}
		}

		if !quiet {
			fmt.Printf("[List] %s done in %.2fs (%d entries)\n",
				systemId, time.Since(sysStart).Seconds(), len(systemFiles))
		}
	}

	// Build Search.txt
	if fresh > 0 || rebuilt > 0 {
		sort.Strings(globalSearch)
		cache.SetList("Search.txt", globalSearch)
		if !cfg.List.RamOnly {
			var sb strings.Builder
			for _, s := range globalSearch {
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
			fmt.Printf("[List] Built Search list with %d entries\n", len(globalSearch))
		}
	}

	// Build Masterlist
	if fresh > 0 || rebuilt > 0 {
		var cacheMaster []string
		var sb strings.Builder
		for system, entries := range masterlist {
			sort.Strings(entries)
			header := fmt.Sprintf("### %s (%d)", system, len(entries))
			cacheMaster = append(cacheMaster, header)
			sb.WriteString(header + "\n")
			for _, e := range entries {
				clean := utils.StripTimestamp(e)
				cacheMaster = append(cacheMaster, clean)
				sb.WriteString(clean + "\n")
			}
			sb.WriteByte('\n')
		}
		cache.SetList("Masterlist.txt", cacheMaster)
		if !cfg.List.RamOnly {
			masterPath := filepath.Join(gamelistDir, "Masterlist.txt")
			if err := os.MkdirAll(filepath.Dir(masterPath), 0755); err != nil {
				panic(err)
			}
			if err := os.WriteFile(masterPath, []byte(sb.String()), 0644); err != nil {
				panic(err)
			}
		}
		if !quiet {
			fmt.Printf("[List] Built Masterlist with %d systems\n", len(masterlist))
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
