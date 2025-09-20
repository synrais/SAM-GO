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
// Core cache maps
// -----------------------------

var (
	mu        sync.RWMutex
	lists     = make(map[string][]string)   // working copy (mutable)
	masters   = make(map[string][]string)   // pristine originals (never touched)
	gameIndex []GameEntry                   // central slice of all games
)

// -----------------------------
// Text list cache
// -----------------------------

// ReloadAll clears and reloads all .txt files from a directory into RAM.
// Both working and master copies are initialized.
func ReloadAll(dir string) error {
	mu.Lock()
	defer mu.Unlock()

	lists = make(map[string][]string)
	masters = make(map[string][]string)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reload cache: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".txt") {
			continue
		}
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
	copied := append([]string(nil), lines...)
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
	// masters left intact, ResetAll can restore later
}

// ResetAll restores all working lists back to their master originals.
func ResetAll() {
	mu.Lock()
	defer mu.Unlock()
	lists = make(map[string][]string, len(masters))
	for k, v := range masters {
		lists[k] = append([]string(nil), v...)
	}
}

// -----------------------------
// GameIndex cache
// -----------------------------

// GetGameIndex returns the full game index (copy).
func GetGameIndex() []GameEntry {
	mu.RLock()
	defer mu.RUnlock()
	return append([]GameEntry(nil), gameIndex...)
}

// SetGameIndex replaces the entire index.
func SetGameIndex(entries []GameEntry) {
	mu.Lock()
	defer mu.Unlock()
	gameIndex = append([]GameEntry(nil), entries...)
}

// AppendGameIndex adds entries to the index.
func AppendGameIndex(entries ...GameEntry) {
	mu.Lock()
	defer mu.Unlock()
	gameIndex = append(gameIndex, entries...)
}

// ClearGameIndex wipes the index completely.
func ClearGameIndex() {
	mu.Lock()
	defer mu.Unlock()
	gameIndex = nil
}

// LenGameIndex returns the number of entries in the index.
func LenGameIndex() int {
	mu.RLock()
	defer mu.RUnlock()
	return len(gameIndex)
}
