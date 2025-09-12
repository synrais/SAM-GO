package history

import (
	"bufio"
	"errors"
	"os"
	"strings"

	"github.com/synrais/SAM-GO/pkg/run"
)

const (
	nowPlayingFile = "/tmp/Now_Playing.txt"
	historyFile    = "/tmp/History.txt"
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

func appendLine(path, line string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(line + "\n")
	return err
}

// WriteNowPlaying records the current file and appends it to history if new.
func WriteNowPlaying(path string) error {
	if err := os.WriteFile(nowPlayingFile, []byte(path), 0644); err != nil {
		return err
	}
	hist, err := readLines(historyFile)
	if err == nil {
		for _, h := range hist {
			if h == path {
				return nil
			}
		}
	}
	return appendLine(historyFile, path)
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

// Play launches the provided path and records it as Now_Playing.
func Play(path string) error {
	if err := WriteNowPlaying(path); err != nil {
		return err
	}
	return run.Run([]string{path})
}

// PlayNext moves to the next entry in history and launches it.
func PlayNext() error {
	p, ok := Next()
	if !ok {
		return errors.New("no next history")
	}
	return Play(p)
}

// PlayBack moves to the previous entry in history and launches it.
func PlayBack() error {
	p, ok := Back()
	if !ok {
		return errors.New("no previous history")
	}
	return Play(p)
}

func NowPlayingPath() string {
	p, _ := readNowPlaying()
	return p
}
