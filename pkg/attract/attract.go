package attract

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/history"
	"github.com/synrais/SAM-GO/pkg/run"
)

// readLines reads all non-empty lines from a file.
func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines, scanner.Err()
}

// writeLines writes lines to a file (overwrites).
func writeLines(path string, lines []string) error {
	tmp, err := os.CreateTemp("", "list-*.txt")
	if err != nil {
		return err
	}
	defer tmp.Close()

	for _, l := range lines {
		_, _ = tmp.WriteString(l + "\n")
	}
	return os.Rename(tmp.Name(), path)
}

// parsePlayTime handles "40" or "40-130"
func parsePlayTime(value string, r *rand.Rand) time.Duration {
	if strings.Contains(value, "-") {
		parts := strings.SplitN(value, "-", 2)
		min, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
		max, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
		if max > min {
			return time.Duration(r.Intn(max-min+1)+min) * time.Second
		}
		return time.Duration(min) * time.Second
	}
	secs, _ := strconv.Atoi(value)
	return time.Duration(secs) * time.Second
}

// matchesPattern checks if string matches a wildcard (*foo*, bar*, *baz)
func matchesPattern(s, pattern string) bool {
	p := strings.ToLower(pattern)
	s = strings.ToLower(s)

	if strings.HasPrefix(p, "*") && strings.HasSuffix(p, "*") {
		return strings.Contains(s, strings.Trim(p, "*"))
	}
	if strings.HasPrefix(p, "*") {
		return strings.HasSuffix(s, strings.TrimPrefix(p, "*"))
	}
	if strings.HasSuffix(p, "*") {
		return strings.HasPrefix(s, strings.TrimSuffix(p, "*"))
	}
	return s == p
}

// disabled checks if a game should be blocked by rules
func disabled(system string, gamePath string, cfg *config.UserConfig) bool {
	rules, ok := cfg.Disable[system]
	if !ok {
		return false
	}

	base := filepath.Base(gamePath)
	ext := filepath.Ext(gamePath)
	dir := filepath.Base(filepath.Dir(gamePath))

	for _, f := range rules.Folders {
		if matchesPattern(dir, f) {
			return true
		}
	}
	for _, f := range rules.Files {
		if matchesPattern(base, f) {
			return true
		}
	}
	for _, e := range rules.Extensions {
		if strings.EqualFold(ext, e) {
			return true
		}
	}
	return false
}

// rebuildLists calls SAM -list to regenerate gamelists.
func rebuildLists(listDir string) []string {
	fmt.Println("All gamelists empty. Rebuilding with SAM -list...")

	exe, _ := os.Executable()
	cmd := exec.Command(exe, "-list")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()

	refreshed, _ := filepath.Glob(filepath.Join(listDir, "*_gamelist.txt"))
	if len(refreshed) > 0 {
		fmt.Printf("Rebuilt %d gamelists, resuming Attract Mode.\n", len(refreshed))
	}
	return refreshed
}

