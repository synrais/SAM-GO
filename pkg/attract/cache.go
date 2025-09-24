package attract

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/synrais/SAM-GO/pkg/utils"
)

// -----------------------------
// Core in-RAM cache
// -----------------------------

var (
	mu       sync.RWMutex
	lists    = make(map[string][]string) // working copy (mutates as games are consumed)
	masters  = make(map[string][]string) // pristine originals (never touched)
	gameIndex []string                   // full search index, lines like "name|path"
)

// -----------------------------
// List cache (gamelists, masterlist, history)
// -----------------------------

// ReloadAll clears and reloads all list files and GameIndex from a directory into RAM.
// Both working and master copies are initialized, and GameIndex is rebuilt too.
func ReloadAll(dir string) error {
	mu.Lock()
	defer mu.Unlock()

	// Reset everything
	lists = make(map[string][]string)
	masters = make(map[string][]string)
	gameIndex = []string{}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reload cache: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		// 🔥 Special case: rebuild GameIndex from its file
		if e.Name() == "GameIndex" {
			path := filepath.Join(dir, e.Name())
			lines, err := utils.ReadLines(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[WARN] failed to read %s: %v\n", path, err)
				continue
			}
			gameIndex = append(gameIndex, lines...)
			continue
		}

		// Normal list files
		path := filepath.Join(dir, e.Name())
		lines, err := utils.ReadLines(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[WARN] failed to read %s: %v\n", path, err)
			continue
		}
		lists[e.Name()] = append([]string(nil), lines...)
		masters[e.Name()] = append([]string(nil), lines...)
	}

	return nil
}

// GetList returns the cached working copy for a filename.
func GetList(name string) []string {
	mu.RLock()
	defer mu.RUnlock()
	return append([]string(nil), lists[name]...) // defensive copy
}

// SetList replaces or creates a list in cache.
// The first time we see a name, we also set its master copy.
func SetList(name string, lines []string) {
	mu.Lock()
	defer mu.Unlock()
	copied := append([]string(nil), lines...) // defensive copy
	lists[name] = copied
	if _, ok := masters[name]; !ok {
		masters[name] = append([]string(nil), lines...)
	}
}

// ListKeys returns all cached filenames (working copy).
func ListKeys() []string {
	mu.RLock()
	defer mu.RUnlock()
	keys := make([]string, 0, len(lists))
	for k := range lists {
		keys = append(keys, k)
	}
	return keys
}

// DeleteKey removes a list entirely from cache (working only).
func DeleteKey(name string) {
	mu.Lock()
	defer mu.Unlock()
	delete(lists, name)
	// leave masters untouched so ResetAll can restore it later
}

// ResetAll restores all working lists back to their master originals,
// and also clears the GameIndex.
func ResetAll() {
	mu.Lock()
	defer mu.Unlock()

	// reset lists
	lists = make(map[string][]string, len(masters))
	for k, v := range masters {
		lists[k] = append([]string(nil), v...)
	}

	// 🔥 also reset index
	gameIndex = []string{}
}

// -----------------------------
// GameIndex cache
// -----------------------------

// AppendGameIndex adds one line ("name|path") into the in-RAM index.
func AppendGameIndex(line string) {
	mu.Lock()
	defer mu.Unlock()
	gameIndex = append(gameIndex, line)
}

// GetGameIndex returns a snapshot of the current index.
func GetGameIndex() []string {
	mu.RLock()
	defer mu.RUnlock()
	return append([]string(nil), gameIndex...)
}

// ReloadGameIndexFromDisk repopulates index if it's empty (for reused runs).
func ReloadGameIndexFromDisk(path string) {
	mu.Lock()
	defer mu.Unlock()
	if len(gameIndex) > 0 {
		return
	}
	lines, err := utils.ReadLines(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[WARN] failed to read %s: %v\n", path, err)
		return
	}
	gameIndex = append(gameIndex, lines...)
}
