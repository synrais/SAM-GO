package list

import (
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

// SAM uses slightly different system IDs.
var idMap = map[string]string{
	"Gameboy":        "gb",
	"GameboyColor":   "gbc",
	"GameGear":       "gg",
	"Nintendo64":     "n64",
	"MasterSystem":   "sms",
	"Sega32X":        "s32x",
	"SuperGameboy":   "sgb",
	"TurboGrafx16":   "tgfx16",
	"TurboGrafx16CD": "tgfx16cd",
}

func samId(id string) string {
	if id, ok := idMap[id]; ok {
		return id
	}
	return id
}

func reverseId(id string) string {
	for k, v := range idMap {
		if strings.EqualFold(v, id) {
			return k
		}
	}
	return id
}

func gamelistFilename(systemId string) string {
	if id, ok := idMap[systemId]; ok {
		return strings.ToLower(id) + "_gamelist.txt"
	}
	return strings.ToLower(systemId) + "_gamelist.txt"
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

func writeAmigaVisionLists(gamelistDir string, paths []string) {
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
		writeCustomList(gamelistDir, "amigavisiongames_gamelist.txt", gamesList)
	}
	if len(demosList) > 0 {
		writeCustomList(gamelistDir, "amigavisiondemos_gamelist.txt", demosList)
	}
}

func createGamelists(gamelistDir string, systemPaths map[string][]string, progress bool, quiet bool, filter bool, overwrite bool) int {
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
	for systemId, paths := range systemPaths {
		// --- Skip if gamelist already exists and overwrite=false ---
		gamelistPath := filepath.Join(gamelistDir, gamelistFilename(systemId))
		if !overwrite {
			if _, err := os.Stat(gamelistPath); err == nil {
				if !quiet {
					fmt.Printf("Skipping %s: gamelist already exists\n", systemId)
				}
				continue
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

		// Sort files instantly, so gamelists are always ordered
		if len(systemFiles) > 0 {
			sort.Strings(systemFiles)
			totalGames += len(systemFiles)
			writeGamelist(gamelistDir, systemId, systemFiles)
		}

		if strings.EqualFold(systemId, "Amiga") {
			writeAmigaVisionLists(gamelistDir, paths)
		}
	}

	// Copy all gamelists into /tmp/.SAM_List
	tmpDir := "/tmp/.SAM_List"
	_ = os.RemoveAll(tmpDir)
	_ = os.MkdirAll(tmpDir, 0755)

	filepath.Walk(gamelistDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(info.Name(), "_gamelist.txt") {
			dest := filepath.Join(tmpDir, info.Name())
			_ = utils.CopyFile(path, dest)
		}
		return nil
	})

	if !quiet {
		taken := int(time.Since(start).Seconds())
		if progress {
			fmt.Println("XXX")
			fmt.Println("100")
			fmt.Printf("Indexing complete (%d games in %ds)\n", totalGames, taken)
			fmt.Println("XXX")
		} else {
			fmt.Printf("Indexing complete (%d games in %ds)\n", totalGames, taken)
		}
	}

	return totalGames
}

// Entry point for this tool when called from SAM
func Run(args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	// Default gamelist dir now points to SAM_Gamelists
	defaultOut := "/media/fat/Scripts/.MiSTer_SAM/SAM_Gamelists"
	gamelistDir := fs.String("o", defaultOut, "gamelist files directory")

	filter := fs.String("s", "all", "list of systems to index (comma separated)")
	progress := fs.Bool("p", false, "print output for dialog gauge")
	quiet := fs.Bool("q", false, "suppress all status output")
	detect := fs.Bool("d", false, "list active system folders")
	noDupes := fs.Bool("nodupes", false, "filter out duplicate games")
	overwrite := fs.Bool("overwrite", false, "overwrite existing gamelists if present")

	_ = fs.Parse(args)

	// Load user config (for List.Exclude)
	cfg, _ := config.LoadUserConfig("SAM", &config.UserConfig{})

	var systems []games.System
	if *filter == "all" {
		// Respect [List] Exclude in ini
		if len(cfg.List.Exclude) > 0 {
			systems = games.AllSystemsExcept(cfg.List.Exclude)
		} else {
			systems = games.AllSystems()
		}
	} else {
		for _, filterId := range strings.Split(*filter, ",") {
			filterId = strings.TrimSpace(strings.ToLower(filterId))
			systemId := reverseId(filterId)

			if system, ok := games.Systems[systemId]; ok {
				systems = append(systems, system)
				continue
			}

			system, err := games.LookupSystem(systemId)
			if err != nil {
				continue
			}

			systems = append(systems, *system)
		}
	}

	if *detect {
		results := games.GetActiveSystemPaths(cfg, systems)
		for _, r := range results {
			fmt.Printf("%s:%s\n", strings.ToLower(samId(r.System.Id)), r.Path)
		}
		os.Exit(0)
	}

	systemPaths := games.GetSystemPaths(cfg, systems)
	systemPathsMap := make(map[string][]string)

	for _, p := range systemPaths {
		systemPathsMap[p.System.Id] = append(systemPathsMap[p.System.Id], p.Path)
	}

	total := createGamelists(*gamelistDir, systemPathsMap, *progress, *quiet, *noDupes, *overwrite)

	if total == 0 {
		os.Exit(8)
	} else {
		os.Exit(0)
	}
}
