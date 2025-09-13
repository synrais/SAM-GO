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

func writeAmigaVisionLists(gamelistDir string, paths []string) int {
	var gamesList, demosList []string
	written := 0

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
		written += len(gamesList)
	}
	if len(demosList) > 0 {
		writeCustomList(gamelistDir, "AmigaVisionDemos_gamelist.txt", demosList)
		written += len(demosList)
	}

	return written
}

func stripTimestamp(line string) string {
	if strings.HasPrefix(line, "<") {
		if idx := strings.Index(line, ">"); idx > 1 {
			return line[idx+1:]
		}
	}
	return line
}

func buildSearchList(gamelistDir string, overwrite bool, quiet bool) {
	searchPath := filepath.Join(gamelistDir, "Search.txt")
	exists := fileExists(searchPath)

	if exists && !overwrite {
		if !quiet {
			data, _ := os.ReadFile(searchPath)
			lines := parseLines(string(data))
			fmt.Printf("Reusing Search.txt: already exists (%d entries)\n", len(lines))
		}
		return
	}

	tmp, err := os.CreateTemp("", "search-*.txt")
	if err != nil {
		return
	}

	files, err := filepath.Glob(filepath.Join(gamelistDir, "*_gamelist.txt"))
	if err != nil {
		tmp.Close()
		return
	}

	total := 0
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
			_, _ = tmp.WriteString(line + "\n")
			total++
		}
		f.Close()
	}

	_ = tmp.Sync()
	_ = tmp.Close()
	if err := utils.MoveFile(tmp.Name(), searchPath); err == nil && !quiet {
		if exists && overwrite {
			fmt.Printf("Rebuilding Search.txt (%d entries, overwrite enabled)\n", total)
		} else {
			fmt.Printf("Fresh Search.txt created (%d entries)\n", total)
		}
	}
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

	totalPaths := 0
	for _, v := range systemPaths {
		totalPaths += len(v)
	}
	totalSteps := totalPaths
	currentStep := 0

	totalGames := 0
	fresh, rebuilt, reused := 0, 0, 0
	var emptySystems []string

	for systemId, paths := range systemPaths {
		gamelistPath := filepath.Join(gamelistDir, gamelistFilename(systemId))

		_, err := os.Stat(gamelistPath)
		exists := (err == nil)

		if !overwrite && exists {
			if !quiet {
				fmt.Printf("Reusing %s: gamelist already exists\n", systemId)
			}
			reused++

			data, _ := os.ReadFile(gamelistPath)
			lines := parseLines(string(data))
			totalGames += len(lines)

			if !strings.EqualFold(systemId, "Amiga") {
				continue
			}
		} else if overwrite && exists {
			if !quiet {
				fmt.Printf("Rebuilding %s (overwrite enabled)\n", systemId)
			}
		}

		var systemFiles []string
		for _, path := range paths {
			if !quiet {
				if progress {
					fmt.Println("XXX")
					fmt.Println(int(float64(currentStep) / float64(totalSteps) * 100))
					fmt.Printf("Scanning %s (%s)\n", systemId, path)
					fmt.Println("XXX")
				} else {
					fmt.Printf("Scanning %s: %s\n", systemId, path)
				}
			}

			files, err := games.GetFiles(systemId, path)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				continue
			}
			systemFiles = append(systemFiles, files...)

			currentStep++
		}

		if filter {
			systemFiles = filterUniqueWithMGL(systemFiles)
		}

		systemFiles = filterExtensions(systemFiles, systemId, cfg)

		if len(systemFiles) > 0 {
			sort.Strings(systemFiles)
			totalGames += len(systemFiles)
			writeGamelist(gamelistDir, systemId, systemFiles)

			if exists && overwrite {
				rebuilt++
			} else if exists {
				reused++
			} else {
				fresh++
			}
		} else {
			emptySystems = append(emptySystems, systemId)
		}

		if strings.EqualFold(systemId, "Amiga") {
			for visionId, filename := range map[string]string{
				"AmigaVisionGames": "AmigaVisionGames_gamelist.txt",
				"AmigaVisionDemos": "AmigaVisionDemos_gamelist.txt",
			} {
				visionPath := filepath.Join(gamelistDir, filename)
				exists := fileExists(visionPath)

				if overwrite || !exists {
					visionCount := writeAmigaVisionLists(gamelistDir, paths)
					if visionCount > 0 {
						totalGames += visionCount
						if exists && overwrite {
							if !quiet {
								fmt.Printf("Rebuilding %s (overwrite enabled)\n", visionId)
							}
							rebuilt++
						} else {
							if !quiet {
								fmt.Printf("Fresh %s list created (%d entries)\n", visionId, visionCount)
							}
							fresh++
						}
					}
				} else if exists {
					data, _ := os.ReadFile(visionPath)
					lines := parseLines(string(data))
					totalGames += len(lines)
					if !quiet {
						fmt.Printf("Reusing %s: gamelist already exists\n", visionId)
					}
					reused++
				} else {
					emptySystems = append(emptySystems, visionId)
				}
			}
		}
	}

	// build Search.txt (auxiliary index, not counted in summary)
	buildSearchList(gamelistDir, overwrite, quiet)

	// Copy lists into /tmp/.SAM_List
	tmpDir := "/tmp/.SAM_List"
	_ = os.RemoveAll(tmpDir)
	_ = os.MkdirAll(tmpDir, 0755)

	copied := 0
	for systemId := range systemPaths {
		name := gamelistFilename(systemId)
		src := filepath.Join(gamelistDir, name)
		if fileExists(src) {
			dest := filepath.Join(tmpDir, name)
			if err := utils.CopyFile(src, dest); err == nil {
				copied++
			}
		}
	}

	// Copy AmigaVision lists if present
	for _, name := range []string{"AmigaVisionGames_gamelist.txt", "AmigaVisionDemos_gamelist.txt"} {
		src := filepath.Join(gamelistDir, name)
		if fileExists(src) {
			dest := filepath.Join(tmpDir, name)
			if err := utils.CopyFile(src, dest); err == nil {
				copied++
			}
		}
	}

	// Copy Search.txt always
	searchPath := filepath.Join(gamelistDir, "Search.txt")
	if fileExists(searchPath) {
		dest := filepath.Join(tmpDir, "Search.txt")
		if err := utils.CopyFile(searchPath, dest); err == nil {
			copied++
		}
	}

	if !quiet {
		taken := int(time.Since(start).Seconds())
		if progress {
			fmt.Println("XXX")
			fmt.Println("100")
			fmt.Printf("Indexing complete (%d games in %ds)\n", totalGames, taken)
			fmt.Println("XXX")
		} else {
			fmt.Printf("Indexing complete (%d games in %ds)\n", totalGames, taken)
			fmt.Printf("Summary: %d fresh, %d rebuilt, %d reused\n", fresh, rebuilt, reused)

			if len(emptySystems) > 0 {
				fmt.Printf("No games found for: %s\n", strings.Join(emptySystems, ", "))
			}

			if copied > 0 {
				fmt.Printf("%d lists copied to tmp for this session (including Search.txt)\n", copied)
			}
		}
	}

	return totalGames
}

// Entry point
func RunList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)

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
