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

func writeGamelist(gamelistDir string, systemId string, files []string, ramOnly bool) {
	// Always cache in memory
	cache.SetList(gamelistFilename(systemId), files)

	if ramOnly {
		return
	}

	// Build entire file in RAM
	var sb strings.Builder
	for _, file := range files {
		sb.WriteString(file)
		sb.WriteByte('\n')
	}
	data := []byte(sb.String())

	// Ensure directory exists
	gamelistPath := filepath.Join(gamelistDir, gamelistFilename(systemId))
	if err := os.MkdirAll(filepath.Dir(gamelistPath), 0755); err != nil {
		panic(err)
	}

	// Write once to SD
	if err := os.WriteFile(gamelistPath, data, 0644); err != nil {
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

func writeCustomList(dir, filename string, entries []string, ramOnly bool) {
	// Always cache
	cache.SetList(filename, entries)

	if ramOnly {
		return
	}

	// Build in RAM
	var sb strings.Builder
	for _, e := range entries {
		sb.WriteString(e)
		sb.WriteByte('\n')
	}
	data := []byte(sb.String())

	path := filepath.Join(dir, filename)

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		panic(err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		panic(err)
	}
}

func writeAmigaVisionLists(gamelistDir string, paths []string, ramOnly bool) (int, int) {
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
		writeCustomList(gamelistDir, "AmigaVisionGames_gamelist.txt", gamesList, ramOnly)
	}
	if len(demosList) > 0 {
		writeCustomList(gamelistDir, "AmigaVisionDemos_gamelist.txt", demosList, ramOnly)
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
	overwrite bool) int {

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
		sysStart := time.Now()

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

			if !quiet {
				fmt.Printf("Finished %s in %.2fs (reused %d entries)\n",
					systemId, time.Since(sysStart).Seconds(), len(lines))
			}
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

		// .mgl preference (always applied)
		systemFiles = filterUniqueWithMGL(systemFiles)

		// Extension filtering
		systemFiles = filterExtensions(systemFiles, systemId, cfg)

		// Dedup within system
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

		// Apply staticlist merge
		systemFiles = mergeStaticlist(gamelistDir, systemId, systemFiles, cfg)

		sort.Strings(systemFiles)
		totalGames += len(systemFiles)

		// Write gamelist
		writeGamelist(gamelistDir, systemId, systemFiles, cfg.List.RamOnly)

		if exists && overwrite && !cfg.List.RamOnly {
			rebuilt++
		} else {
			fresh++
		}

		// Update Masterlist + Search
		for _, f := range systemFiles {
			masterlist[systemId] = append(masterlist[systemId], f)

			clean := utils.StripTimestamp(f)
			base := strings.TrimSpace(clean)
			if base != "" {
				globalSearch = append(globalSearch, base)
			}
		}

		if !quiet {
			fmt.Printf("Finished %s in %.2fs (%d entries)\n",
				systemId, time.Since(sysStart).Seconds(), len(systemFiles))
		}
	}

	// Build Search.txt
	if overwrite || fresh > 0 || rebuilt > 0 {
		sort.Strings(globalSearch)
		cache.SetList("Search.txt", globalSearch)

		if !cfg.List.RamOnly {
			var sb strings.Builder
			for _, s := range globalSearch {
				sb.WriteString(s)
				sb.WriteByte('\n')
			}
			data := []byte(sb.String())

			searchPath := filepath.Join(gamelistDir, "Search.txt")
			if err := os.MkdirAll(filepath.Dir(searchPath), 0755); err != nil {
				panic(err)
			}
			if err := os.WriteFile(searchPath, data, 0644); err != nil {
				panic(err)
			}
		}

		if !quiet {
			fmt.Printf("Built Search list with %d entries\n", len(globalSearch))
		}
	}

	// Build Masterlist.txt
	if overwrite || fresh > 0 || rebuilt > 0 {
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

	cfg, _ := config.LoadUserConfig("SAM", &config.UserConfig{})

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

	total := createGamelists(cfg, *gamelistDir, systemPathsMap, *quiet, *overwrite)

	if total == 0 {
		return fmt.Errorf("no games indexed")
	}
	return nil
}
