package attract

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/games"
	"github.com/synrais/SAM-GO/pkg/utils"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
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

func stripTimestamp(line string) string {
	if strings.HasPrefix(line, "<") {
		if idx := strings.Index(line, ">"); idx > 1 {
			return line[idx+1:]
		}
	}
	return line
}

func buildSearchList(gamelistDir string) (int, error) {
	searchPath := filepath.Join(gamelistDir, "Search.txt")
	tmp, err := os.CreateTemp("", "search-*.txt")
	if err != nil {
		return 0, err
	}

	files, err := filepath.Glob(filepath.Join(gamelistDir, "*_gamelist.txt"))
	if err != nil {
		tmp.Close()
		return 0, err
	}

	count := 0
	seen := make(map[string]struct{})
	for _, path := range files {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(stripTimestamp(scanner.Text()))
			if line == "" {
				continue
			}
			if _, ok := seen[line]; ok {
				continue
			}
			seen[line] = struct{}{}
			_, _ = tmp.WriteString(line + "\n")
			count++
		}
		f.Close()
	}

	_ = tmp.Sync()
	_ = tmp.Close()
	return count, utils.MoveFile(tmp.Name(), searchPath)
}

func buildMasterList(gamelistDir string) (int, error) {
	masterPath := filepath.Join(gamelistDir, "Masterlist.txt")
	tmp, err := os.CreateTemp("", "master-*.txt")
	if err != nil {
		return 0, err
	}
	defer tmp.Close()

	files, err := filepath.Glob(filepath.Join(gamelistDir, "*_gamelist.txt"))
	if err != nil {
		return 0, err
	}
	sort.Strings(files)

	count := 0
	for _, path := range files {
		system := strings.TrimSuffix(filepath.Base(path), "_gamelist.txt")
		_, _ = tmp.WriteString(fmt.Sprintf("### %s ###\n", system))

		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		lines := parseLines(string(data))
		sort.Strings(lines)
		for _, line := range lines {
			_, _ = tmp.WriteString(line + "\n")
			count++
		}
		_, _ = tmp.WriteString("\n")
	}

	_ = tmp.Sync()
	return count, utils.MoveFile(tmp.Name(), masterPath)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func createGamelists(cfg *config.UserConfig, gamelistDir string, systemPaths map[string][]string,
	progress bool, quiet bool, filter bool, overwrite bool) int {

	start := time.Now()
	if !quiet && !progress {
		fmt.Println("Finding system folders...")
	}

	// Results
	totalGames := 0
	fresh, rebuilt, reused := 0, 0, 0
	var emptySystems []string

	// Accumulators
	globalSeen := make(map[string]struct{}) // dedupe Search.txt globally
	var globalSearch []string
	masterlist := make(map[string][]string)

	// Build system gamelists
	for systemId, paths := range systemPaths {
		gamelistPath := filepath.Join(gamelistDir, gamelistFilename(systemId))
		exists := fileExists(gamelistPath)

		if !overwrite && exists {
			if !quiet {
				fmt.Printf("Reusing %s: gamelist already exists\n", systemId)
			}
			reused++
			// Count entries (for reporting)
			data, _ := os.ReadFile(gamelistPath)
			lines := parseLines(string(data))
			totalGames += len(lines)
			continue
		}

		if exists && overwrite && !quiet {
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
		// Dedup within system
		seenSys := make(map[string]struct{})
		deduped := systemFiles[:0]
		for _, f := range systemFiles {
			base := strings.TrimSuffix(strings.ToLower(filepath.Base(f)), filepath.Ext(f))
			ext := strings.ToLower(filepath.Ext(f))
			key := base + ext
			if _, ok := seenSys[key]; ok {
				continue
			}
			seenSys[key] = struct{}{}
			deduped = append(deduped, f)
		}
		systemFiles = deduped

		if len(systemFiles) == 0 {
			emptySystems = append(emptySystems, systemId)
			continue
		}

		sort.Strings(systemFiles)
		totalGames += len(systemFiles)

		// Write system gamelist
		writeGamelist(gamelistDir, systemId, systemFiles)
		if exists && overwrite {
			rebuilt++
		} else {
			fresh++
		}

		// Update Masterlist + global Search
		for _, f := range systemFiles {
			masterlist[systemId] = append(masterlist[systemId], f)

			// Add to Search.txt (dedupe global)
			line := stripTimestamp(f)
			base := strings.TrimSpace(line)
			if base == "" {
				continue
			}
			if _, ok := globalSeen[base]; !ok {
				globalSeen[base] = struct{}{}
				globalSearch = append(globalSearch, base)
			}
		}
	}

	// Build Search.txt
	if overwrite || fresh > 0 || rebuilt > 0 {
		sort.Strings(globalSearch)
		searchPath := filepath.Join(gamelistDir, "Search.txt")
		tmp, _ := os.CreateTemp("", "search-*.txt")
		for _, s := range globalSearch {
			_, _ = tmp.WriteString(s + "\n")
		}
		tmp.Close()
		_ = utils.MoveFile(tmp.Name(), searchPath)

		if !quiet {
			fmt.Printf("Built Search.txt with %d entries\n", len(globalSearch))
		}
	}

	// Build Masterlist.txt
	if overwrite || fresh > 0 || rebuilt > 0 {
		masterPath := filepath.Join(gamelistDir, "Masterlist.txt")
		tmp, _ := os.CreateTemp("", "master-*.txt")
		for system, entries := range masterlist {
			sort.Strings(entries)
			_, _ = tmp.WriteString(fmt.Sprintf("### %s (%d)\n", system, len(entries)))
			for _, e := range entries {
				_, _ = tmp.WriteString(e + "\n")
			}
			_, _ = tmp.WriteString("\n")
		}
		tmp.Close()
		_ = utils.MoveFile(tmp.Name(), masterPath)

		if !quiet {
			fmt.Printf("Built Masterlist.txt with %d systems\n", len(masterlist))
		}
	}

	// Copy to /tmp/.SAM_List (respect Attract Include/Exclude)
	tmpDir := "/tmp/.SAM_List"
	_ = os.RemoveAll(tmpDir)
	_ = os.MkdirAll(tmpDir, 0755)

	copied := 0
	for systemId := range systemPaths {
		if !allowedFor(systemId, cfg.Attract.Include, cfg.Attract.Exclude) {
			continue
		}
		src := filepath.Join(gamelistDir, gamelistFilename(systemId))
		if fileExists(src) {
			dest := filepath.Join(tmpDir, gamelistFilename(systemId))
			if err := utils.CopyFile(src, dest); err == nil {
				copied++
			}
		}
	}

	// Always copy Search + Masterlist
	for _, name := range []string{"Search.txt", "Masterlist.txt"} {
		src := filepath.Join(gamelistDir, name)
		if fileExists(src) {
			dest := filepath.Join(tmpDir, name)
			if err := utils.CopyFile(src, dest); err == nil {
				copied++
			}
		}
	}

	if !quiet {
		taken := int(time.Since(start).Seconds())
		fmt.Printf("Indexing complete (%d games in %ds)\n", totalGames, taken)
		fmt.Printf("Summary: %d fresh, %d rebuilt, %d reused, %d copied\n",
			fresh, rebuilt, reused, copied)
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
	progress := fs.Bool("p", false, "print output for dialog gauge")
	quiet := fs.Bool("q", false, "suppress all status output")
	detect := fs.Bool("d", false, "list active system folders")
	noDupes := fs.Bool("nodupes", false, "filter out duplicate games")
	overwrite := fs.Bool("overwrite", false, "overwrite existing gamelists if present")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Load user config (for List.Exclude)
	cfg, _ := config.LoadUserConfig("SAM", &config.UserConfig{})

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

	total := createGamelists(cfg, *gamelistDir, systemPathsMap, *progress, *quiet, *noDupes, *overwrite)

	if total == 0 {
		return fmt.Errorf("no games indexed")
	}
	return nil
}
