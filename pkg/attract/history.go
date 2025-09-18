package attract

import (
	"github.com/synrais/SAM-GO/pkg/cache"
)

// currentIndex points into the History timeline (in cache).
var currentIndex int = -1

// --- core API ---

// Play appends a game path to the in-memory history timeline
// and moves the pointer to it.
// Used only when attract *actually launches* a game.
func Play(path string) {
	hist := cache.GetList("History.txt")
	hist = append(hist, path)
	cache.SetList("History.txt", hist)
	currentIndex = len(hist) - 1
}

// Next returns the next history entry after the current pointer.
// It advances currentIndex if possible.
func Next() (string, bool) {
	hist := cache.GetList("History.txt")
	if currentIndex >= 0 && currentIndex < len(hist)-1 {
		currentIndex++
		return hist[currentIndex], true
	}
	return "", false
}

// Back returns the previous history entry before the current pointer.
// It moves currentIndex back if possible.
func Back() (string, bool) {
	hist := cache.GetList("History.txt")
	if currentIndex > 0 {
		currentIndex--
		return hist[currentIndex], true
	}
	return "", false
}

// PlayNext moves forward in history (no new entry added).
// If there’s nothing after, it just returns empty —
// attract mode will handle picking a new random game.
func PlayNext() (string, bool) {
	return Next()
}

// PlayBack moves backward in history (no new entry added).
func PlayBack() (string, bool) {
	return Back()
}
