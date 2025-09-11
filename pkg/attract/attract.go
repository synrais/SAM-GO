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

// appendLine appends a single line to a file.
func appendLine(path string, line string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(line + "\n")
	return err
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
	fmt.Println("⚠️  All gamelists empty. Rebuilding with SAM -list...")

	exe, _ := os.Executable()
	cmd := exec.Command(exe, "-list")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()

	refreshed, _ := filepath.Glob(filepath.Join(listDir, "*_gamelist.txt"))
	if len(refreshed) > 0 {
		fmt.Printf("✓ Rebuilt %d gamelists, resuming Attract Mode.\n", len(refreshed))
	}
	return refreshed
}

// filterAllowed applies system restriction case-insensitively.
func filterAllowed(allFiles []string, systems []string) []string {
	if len(systems) == 0 {
		return allFiles
	}
	var filtered []string
	for _, f := range allFiles {
		base := strings.TrimSuffix(filepath.Base(f), "_gamelist.txt")
		for _, sys := range systems {
			if strings.EqualFold(strings.TrimSpace(sys), base) {
				filtered = append(filtered, f)
				break
			}
		}
	}
	return filtered
}

// Run is the entry point for the attract tool.
func Run(_ []string) {
	// Load config
	cfg, _ := config.LoadUserConfig("SAM", &config.UserConfig{})
	attractCfg := cfg.Attract

	listDir := "/tmp/.SAM_List"
	historyFile := "/tmp/.SAM_History.txt"

	// Collect gamelists
	allFiles, err := filepath.Glob(filepath.Join(listDir, "*_gamelist.txt"))
	if err != nil || len(allFiles) == 0 {
		fmt.Println("No gamelists found in", listDir)
		os.Exit(1)
	}

	// Restrict to allowed systems up front
	files := filterAllowed(allFiles, attractCfg.Systems)
	if len(files) == 0 {
		fmt.Println("No gamelists match Systems in INI")
		os.Exit(1)
	}

	// Seed random
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	fmt.Println("Attract mode running. Ctrl-C to exit.")

	for {
		// Stop if no files left
		if len(files) == 0 {
			files = filterAllowed(rebuildLists(listDir), attractCfg.Systems)
			if len(files) == 0 {
				fmt.Println("❌ Failed to rebuild gamelists, exiting.")
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
		gamePath := lines[index]

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

		// Record current game and core
		gameName := filepath.Base(gamePath)
		content := fmt.Sprintf("[%s] %s", systemID, gameName)
		_ = os.WriteFile("/tmp/Now_Playing.txt", []byte(content), 0644)

		// Launch game
		run.Run([]string{gamePath})

		// Update list
		lines = append(lines[:index], lines[index+1:]...)
		_ = writeLines(listFile, lines)

		// Append history
		_ = appendLine(historyFile, gamePath)

		// Wait
		wait := parsePlayTime(attractCfg.PlayTime, r)
		time.Sleep(wait)
	}
}
