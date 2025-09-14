package attract

import (
	"bufio"
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

func gamelistFilename(systemId string) string {
	return systemId + "_gamelist.txt"
}

func writeGamelist(gamelistDir string, systemId string, files []string) {
	gamelistPath := filepath.Join(gamelistDir, gamelistFilename(systemId))
	tmpPath, err := os.CreateTemp("", "gamelist-*.txt")
	if err != nil {
		panic(err)
	}

	for _, file := range files {
		_, _ = tmpPath.WriteString(file + "\n")
	}
	_ = tmpPath.Sync()
	_ = tmpPath.Close()

	if err := utils.MoveFile(tmpPath.Name(), gamelistPath); err != nil {
		panic(err)
	}
}

func filterUniqueWithMGL(files []string) []string {
	chosen := make(map[string]string)
	for _, f := range files {
		base := strings.TrimSuffix(strings.ToLower(filepath.Base(f)), filepath.Ext(f))
		ext := strings.ToLower(filepath.Ext(f))
		if prev, ok := chosen[base]; ok {
			if strings.HasSuffix(prev, ".mgl") {
				continue
			}
			if ext == ".mgl" {
				chosen[base] = f
			}
		} else {
			chosen[base] = f
		}
	}
	result := []string{}
	for _, v := range chosen {
		result = append(result, v)
	}
	return result
}

func filterExtensions(files []string, systemId string, cfg *config.UserConfig) []string {
	rules, ok := cfg.Disable[systemId]
	if !ok || len(rules.Extensions) == 0 {
		return files
	}

	extMap := make(map[string]struct{})
	for _, e := range rules.Extensions {
		e = strings.ToLower(e)
		if !strings.HasPrefix(e, ".") {
			e = "." + e
		}
		extMap[e] = struct{}{}
	}

	var filtered []string
	for _, f := range files {
		ext := strings.ToLower(filepath.Ext(f))
		if _, skip := extMap[ext]; skip {
			continue
		}
		filtered = append(filtered, f)
	}

	return filtered
}

// ---- Staticlist merge ----
func mergeStaticlist(gamelistDir, systemId string, files []string, cfg *config.UserConfig) []string {
	if !cfg.List.UseStaticlist {
		return files
	}

	// respect include/exclude
	if len(cfg.List.StaticlistInclude) > 0 {
		found := false
		for _, s := range cfg.List.StaticlistInclude {
			if strings.EqualFold(s, systemId) {
				found = true
				break
			}
		}
		if !found {
			return files
		}
	}
	for _, s := range cfg.List.StaticlistExclude {
		if strings.EqualFold(s, systemId) {
			return files
		}
	}

	// load staticlist
	path := filepath.Join(gamelistDir, systemId+"_staticlist.txt")
	f, err := os.Open(path)
	if err != nil {
		return files
	}
	defer f.Close()

	staticMap := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}
		ts := strings.Trim(parts[0], "<>")
		name, _ := utils.NormalizeEntry(parts[1])
		staticMap[name] = ts
	}

	// apply timestamps
	for i, f := range files {
		base := filepath.Base(f)
		name, _ := utils.NormalizeEntry(base)
		if ts, ok := staticMap[name]; ok {
			files[i] = "<" + ts + ">" + f
		}
	}
	return files
}

