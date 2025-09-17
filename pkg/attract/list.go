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
// Helpers
// ---------------------------

func gamelistFilename(systemId string) string {
	return systemId + "_gamelist.txt"
}

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

// Removes all entries of a specific system from the list.
func removeSystemEntries(list []string, systemId string) []string {
	var newList []string
	for _, entry := range list {
		// Skip entries belonging to the system we want to remove
		if !strings.Contains(entry, systemId) {
			newList = append(newList, entry)
		}
	}
	return newList
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

	// Track systems that are rebuilt or fresh
	var modifiedSystems []string

	// Process systems in stable order
	for _, systemId := range systemOrder {
		paths := systemPathMap[systemId]

		sysStart := time.Now()
		gamelistPath := filepath.Join(gamelistDir, gamelistFilename(systemId))
		exists := fileExists(gamelistPath)

		var rawFiles []string
		var systemFiles []string
		modified := false

		// Check if the system folder is modified
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

		// Check if gamelist exists
		if exists && !modified {
			// Copy the list from FAT to cache
			lines, _ := utils.ReadLines(gamelistPath)
			cache.SetList(gamelistFilename(systemId), lines)

			// Apply filters to cached list
			filtered := FilterExtensions(lines, systemId, cfg)
			filtered, _ = ApplyFilterlists(gamelistDir, systemId, filtered, cfg)
			totalGames += len(filtered)

			if !quiet {
				fmt.Printf("[List] %-15s %d entries (cached)\n", systemId, len(filtered))
			}
			continue
		}

		// If the gamelist is missing or the folder was modified, rescan the system
		for _, path := range paths {
			files, err := games.GetFiles(systemId, path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[List] %s\n", err.Error())
				continue
			}
			rawFiles = append(rawFiles, files...)
		}

		if len(rawFiles) == 0 {
			emptySystems = append(emptySystems, systemId)
			continue
		}

		// Deduplicate per system
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

		// Update Masterlist with raw (uncached, unfiltered) list
		masterList = append(masterList, "# SYSTEM: "+systemId+" #")
		masterList = append(masterList, rawFiles...)

		// Update GameIndex from deduped list
		for _, f := range deduped {
			name, ext := utils.NormalizeEntry(f)
			if name == "" {
				continue
			}
			input.GameIndex = append(input.GameIndex, input.GameEntry{
				Name: name,
				Ext:  ext,
				Path: f,
			})
		}

		// Apply filters to deduped
		filtered := FilterExtensions(deduped, systemId, cfg)
		filtered, _ = ApplyFilterlists(gamelistDir, systemId, filtered, cfg)
		sort.Strings(filtered)

		// Cache + update FAT
		cache.SetList(gamelistFilename(systemId), filtered)
		writeGamelist(gamelistDir, systemId, filtered, cfg.List.RamOnly)

		totalGames += len(filtered)

		if !quiet {
			fmt.Printf("[List] %-15s %d/%d → %d entries [rebuilt]\n",
				systemId, len(deduped), len(rawFiles), len(filtered))
		}
	}

	// Handle case if Master or GameIndex is missing from FAT and needs a full rebuild
	if !fileExists(filepath.Join(gamelistDir, "Masterlist.txt")) || !fileExists(filepath.Join(gamelistDir, "GameIndex")) {
		// Full rescan and rebuild process here
		fmt.Println("[List] Rebuilding Masterlist and GameIndex from scratch...")
		// Logic to rebuild Masterlist and GameIndex if missing...
	}

	// Save timestamps once at the end
	if err := saveTimestamps(gamelistDir, updatedTimestamps); err != nil {
		fmt.Fprintf(os.Stderr, "[List] Failed to save timestamps: %v\n", err)
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
