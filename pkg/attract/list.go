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

// fileExists is a local helper (replacement for utils.FileExists).
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// removeSystemBlock removes the block for a system (separator + entries)
func removeSystemBlock(list []string, systemId string) []string {
	var out []string
	skip := false
	for _, line := range list {
		if strings.HasPrefix(line, "# SYSTEM: ") {
			if strings.Contains(line, systemId) {
				skip = true
				continue
			} else {
				skip = false
			}
		}
		if !skip {
			out = append(out, line)
		}
	}
	return out
}

// countGames ignores system separators when counting titles
func countGames(list []string) int {
	n := 0
	for _, line := range list {
		if strings.HasPrefix(line, "# SYSTEM:") {
			continue
		}
		n++
	}
	return n
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

	// Load existing Masterlist & GameIndex if present
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

			// ✅ Apply filters in correct order
			beforeFilter := dedupedCount
			systemFiles = FilterFoldersAndFiles(deduped, systemId, cfg)
			systemFiles = FilterExtensions(systemFiles, systemId, cfg)
			systemFiles = FilterUniqueWithMGL(systemFiles)
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

			// 🔥 Update Masterlist & GameIndex for this system only
			masterList = removeSystemBlock(masterList, systemId)
			masterList = append(masterList, "# SYSTEM: "+systemId+" #")
			masterList = append(masterList, rawFiles...)

			// Remove system entries from GameIndex
			newIndex := make([]input.GameEntry, 0, len(input.GameIndex))
			for _, e := range input.GameIndex {
				if !strings.Contains(e.Path, "/"+systemId+"/") {
					newIndex = append(newIndex, e)
				}
			}
			input.GameIndex = newIndex

			// Add fresh deduped entries
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

	// Save Masterlist + GameIndex
	cache.SetList("Masterlist.txt", masterList)
	if !cfg.List.RamOnly {
		writeSimpleList(masterPath, masterList)
		if data, err := json.MarshalIndent(input.GameIndex, "", "  "); err == nil {
			_ = os.WriteFile(indexPath, data, 0644)
		}
	}

	// Stats
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
