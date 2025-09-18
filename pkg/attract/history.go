package attract

import (
	"github.com/synrais/SAM-GO/pkg/cache"
)

var currentIndex int = -1

// --- core API ---

// Play records the provided path in history and moves pointer to it.
// Used only for attract’s picks (never for browsing).
func Play(path string) {
	hist := cache.GetList("History.txt")
	hist = append(hist, path)
	cache.SetList("History.txt", hist)
	currentIndex = len(hist) - 1
}

// Next returns the next history entry after the current pointer.
func Next() (string, bool) {
	hist := cache.GetList("History.txt")
	if currentIndex >= 0 && currentIndex < len(hist)-1 {
		currentIndex++
		return hist[currentIndex], true
	}
	return "", false
}

// Back returns the previous history entry before the current pointer.
func Back() (string, bool) {
	hist := cache.GetList("History.txt")
	if currentIndex > 0 {
		currentIndex--
		return hist[currentIndex], true
	}
	return "", false
}

// PlayNext moves to the next entry (no new entry added).
// If there’s nothing after, just return empty — attract handles random.
func PlayNext() (string, bool) {
	return Next()
}

// PlayBack moves to the previous entry (no new entry added).
func PlayBack() (string, bool) {
	return Back()
}