// ---- AmigaVision helpers ----
func parseLines(data string) []string {
	var out []string
	lines := strings.Split(strings.ReplaceAll(data, "\r\n", "\n"), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func writeCustomList(dir, filename string, entries []string) {
	path := filepath.Join(dir, filename)
	tmp, _ := os.CreateTemp("", "amiga-*.txt")
	for _, e := range entries {
		_, _ = tmp.WriteString(e + "\n")
	}
	tmp.Close()
	_ = utils.MoveFile(tmp.Name(), path)
}

func writeAmigaVisionLists(gamelistDir string, paths []string) (int, int) {
	var gamesList, demosList []string

	for _, path := range paths {
		filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			switch strings.ToLower(d.Name()) {
			case "games.txt":
				data, _ := os.ReadFile(p)
				gamesList = append(gamesList, parseLines(string(data))...)
			case "demos.txt":
				data, _ := os.ReadFile(p)
				demosList = append(demosList, parseLines(string(data))...)
			}
			return nil
		})
	}

	if len(gamesList) > 0 {
		writeCustomList(gamelistDir, "AmigaVisionGames_gamelist.txt", gamesList)
	}
	if len(demosList) > 0 {
		writeCustomList(gamelistDir, "AmigaVisionDemos_gamelist.txt", demosList)
	}

	return len(gamesList), len(demosList)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func createGamelists(cfg *config.UserConfig,
    gamelistDir string,
    systemPaths map[string][]string,
    quiet bool,
    overwrite bool) int

	start := time.Now()
	if !quiet {
		if cfg.List.RamOnly {
			fmt.Println("Building lists in RAM-only mode (no SD writes)...")
		} else {
			fmt.Println("Finding system folders...")
		}
	}

	// Results
	totalGames := 0
	fresh, rebuilt, reused := 0, 0, 0
	var emptySystems []string

	// Accumulators
	var globalSearch []string
	masterlist := make(map[string][]string)

	// Build system gamelists
	for systemId, paths := range systemPaths {
		gamelistPath := filepath.Join(gamelistDir, gamelistFilename(systemId))
		exists := fileExists(gamelistPath)

		if !overwrite && exists && !cfg.List.RamOnly {
			if !quiet {
				fmt.Printf("Reusing %s: gamelist already exists\n", systemId)
			}
			reused++

			// Count entries + push into cache
			lines, _ := utils.ReadLines(gamelistPath)
			totalGames += len(lines)
			cache.SetList(gamelistFilename(systemId), lines)
			continue
		}

		if exists && overwrite && !quiet && !cfg.List.RamOnly {
			fmt.Printf("Rebuilding %s (overwrite enabled)\n", systemId)
		}

		var systemFiles []string
		for _, path := range paths {
			files, err := games.GetFiles(systemId, path)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				continue
			}
			systemFiles = append(systemFiles, files...)
		}

		// .mgl preference
		if filter {
			systemFiles = filterUniqueWithMGL(systemFiles)
		}
		// Extension filtering
		systemFiles = filterExtensions(systemFiles, systemId, cfg)
		// Dedup within system (full filename+ext)
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

		// Apply staticlist merge here
		systemFiles = mergeStaticlist(gamelistDir, systemId, systemFiles, cfg)

		sort.Strings(systemFiles)
		totalGames += len(systemFiles)

		// Only write gamelist if not in RAM-only mode
		if !cfg.List.RamOnly {
			writeGamelist(gamelistDir, systemId, systemFiles)
		}

		// Push gamelist into cache
		cache.SetList(gamelistFilename(systemId), systemFiles)

		if exists && overwrite && !cfg.List.RamOnly {
			rebuilt++
		} else {
			fresh++
		}

		// Update Masterlist + global Search
		for _, f := range systemFiles {
			masterlist[systemId] = append(masterlist[systemId], f)

			// Add to Search.txt (per system deduped already)
			clean := utils.StripTimestamp(f)
			base := strings.TrimSpace(clean)
			if base != "" {
				globalSearch = append(globalSearch, base)
			}
		}
	}

	// Build Search.txt
	if overwrite || fresh > 0 || rebuilt > 0 {
		sort.Strings(globalSearch)
		if !cfg.List.RamOnly {
			searchPath := filepath.Join(gamelistDir, "Search.txt")
			tmp, _ := os.CreateTemp("", "search-*.txt")
			for _, s := range globalSearch {
				_, _ = tmp.WriteString(s + "\n")
			}
			tmp.Close()
			_ = utils.MoveFile(tmp.Name(), searchPath)
		}
		cache.SetList("Search.txt", globalSearch)

		if !quiet {
			fmt.Printf("Built Search list with %d entries\n", len(globalSearch))
		}
	}

	// Build Masterlist.txt
	if overwrite || fresh > 0 || rebuilt > 0 {
		var cacheMaster []string
		if !cfg.List.RamOnly {
			masterPath := filepath.Join(gamelistDir, "Masterlist.txt")
			tmp, _ := os.CreateTemp("", "master-*.txt")
			for system, entries := range masterlist {
				sort.Strings(entries)
				header := fmt.Sprintf("### %s (%d)", system, len(entries))
				_, _ = tmp.WriteString(header + "\n")
				cacheMaster = append(cacheMaster, header)
				for _, e := range entries {
					clean := utils.StripTimestamp(e)
					_, _ = tmp.WriteString(clean + "\n")
					cacheMaster = append(cacheMaster, clean)
				}
				_, _ = tmp.WriteString("\n")
			}
			tmp.Close()
			_ = utils.MoveFile(tmp.Name(), masterPath)
		} else {
			for system, entries := range masterlist {
				sort.Strings(entries)
				header := fmt.Sprintf("### %s (%d)", system, len(entries))
				cacheMaster = append(cacheMaster, header)
				for _, e := range entries {
					clean := utils.StripTimestamp(e)
					cacheMaster = append(cacheMaster, clean)
				}
			}
		}
		cache.SetList("Masterlist.txt", cacheMaster)

		if !quiet {
			fmt.Printf("Built Masterlist with %d systems\n", len(masterlist))
		}
	}

	if !quiet {
		taken := time.Since(start).Seconds()
		fmt.Printf("Indexing complete: %d games in %.1fs (%d fresh, %d rebuilt, %d reused)\n",
			totalGames, taken, fresh, rebuilt, reused)
		if len(emptySystems) > 0 {
			fmt.Printf("No games found for: %s\n", strings.Join(emptySystems, ", "))
		}
	}

	return totalGames
}

// Entry point for this tool when called from SAM
func RunList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)

	// Default gamelist dir now points to SAM_Gamelists
	defaultOut := "/media/fat/Scripts/.MiSTer_SAM/SAM_Gamelists"
	gamelistDir := fs.String("o", defaultOut, "gamelist files directory")

	filter := fs.String("s", "all", "list of systems to index (comma separated)")
	quiet := fs.Bool("q", false, "suppress all status output")
	detect := fs.Bool("d", false, "list active system folders")
	overwrite := fs.Bool("overwrite", false, "overwrite existing gamelists if present")
	ramOnly := fs.Bool("ramonly", false, "build lists in RAM only (do not write to SD)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Load user config (for List options)
	cfg, _ := config.LoadUserConfig("SAM", &config.UserConfig{})

	// CLI flag overrides INI
	if *ramOnly {
		cfg.List.RamOnly = true
		if !*quiet {
			fmt.Println("[LIST] RamOnly mode enabled via CLI")
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

	total := createGamelists(cfg, *gamelistDir, systemPathsMap, *quiet, *overwrite, *overwrite)

	if total == 0 {
		return fmt.Errorf("no games indexed")
	}
	return nil
}
