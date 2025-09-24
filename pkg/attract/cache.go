package attract

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/synrais/SAM-GO/pkg/utils"
)

// -----------------------------
// Core in-RAM caches
// -----------------------------

var (
	mu     sync.RWMutex
	lists  = make(map[string][]string) // gamelists per system (per file)
	master = make(map[string][]string) // master list per system
	index  = make(map[string][]string) // index per system
)

func cacheSelector(cacheType string) map[string][]string {
	switch cacheType {
	case "lists":
		return lists
	case "master":
		return master
	case "index":
		return index
	default:
		return nil
	}
}

// -----------------------------
// Reload / Reset (global)
// -----------------------------

// ReloadAll clears and reloads all files (lists, master, index) from a directory into RAM.
func ReloadAll(dir string) error {
	mu.Lock()
	defer mu.Unlock()

	// Reset everything
	lists = make(map[string][]string)
	master = make(map[string][]string)
	index = make(map[string][]string)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reload cache: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		path := filepath.Join(dir, e.Name())
		lines, err := utils.ReadLines(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[WARN] failed to read %s: %v\n", path, err)
			continue
		}

		switch e.Name() {
		case "MasterList":
			master["__all__"] = append([]string(nil), lines...)
		case "GameIndex":
			index["__all__"] = append([]string(nil), lines...)
		default:
			// treat everything else as a system gamelist
			lists[e.Name()] = append([]string(nil), lines...)
		}
	}

	return nil
}

// ResetAll clears all caches completely.
func ResetAll() {
	mu.Lock()
	defer mu.Unlock()
	lists = make(map[string][]string)
	master = make(map[string][]string)
	index = make(map[string][]string)
}

// -----------------------------
// Unified cache API
// -----------------------------

func GetCache(cacheType, key string) []string {
	mu.RLock()
	defer mu.RUnlock()
	cache := cacheSelector(cacheType)
	if cache == nil {
		return nil
	}
	return append([]string(nil), cache[key]...)
}

func SetCache(cacheType, key string, vals []string) {
	mu.Lock()
	defer mu.Unlock()
	cache := cacheSelector(cacheType)
	if cache == nil {
		return
	}
	cache[key] = append([]string(nil), vals...)
}

// RemoveCache deletes a key from the chosen cache.
func RemoveCache(cacheType, key string) {
	mu.Lock()
	defer mu.Unlock()
	cache := cacheSelector(cacheType)
	if cache == nil {
		return
	}
	delete(cache, key)
}

// AmendCache appends values onto an existing cache entry.
func AmendCache(cacheType, key string, vals []string) {
	mu.Lock()
	defer mu.Unlock()
	cache := cacheSelector(cacheType)
	if cache == nil {
		return
	}
	cache[key] = append(cache[key], vals...)
}

func CacheKeys(cacheType string) []string {
	mu.RLock()
	defer mu.RUnlock()
	cache := cacheSelector(cacheType)
	if cache == nil {
		return nil
	}
	keys := make([]string, 0, len(cache))
	for k := range cache {
		keys = append(keys, k)
	}
	return keys
}

// -----------------------------
// Flatten helpers
// -----------------------------

// FlattenCache returns the full contents of master or index with system headers.
func FlattenCache(cacheType string) []string {
	mu.RLock()
	defer mu.RUnlock()
	cache := cacheSelector(cacheType)
	if cache == nil {
		return nil
	}

	all := []string{}
	for sys, lines := range cache {
		all = append(all, "# SYSTEM: "+sys)
		all = append(all, lines...)
	}
	return all
}

// FlattenSystem returns only the lines for a given system in lists/master/index.
func FlattenSystem(cacheType, systemID string) []string {
	mu.RLock()
	defer mu.RUnlock()
	cache := cacheSelector(cacheType)
	if cache == nil {
		return nil
	}
	return append([]string(nil), cache[systemID]...)
}
