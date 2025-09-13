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

// --- utils ---

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

func indexOf(slice []string, val string) int {
	for i, v := range slice {
		if v == val {
			return i
		}
	}
	return -1
}

func readNowPlaying() (string, error) {
	b, err := os.ReadFile(nowPlayingFile)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// --- core API ---

// WriteNowPlaying moves Now_Playing to `path` and ensures it's in history.
// If already present, does not duplicate.
func WriteNowPlaying(path string) error {
	_ = os.MkdirAll(filepath.Dir(historyFile), 0777)

	// Always set Now_Playing
	if err := os.WriteFile(nowPlayingFile, []byte(path), 0644); err != nil {
		return err
	}

	// Ensure unique history
	hist, _ := readLines(historyFile)
	if indexOf(hist, path) == -1 {
		hist = append(hist, path)
		if err := writeLines(historyFile, hist); err != nil {
			return err
		}
	}
	return nil
}

// SetNowPlaying updates Now_Playing only (does not change history order).
func SetNowPlaying(path string) error {
	_ = os.MkdirAll(filepath.Dir(historyFile), 0777)
	return os.WriteFile(nowPlayingFile, []byte(path), 0644)
}

// Next returns the next history entry after the current Now_Playing.
// If at the end of history, returns "", false (caller should random).
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

// Back returns the previous history entry before Now_Playing.
// If already at the first entry, returns "", false (no wrap, no random).
func Back() (string, bool) {
	hist, err := readLines(historyFile)
	if err != nil || len(hist) == 0 {
		return "", false
	}
	cur, err := readNowPlaying()
	if err != nil || cur == "" {
		return "", false
	}
	idx := indexOf(hist, cur)
	if idx > 0 {
		return hist[idx-1], true
	}
	return "", false
}

// Play records the provided path in history and moves Now_Playing.
func Play(path string) error {
	if err := WriteNowPlaying(path); err != nil {
		return err
	}
	fmt.Println("[HISTORY] Queued:", path)
	return nil
}

// PlayNext moves to the next entry in history, or random if none.
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

// PlayBack moves to the previous entry in history (no random fallback).
func PlayBack() (string, error) {
	if p, ok := Back(); ok {
		if err := Play(p); err != nil {
			return "", err
		}
		return p, nil
	}
	return "", nil
}

// NowPlayingPath returns the current Now_Playing path.
func NowPlayingPath() string {
	p, _ := readNowPlaying()
	return p
}

// --- random picker ---

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
		// remove from list so it won't repeat immediately
		lines = append(lines[:index], lines[index+1:]...)
		_ = writeLines(listFile, lines)

		return gamePath, nil
	}
	return "", errors.New("no playable games")
}
