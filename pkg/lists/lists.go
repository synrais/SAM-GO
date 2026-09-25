// Package lists reads and writes SAM's per-system game lists:
//
//	.MiSTer_SAM/Lists/Blacklist/SNES_blacklist.txt    games never to play
//	.MiSTer_SAM/Lists/Staticlist/SNES_staticlist.txt  when a game goes static
//	.MiSTer_SAM/Lists/Whitelist/SNES_whitelist.txt    the only games to play
//
// Each line is one game title (with or without its file extension). Static
// list lines start with the time in seconds: "<42> Super Mario World (USA)".
// Blank lines and lines starting with # or ; are ignored. Titles are
// compared ignoring case and punctuation.
package lists

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/utils"
)

// Kind is a type of list. Its value is also its folder name.
type Kind string

const (
	Blacklist  Kind = "Blacklist"
	Staticlist Kind = "Staticlist"
	Whitelist  Kind = "Whitelist"
)

var writeMu sync.Mutex

// Path returns the list file for a system, e.g.
// ".../Lists/Blacklist/SNES_blacklist.txt". An existing file whose name
// differs only in case is used as-is.
func Path(kind Kind, systemID string) string {
	dir := filepath.Join(config.ListsFolder, string(kind))
	name := systemID + "_" + strings.ToLower(string(kind)) + ".txt"
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.EqualFold(e.Name(), name) {
				return filepath.Join(dir, e.Name())
			}
		}
	}
	return filepath.Join(dir, name)
}

// Key is how game titles are compared: see utils.NormalizeTitle.
func Key(title string) string {
	return utils.NormalizeTitle(title)
}

// Names reads a Blacklist or Whitelist into a set of title keys. It
// returns nil if the system has no list file.
func Names(kind Kind, systemID string) map[string]bool {
	names := make(map[string]bool)
	ok := readLines(Path(kind, systemID), func(line string) {
		for _, k := range lineKeys(line) {
			names[k] = true
		}
	})
	if !ok {
		return nil
	}
	return names
}

// StaticTimes reads a system's Staticlist: title key -> seconds into the
// game when its screen went static. It returns nil if there's no file.
func StaticTimes(systemID string) map[string]float64 {
	times := make(map[string]float64)
	ok := readLines(Path(Staticlist, systemID), func(line string) {
		secs, title := utils.ParseLine(line)
		if secs <= 0 {
			return
		}
		for _, k := range lineKeys(title) {
			times[k] = secs
		}
	})
	if !ok {
		return nil
	}
	return times
}

// Add puts a title on a Blacklist or Whitelist, unless it's already there.
func Add(kind Kind, systemID, title string) error {
	if Names(kind, systemID)[Key(title)] {
		return nil
	}
	return appendLine(Path(kind, systemID), title)
}

// AddStatic records when a game's screen went static, unless the game is
// already on the Staticlist.
func AddStatic(systemID, title string, seconds float64) error {
	if _, ok := StaticTimes(systemID)[Key(title)]; ok {
		return nil
	}
	return appendLine(Path(Staticlist, systemID), fmt.Sprintf("<%s> %s", strconv.FormatFloat(seconds, 'f', 0, 64), title))
}

// lineKeys returns the keys a list line can match: the whole line, and the
// line without a trailing file extension (e.g. "Tetris.nes" -> "Tetris").
func lineKeys(line string) []string {
	keys := []string{Key(line)}
	if ext := filepath.Ext(line); len(ext) >= 2 && len(ext) <= 5 && !strings.ContainsAny(ext, " ()[]") {
		keys = append(keys, Key(strings.TrimSuffix(line, ext)))
	}
	return keys
}

func readLines(path string, fn func(line string)) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] == '#' || line[0] == ';' {
			continue
		}
		fn(line)
	}
	return true
}

func appendLine(path, line string) error {
	writeMu.Lock()
	defer writeMu.Unlock()

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintln(f, line)
	return err
}