// filterAllowed applies include/exclude restrictions case-insensitively.
func filterAllowed(allFiles []string, include, exclude []string) []string {
	var filtered []string
	for _, f := range allFiles {
		base := strings.TrimSuffix(filepath.Base(f), "_gamelist.txt")
		if len(include) > 0 {
			match := false
			for _, sys := range include {
				if strings.EqualFold(strings.TrimSpace(sys), base) {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}
		skip := false
		for _, sys := range exclude {
			if strings.EqualFold(strings.TrimSpace(sys), base) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		filtered = append(filtered, f)
	}
	return filtered
}

// Run is the entry point for the attract tool.
func Run(_ []string) {
	// Load config
	cfg, _ := config.LoadUserConfig("SAM", &config.UserConfig{})
	attractCfg := cfg.Attract

	listDir := "/tmp/.SAM_List"
	fullDir := "/media/fat/Scripts/.MiSTer_SAM/SAM_Gamelists"

	ProcessLists(listDir, fullDir, cfg)

	// Collect gamelists
	allFiles, err := filepath.Glob(filepath.Join(listDir, "*_gamelist.txt"))
	if err != nil || len(allFiles) == 0 {
		fmt.Println("No gamelists found in", listDir)
		os.Exit(1)
	}

	// Restrict to allowed systems up front
	files := filterAllowed(allFiles, attractCfg.Include, attractCfg.Exclude)
	if len(files) == 0 {
		fmt.Println("No gamelists match Include/Exclude in INI")
		os.Exit(1)
	}

	// Seed random
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	fmt.Println("Attract mode running. Ctrl-C to exit.")

	for {
		if next, ok := history.Next(); ok {
			name := filepath.Base(next)
			name = strings.TrimSuffix(name, filepath.Ext(name))
			fmt.Printf("%s - %s <%s>\n", time.Now().Format("15:04:05"), name, next)
			_ = history.SetNowPlaying(next)
			run.Run([]string{next})
			wait := parsePlayTime(attractCfg.PlayTime, r)
			time.Sleep(wait)
			continue
		}

		// Stop if no files left
		if len(files) == 0 {
			rebuildLists(listDir)
			ProcessLists(listDir, fullDir, cfg)
			allFiles, _ = filepath.Glob(filepath.Join(listDir, "*_gamelist.txt"))
			files = filterAllowed(allFiles, attractCfg.Include, attractCfg.Exclude)
			if len(files) == 0 {
				fmt.Println("Failed to rebuild gamelists, exiting.")
				return
			}
		}

		// Pick a random list
		listFile := files[r.Intn(len(files))]

		// Load lines
		lines, err := readLines(listFile)
		if err != nil || len(lines) == 0 {
			// remove exhausted list
			for i, f := range files {
				if f == listFile {
					files = append(files[:i], files[i+1:]...)
					break
				}
			}
			continue
		}

		// Pick game
		index := 0
		if attractCfg.Random {
			index = r.Intn(len(lines))
		}
		ts, gamePath := ParseLine(lines[index])

		// System from filename
		systemID := strings.TrimSuffix(filepath.Base(listFile), "_gamelist.txt")

		// Apply disable rules
		if disabled(systemID, gamePath, cfg) {
			lines = append(lines[:index], lines[index+1:]...)
			_ = writeLines(listFile, lines)
			continue
		}

		// Display
		name := filepath.Base(gamePath)
		name = strings.TrimSuffix(name, filepath.Ext(name))
		fmt.Printf("%s - %s <%s>\n", time.Now().Format("15:04:05"), name, gamePath)

		// Record current game and launch
		_ = history.WriteNowPlaying(gamePath)
		run.Run([]string{gamePath})

		// Update list
		lines = append(lines[:index], lines[index+1:]...)
		_ = writeLines(listFile, lines)

		// Wait
		wait := parsePlayTime(attractCfg.PlayTime, r)
		time.Sleep(wait)
		if attractCfg.UseStaticlist {
			start := time.Now()
			norm := normalizeName(gamePath)
			deadline := start.Add(wait)
			var skipAt time.Time
			if ts > 0 {
				skipDuration := time.Duration(ts*float64(time.Second)) + time.Duration(attractCfg.SkipafterStatic)*time.Second
				skipAt = start.Add(skipDuration)
			}
			for time.Now().Before(deadline) {
				time.Sleep(time.Second)
				if !skipAt.IsZero() && time.Now().After(skipAt) {
					break
				}
				newTs := ReadStaticTimestamp(fullDir, systemID, norm)
				if newTs > 0 && newTs != ts {
					ts = newTs
					skipDuration := time.Duration(newTs*float64(time.Second)) + time.Duration(attractCfg.SkipafterStatic)*time.Second
					skipAt = start.Add(skipDuration)
					if time.Now().After(skipAt) {
						break
					}
				}
			}
		} else {
			time.Sleep(wait)
		}
	}
}
