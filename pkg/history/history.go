package history

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/synrais/SAM-GO/pkg/cache"
	"github.com/synrais/SAM-GO/pkg/config"
)

const (
	nowPlayingFile = "/tmp/Now_Playing.txt"
)

// --- utils ---

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

// WriteNowPlaying sets Now_Playing and appends to in-memory History.
// No deduplication: every call appends a new entry.
func WriteNowPlaying(path string) error {
	// Always set Now_Playing on disk so other tools can read it
	if err := os.WriteFile(nowPlayingFile, []byte(path), 0644); err != nil {
		return err
	}

	// Update in-memory history (append only)
	hist := cache.GetList("History.txt")
	hist = append(hist, path)
	cache.SetList("History.txt", hist)

	return nil
}

// SetNowPlaying updates Now_Playing only (no history update).
func SetNowPlaying(path string) error {
	return os.WriteFile(nowPlayingFile, []byte(path), 0644)
}

// Next returns the next history entry after Now_Playing.
func Next() (string, bool) {
	hist := cache.GetList("History.txt")
	if len(hist) == 0 {
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
func Back() (string, bool) {
	hist := cache.GetList("History.txt")
	if len(hist) == 0 {
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
// Used only for random picks (never for browsing).
func Play(path string) error {
	if err := WriteNowPlaying(path); err != nil {
		return err
	}
	fmt.Println("[HISTORY] Logged:", path)
	return nil
}

// PlayNext moves to the next entry in history (no new entry added).
func PlayNext() (string, error) {
	if p, ok := Next(); ok {
		if err := SetNowPlaying(p); err != nil {
			return "", err
		}
		return p, nil
	}
	// No next? → fall back to random
	p, err := randomGame()
	if err != nil {
		return "", err
	}
	if err := Play(p); err != nil {
		return "", err
	}
	return p, nil
}

// PlayBack moves to the previous entry in history (no new entry added).
func PlayBack() (string, error) {
	if p, ok := Back(); ok {
		if err := SetNowPlaying(p); err != nil {
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

func randomGame() (string, error) {
	cfg, err := config.LoadUserConfig("SAM", &config.UserConfig{})
	if err != nil {
		return "", err
	}

retry:
	// Collect all system gamelists from cache
	allKeys := cache.ListKeys()
	var systems []string
	for _, k := range allKeys {
		if strings.HasSuffix(k, "_gamelist.txt") {
			systems = append(systems, k)
		}
	}
	if len(systems) == 0 {
		return "", errors.New("no gamelists in cache")
	}

	// Apply include/exclude
	var filtered []string
	for _, sys := range systems {
		base := strings.TrimSuffix(filepath.Base(sys), "_gamelist.txt")
		if len(cfg.Attract.Include) > 0 {
			match := false
			for _, inc := range cfg.Attract.Include {
				if strings.EqualFold(strings.TrimSpace(inc), base) {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}
		skip := false
		for _, ex := range cfg.Attract.Exclude {
			if strings.EqualFold(strings.TrimSpace(ex), base) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		filtered = append(filtered, sys)
	}
	if len(filtered) == 0 {
		return "", errors.New("no gamelists match systems")
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for len(filtered) > 0 {
		listKey := filtered[r.Intn(len(filtered))]
		lines := cache.GetList(listKey)
		if len(lines) == 0 {
			// remove empty system from rotation
			var tmp []string
			for _, f := range filtered {
				if f != listKey {
					tmp = append(tmp, f)
				}
			}
			filtered = tmp
			continue
		}

		index := 0
		if cfg.Attract.Random {
			index = r.Intn(len(lines))
		}
		gamePath := lines[index]

		// remove so it won’t repeat immediately
		lines = append(lines[:index], lines[index+1:]...)
		cache.SetList(listKey, lines)

		return gamePath, nil
	}

	// If we got here, all systems are exhausted → reset & retry once
	fmt.Println("[HISTORY] All systems exhausted, refreshing cache...")
	cache.ResetAll()
	goto retry
}
