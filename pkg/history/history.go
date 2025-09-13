package history

import (
	"bufio"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/synrais/SAM-GO/pkg/config"
)

const (
	nowPlayingFile = "/tmp/Now_Playing.txt"
	historyFile    = "/tmp/.SAM_List/History.txt"
)

func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines, sc.Err()
}

func writeLines(path string, lines []string) error {
	_ = os.MkdirAll(filepath.Dir(path), 0777)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, l := range lines {
		if _, err := f.WriteString(l + "\n"); err != nil {
			return err
		}
	}
	return nil
}

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

// WriteNowPlaying appends a new game to history if it's not already present.
func WriteNowPlaying(path string) error {
	_ = os.MkdirAll(filepath.Dir(historyFile), 0777)
	if err := os.WriteFile(nowPlayingFile, []byte(path), 0644); err != nil {
		return err
	}
	hist, _ := readLines(historyFile)
	if indexOf(hist, path) == -1 {
		hist = append(hist, path)
	}
	return writeLines(historyFile, hist)
}

// updateNowPlaying only updates the Now_Playing file (no history mutation).
func updateNowPlaying(path string) error {
	_ = os.MkdirAll(filepath.Dir(historyFile), 0777)
	return os.WriteFile(nowPlayingFile, []byte(path), 0644)
}

// SetNowPlaying updates the Now_Playing file without modifying history order.
func SetNowPlaying(path string) error {
	_ = os.MkdirAll(filepath.Dir(historyFile), 0777)
	return os.WriteFile(nowPlayingFile, []byte(path), 0644)
}

func readNowPlaying() (string, error) {
	b, err := os.ReadFile(nowPlayingFile)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func indexOf(slice []string, val string) int {
	for i, v := range slice {
		if v == val {
			return i
		}
	}
	return -1
}

// Next returns the next history entry after the current Now_Playing.
func Next() (string, bool) {
	hist, err := readLines(historyFile)
	if err != nil || len(hist) == 0 {
		return "", false
	}
	cur, err := readNowPlaying()
	if err != nil {
		return "", false
	}
	idx := indexOf(hist, cur)
	if idx >= 0 && idx < len(hist)-1 {
		return hist[idx+1], true
	}
	return "", false
}

// Back returns the previous history entry before the current Now_Playing.
func Back() (string, bool) {
	hist, err := readLines(historyFile)
	if err != nil || len(hist) == 0 {
		return "", false
	}
	cur, err := readNowPlaying()
	if err != nil || cur == "" {
		return hist[len(hist)-1], true
	}
	idx := indexOf(hist, cur)
	if idx > 0 {
		return hist[idx-1], true
	}
	return "", false
}

// Play records the provided path in history and Now_Playing.
// It no longer launches the game directly.
func Play(path string) error {
	if err := WriteNowPlaying(path); err != nil {
		return err
	}
	fmt.Println("[HISTORY] Queued:", path)
	return nil
}

// PlayNext moves to the next entry in history (or random) and returns it.
// Caller is responsible for launching the game.
func PlayNext() (string, error) {
	if p, ok := Next(); ok {
		if err := Play(p); err != nil {
			return "", err
		}
		return p, nil
	}
	p, err := randomGame()
	if err != nil {
		return "", err
	}
	if err := Play(p); err != nil {
		return "", err
	}
	return p, nil
}

// PlayBack moves to the previous entry in history and returns it.
// Caller is responsible for launching the game.
func PlayBack() (string, error) {
	if p, ok := Back(); ok {
		if err := Play(p); err != nil {
			return "", err
		}
		return p, nil
	}
	return "", nil
}

func NowPlayingPath() string {
	p, _ := readNowPlaying()
	return p
}

func randomGame() (string, error) {
	cfg, err := config.LoadUserConfig("SAM", &config.UserConfig{})
	if err != nil {
		return "", err
	}
	listDir := "/tmp/.SAM_List"
	files, err := filepath.Glob(filepath.Join(listDir, "*_gamelist.txt"))
	if err != nil || len(files) == 0 {
		return "", errors.New("no gamelists")
	}
	files = filterAllowed(files, cfg.Attract.Include, cfg.Attract.Exclude)
	if len(files) == 0 {
		return "", errors.New("no gamelists match systems")
	}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for len(files) > 0 {
		listFile := files[r.Intn(len(files))]
		lines, err := readLines(listFile)
		if err != nil || len(lines) == 0 {
			for i, f := range files {
				if f == listFile {
					files = append(files[:i], files[i+1:]...)
					break
				}
			}
			continue
		}
		index := 0
		if cfg.Attract.Random {
			index = r.Intn(len(lines))
		}
		gamePath := lines[index]
		systemID := strings.TrimSuffix(filepath.Base(listFile), "_gamelist.txt")
		if disabled(systemID, gamePath, cfg) {
			lines = append(lines[:index], lines[index+1:]...)
			_ = writeLines(listFile, lines)
			continue
		}
		lines = append(lines[:index], lines[index+1:]...)
		_ = writeLines(listFile, lines)
		return gamePath, nil
	}
	return "", errors.New("no playable games")
}

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

func disabled(system, gamePath string, cfg *config.UserConfig) bool {
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
