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
					fmt.Printf("[List] %-15s %5d/%-5d → %-5d entries (-%d filtered) (%7.2fs) [%s]\n",
						systemId, dedupedCount, rawCount, filteredCount, filtersRemoved,
						time.Since(sysStart).Seconds(), status,
					)
				} else {
					fmt.Printf("[List] %-15s %5d/%-5d → %-5d entries (no filterlists) (%7.2fs) [%s]\n",
						systemId, dedupedCount, rawCount, filteredCount,
						time.Since(sysStart).Seconds(), status,
					)
				}
			}

			// ** Remove old system entries and add new data for this system**
			masterList = removeSystemEntries(masterList, systemId)
			globalSearch = removeSystemEntries(globalSearch, systemId)

			// Add new system entries
			masterList = append(masterList, "# SYSTEM: "+systemId+" #")
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
				fmt.Printf("[List] %-15s %5d/%-5d → %-5d entries (%7.2fs) [%s]\n",
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

	// --- Masterlist & GameIndex piggyback ---
	masterPath := filepath.Join(gamelistDir, "Masterlist.txt")
	indexPath := filepath.Join(gamelistDir, "GameIndex")

	if fresh > 0 || rebuilt > 0 || reused == 0 || !fileExists(masterPath) || !fileExists(indexPath) {
		fmt.Println("[List] Rebuilding Masterlist and GameIndex from scratch...")

		cache.SetList("Masterlist.txt", masterList)
		if !cfg.List.RamOnly {
			writeSimpleList(masterPath, masterList)
		}
		fmt.Printf("[List] Masterlist contains %d titles\n", len(masterList))

		input.GameIndex = make([]input.GameEntry, 0, len(globalSearch))
		for _, f := range globalSearch {
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
		fmt.Printf("[List] GameIndex contains %d titles\n", len(input.GameIndex))

		if !cfg.List.RamOnly {
			if data, err := json.MarshalIndent(input.GameIndex, "", "  "); err == nil {
				_ = os.WriteFile(indexPath, data, 0644)
			} else {
				fmt.Fprintf(os.Stderr, "[List] Failed to write GameIndex: %v\n", err)
			}
		}

		if !quiet {
			state := "fresh"
			if anyRebuilt {
				state = "rebuilt"
			}
			fmt.Printf("[List] %-12s %7d entries [%s]\n", "Masterlist.txt", len(masterList), state)
			fmt.Printf("[List] %-12s %7d entries [%s]\n", "GameIndex", len(input.GameIndex), state)
		}
	} else {
		// reuse Masterlist + GameIndex
		lines := cache.GetList("Masterlist.txt")
		if len(lines) == 0 {
			lines, _ = utils.ReadLines(masterPath)
			cache.SetList("Masterlist.txt", lines)
		}
		if data, err := os.ReadFile(indexPath); err == nil {
			_ = json.Unmarshal(data, &input.GameIndex)
		}
		if !quiet {
			fmt.Printf("[List] %-12s %7d entries [reused]\n", "Masterlist.txt", len(lines))
			fmt.Printf("[List] %-12s %7d entries [reused]\n", "GameIndex", len(input.GameIndex))
		}
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
