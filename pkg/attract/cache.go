package attract

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/synrais/SAM-GO/pkg/utils"
)

// -----------------------------
// Core in-RAM cache
// -----------------------------

var (
	mu     sync.RWMutex
	lists  = make(map[string][]string) // gamelists per system
	master = make(map[string][]string) // master list per system
	index  = make(map[string][]string) // index per system
)

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
// Lists cache
// -----------------------------

func GetList(name string) []string {
	mu.RLock()
	defer mu.RUnlock()
	return append([]string(nil), lists[name]...)
}

func SetList(name string, lines []string) {
	mu.Lock()
	defer mu.Unlock()
	lists[name] = append([]string(nil), lines...)
}

func RemoveList(name string) {
	mu.Lock()
	defer mu.Unlock()
	delete(lists, name)
}

func AmendList(name string, lines []string) {
	mu.Lock()
	defer mu.Unlock()
	lists[name] = append([]string(nil), lines...)
}

func ListKeys() []string {
	mu.RLock()
	defer mu.RUnlock()
	keys := make([]string, 0, len(lists))
	for k := range lists {
		keys = append(keys, k)
	}
	return keys
}

// -----------------------------
// MasterList cache
// -----------------------------

func GetMasterSystem(systemID string) []string {
	mu.RLock()
	defer mu.RUnlock()
	return append([]string(nil), master[systemID]...)
}

func SetMasterSystem(systemID string, paths []string) {
	mu.Lock()
	defer mu.Unlock()
	master[systemID] = append([]string(nil), paths...)
}

func RemoveMasterSystem(systemID string) {
	mu.Lock()
	defer mu.Unlock()
	delete(master, systemID)
}

func AmendMasterSystem(systemID string, paths []string) {
	mu.Lock()
	defer mu.Unlock()
	master[systemID] = append([]string(nil), paths...)
}

func MasterKeys() []string {
	mu.RLock()
	defer mu.RUnlock()
	keys := make([]string, 0, len(master))
	for k := range master {
		keys = append(keys, k)
	}
	return keys
}

// -----------------------------
// GameIndex cache
// -----------------------------

func GetIndexSystem(systemID string) []string {
	mu.RLock()
	defer mu.RUnlock()
	return append([]string(nil), index[systemID]...)
}

func SetIndexSystem(systemID string, paths []string) {
	mu.Lock()
	defer mu.Unlock()
	index[systemID] = append([]string(nil), paths...)
}

func RemoveIndexSystem(systemID string) {
	mu.Lock()
	defer mu.Unlock()
	delete(index, systemID)
}

func AmendIndexSystem(systemID string, paths []string) {
	mu.Lock()
	defer mu.Unlock()
	index[systemID] = append([]string(nil), paths...)
}

func IndexKeys() []string {
	mu.RLock()
	defer mu.RUnlock()
	keys := make([]string, 0, len(index))
	for k := range index {
		keys = append(keys, k)
	}
	return keys
}
