package history

import (
	"errors"
	"os"
	"strings"

	"github.com/synrais/SAM-GO/pkg/cache"
)

const nowPlayingFile = "/tmp/Now_Playing.txt"

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
	if err := os.WriteFile(nowPlayingFile, []byte(path), 0644); err != nil {
		return err
	}
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
// Used only for attract’s picks (never for browsing).
func Play(path string) error {
	if err := WriteNowPlaying(path); err != nil {
		return err
	}
	return nil
}

// PlayNext moves to the next entry in history (no new entry added).
// If there’s nothing after, just return empty — attract handles random.
func PlayNext() (string, error) {
	if p, ok := Next(); ok {
		if err := SetNowPlaying(p); err != nil {
			return "", err
		}
		return p, nil
	}
	return "", nil
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
