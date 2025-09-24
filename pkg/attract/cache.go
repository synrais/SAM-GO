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
	mu sync.RWMutex

	// system gamelists
	gamelists    = make(map[string][]string) // working copy
	gamelistBase = make(map[string][]string) // pristine originals

	// global indexes (MasterList, GameIndex, etc.)
	indexes    = make(map[string][]string) // working copy
	indexBase  = make(map[string][]string) // pristine originals
)

// -----------------------------
// Reload + Reset
// -----------------------------

// ReloadAll clears and reloads all gamelists + indexes from a directory into RAM.
func ReloadAll(dir string) error {
	mu.Lock()
	defer mu.Unlock()

	// reset everything
	gamelists = make(map[string][]string)
	gamelistBase = make(map[string][]string)
	indexes = make(map[string][]string)
	indexBase = make(map[string][]string)

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

		// classify into indexes or gamelists
		switch e.Name() {
		case "MasterList", "GameIndex":
			indexes[e.Name()] = append([]string(nil), lines...)
			indexBase[e.Name()] = append([]string(nil), lines...)
		default:
			// treat all other text files as gamelists
			if !strings.HasSuffix(e.Name(), ".txt") {
				continue
			}
			gamelists[e.Name()] = append([]string(nil), lines...)
			gamelistBase[e.Name()] = append([]string(nil), lines...)
		}
	}
	return nil
}

// ResetAll restores all working gamelists + indexes from their pristine originals.
func ResetAll() {
	mu.Lock()
	defer mu.Unlock()

	gamelists = make(map[string][]string, len(gamelistBase))
	for k, v := range gamelistBase {
		gamelists[k] = append([]string(nil), v...)
	}

	indexes = make(map[string][]string, len(indexBase))
	for k, v := range indexBase {
		indexes[k] = append([]string(nil), v...)
	}
}

// -----------------------------
// Gamelist helpers
// -----------------------------

func GetGamelist(name string) []string {
	mu.RLock()
	defer mu.RUnlock()
	return append([]string(nil), gamelists[name]...)
}

func SetGamelist(name string, lines []string) {
	mu.Lock()
	defer mu.Unlock()
	copied := append([]string(nil), lines...)
	gamelists[name] = copied
	if _, ok := gamelistBase[name]; !ok {
		gamelistBase[name] = append([]string(nil), lines...)
	}
}

func ListGamelistKeys() []string {
	mu.RLock()
	defer mu.RUnlock()
	keys := make([]string, 0, len(gamelists))
	for k := range gamelists {
		keys = append(keys, k)
	}
	return keys
}

func DeleteGamelist(name string) {
	mu.Lock()
	defer mu.Unlock()
	delete(gamelists, name)
}

// -----------------------------
// Index helpers (MasterList, GameIndex…)
// -----------------------------

func GetIndex(name string) []string {
	mu.RLock()
	defer mu.RUnlock()
	return append([]string(nil), indexes[name]...)
}

func SetIndex(name string, lines []string) {
	mu.Lock()
	defer mu.Unlock()
	copied := append([]string(nil), lines...)
	indexes[name] = copied
	if _, ok := indexBase[name]; !ok {
		indexBase[name] = append([]string(nil), lines...)
	}
}

func ListIndexKeys() []string {
	mu.RLock()
	defer mu.RUnlock()
	keys := make([]string, 0, len(indexes))
	for k := range indexes {
		keys = append(keys, k)
	}
	return keys
}

func DeleteIndex(name string) {
	mu.Lock()
	defer mu.Unlock()
	delete(indexes, name)
}
