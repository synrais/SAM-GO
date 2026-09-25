package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/synrais/SAM-GO/pkg/config"
)

// -------------------------
// Remember Position
// -------------------------
//
// Display & Sorting -> Menu list options -> Remember position. When a game
// is launched or the menu exits, where you were is saved: one step per
// level, e.g. "s:SNES" -> "f:Hacks" -> "i:Super Mario World.sfc", each with
// its line number. The next time the menu opens it walks back down that
// path. A step that no longer exists (renamed, removed, sorted
// differently) stops the walk there, on the same line number if possible.

var optRememberPos = labelOption{"Remember position", []string{"Off", "On"}, 0}

type navStep struct {
	key   string
	index int
}

var (
	nav     []navStep // where we are now, one step per level
	restore []navStep // saved position being walked back to
	posFile string
)

// navHere records the highlighted line at a level, dropping deeper steps.
func navHere(depth int, key string, index int) {
	if len(nav) > depth {
		nav = nav[:depth]
	}
	for len(nav) < depth { // shouldn't happen; keep the path well formed
		nav = append(nav, navStep{})
	}
	nav = append(nav, navStep{key, index})
}

// restoreAt returns where to start at a level, from the saved position:
// the line with the saved key, else the saved line number (clamped). ok
// is false when there's nothing saved for this level. found says the key
// itself was found; open says the saved position goes deeper, so this
// line should be opened straight away.
func restoreAt(depth int, keyAt func(i int) string, count int) (index int, ok, found, open bool) {
	if depth >= len(restore) || count == 0 {
		return 0, false, false, false
	}
	step := restore[depth]
	for i := 0; i < count; i++ {
		if keyAt(i) == step.key {
			return i, true, true, depth < len(restore)-1
		}
	}
	// Gone: stay near where it was, and stop walking.
	restore = nil
	index = step.index
	if index >= count {
		index = count - 1
	}
	if index < 0 {
		index = 0
	}
	return index, true, false, false
}

// doneRestoring stops walking back once a level has used its step.
func doneRestoring(depth int) {
	if depth >= len(restore)-1 {
		restore = nil
	}
}

func setupPosition(cfg *config.Config) {
	posFile = filepath.Join(filepath.Dir(cfg.Path), "gamesmenu_position.txt")
	if !optRememberPos.isOn() {
		return
	}
	b, err := os.ReadFile(posFile)
	if err != nil {
		return
	}
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		key, idx, ok := strings.Cut(line, "\t")
		if !ok || key == "" {
			break
		}
		n, _ := strconv.Atoi(idx)
		restore = append(restore, navStep{key, n})
	}
}

// savePosition writes where we are (when a game launches or the menu
// exits), or forgets it when the setting is off.
func savePosition() {
	if posFile == "" {
		return
	}
	if !optRememberPos.isOn() || len(nav) == 0 {
		_ = os.Remove(posFile)
		return
	}
	var b strings.Builder
	for _, s := range nav {
		b.WriteString(s.key + "\t" + strconv.Itoa(s.index) + "\n")
	}
	_ = os.WriteFile(posFile, []byte(b.String()), 0644)
}

// entryKey names a folder listing's line for the saved position.
func browseKey(e browseEntry) string {
	switch {
	case e.Node != nil:
		return "d:" + e.Folder
	case e.File == nil:
		return "f:" + e.Folder
	default:
		return "i:" + e.File.Name + "." + e.File.Ext
	}
}
