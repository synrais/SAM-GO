package attract

import "github.com/synrais/SAM-GO/pkg/gamesdb"

// History
//
// The games attract mode has played, in order, with a pointer to the one
// playing now. Back steps the pointer back and replays that game; Next
// steps forward again. Only at the newest end does Next (or the play time
// running out) pick a new random game, which is added to the end.

const historySize = 100

type timeline struct {
	games []gamesdb.FileInfo
	pos   int // index of the game playing now, -1 before the first
}

func newTimeline() *timeline { return &timeline{pos: -1} }

// add records a new (random) game at the end and points to it.
func (t *timeline) add(g gamesdb.FileInfo) {
	t.games = append(t.games, g)
	if len(t.games) > historySize {
		t.games = t.games[1:]
	}
	t.pos = len(t.games) - 1
}

// canGoBack reports whether there's an earlier game.
func (t *timeline) canGoBack() bool { return t.pos > 0 }

// back steps to the previous game.
func (t *timeline) back() (gamesdb.FileInfo, bool) {
	if !t.canGoBack() {
		return gamesdb.FileInfo{}, false
	}
	t.pos--
	return t.games[t.pos], true
}

// forward steps to the next game, if you've gone back. At the newest end
// it reports false: time for a new random game.
func (t *timeline) forward() (gamesdb.FileInfo, bool) {
	if t.pos+1 >= len(t.games) {
		return gamesdb.FileInfo{}, false
	}
	t.pos++
	return t.games[t.pos], true
}

// remove takes every entry for a game out (after blacklisting it). If the
// current game was removed, the pointer is left just before where it was,
// so forward() continues with the game that came after it; otherwise it
// stays on the current game.
func (t *timeline) remove(path string) {
	kept := t.games[:0]
	newPos := -1
	for i, g := range t.games {
		if g.Path == path {
			continue
		}
		kept = append(kept, g)
		if i <= t.pos {
			newPos = len(kept) - 1
		}
	}
	t.games = kept
	t.pos = newPos
}
