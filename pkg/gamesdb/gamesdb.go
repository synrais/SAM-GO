package gamesdb

import (
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/games"
)

// -------------------------
// Gob-backed index
// -------------------------

// GobEntry stores full details for a single game file.
type GobEntry struct {
	SystemId   string // Internal system ID
	Name       string // Base name without extension
	Ext        string // File extension (e.g. "nes", "gg")
	Path       string // Full path to file
	MenuPath   string // "SystemName/<relative path under SystemFolder>"
	Search     string // normalized search string (name + .ext as tokens)
	SearchName string // "[System] Name.ext" for display
}

// GobIndex holds both the sorted slice and the search map.
type GobIndex struct {
	Entries []GobEntry               // Sorted slice (canonical order for menus)
	Map     map[string][]GobEntry    // Fast lookup for search
}

// -------------------------
// Saving / Loading
// -------------------------

// SaveGobIndex encodes only the sorted slice to disk.
func SaveGobIndex(idx GobIndex, filename string) error {
	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		return err
	}

	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := gob.NewEncoder(f)
	return enc.Encode(idx.Entries)
}

// LoadGobIndex decodes the slice from disk and rebuilds the map.
func LoadGobIndex(filename string) (GobIndex, error) {
	f, err := os.Open(filename)
	if err != nil {
		return GobIndex{}, err
	}
	defer f.Close()

	var entries []GobEntry
	dec := gob.NewDecoder(f)
	if err := dec.Decode(&entries); err != nil {
		return GobIndex{}, err
	}

	// Rebuild the map from the sorted slice
	m := make(map[string][]GobEntry)
	for _, e := range entries {
		m[e.Name] = append(m[e.Name], e)
	}

	return GobIndex{Entries: entries, Map: m}, nil
}

// -------------------------
// Building
// -------------------------

// BuildGobIndex scans systems and builds both slice + map.
// This is the *only* place we sort — truth for order is baked here.
func BuildGobIndex(
	cfg *config.UserConfig,
	systems []games.System,
	update func(systemName string, done, total int),
) (GobIndex, error) {
	var all []GobEntry
	total := len(systems)
	done := 0

	for _, sys := range systems {
		done++
		if update != nil {
			update(sys.Name, done, total)
		}

		paths := games.GetSystemPaths(cfg, []games.System{sys})
		for _, sp := range paths {
			files, err := games.GetFiles(sys.Id, sp.Path)
			if err != nil {
				return GobIndex{}, fmt.Errorf("error getting files for %s: %w", sys.Id, err)
			}

			for _, fullPath := range files {
				base := filepath.Base(fullPath)
				ext := strings.TrimPrefix(filepath.Ext(base), ".")
				name := strings.TrimSuffix(base, filepath.Ext(base))

				// --- Build MenuPath with TXT + ZIP logic ---
				rel, _ := filepath.Rel(sp.Path, fullPath)
				relParts := strings.Split(filepath.ToSlash(rel), "/")

				if len(relParts) > 0 && strings.HasSuffix(relParts[0], ".zip") {
					relParts = relParts[1:]
				}
				if len(relParts) > 1 && relParts[0] == "listings" && strings.HasSuffix(relParts[1], ".txt") {
					label := strings.TrimSuffix(relParts[1], ".txt")
					if len(label) > 0 {
						label = strings.ToUpper(label[:1]) + label[1:]
					}
					relParts = append([]string{label}, relParts[2:]...)
				}
				if len(relParts) > 0 && relParts[0] == "media" {
					continue
				}

				menuPath := filepath.Join(append([]string{sys.Name}, relParts...)...)

				search := strings.ToLower(fmt.Sprintf("%s .%s", name, ext))
				searchName := fmt.Sprintf("[%s] %s", sys.Name, base)

				all = append(all, GobEntry{
					SystemId:   sys.Id,
					Name:       name,
					Ext:        ext,
					Path:       fullPath,
					MenuPath:   filepath.ToSlash(menuPath),
					Search:     search,
					SearchName: searchName,
				})
			}
		}
	}

	// 🔹 Single global sort by MenuPath (source of truth)
	sort.Slice(all, func(i, j int) bool {
		return strings.ToLower(all[i].MenuPath) < strings.ToLower(all[j].MenuPath)
	})

	// Build map from sorted slice
	m := make(map[string][]GobEntry)
	for _, e := range all {
		m[e.Name] = append(m[e.Name], e)
	}

	return GobIndex{Entries: all, Map: m}, nil
}

// -------------------------
// Searching
// -------------------------

// SearchWords returns one entry per unique SearchName,
// sorted by SearchName for consistent ordering.
func (idx GobIndex) SearchWords(query string) []GobEntry {
	var results []GobEntry
	words := strings.Fields(strings.ToLower(query))
	seen := make(map[string]bool)

	for _, entries := range idx.Map {
		for _, e := range entries {
			lower := strings.ToLower(e.Search)
			match := true
			for _, w := range words {
				if !strings.Contains(lower, w) {
					match = false
					break
				}
			}
			if !match {
				continue
			}

			// Deduplicate by SearchName
			if seen[e.SearchName] {
				continue
			}
			seen[e.SearchName] = true
			results = append(results, e)
		}
	}

	// Sort once, by SearchName
	sort.Slice(results, func(i, j int) bool {
		return strings.ToLower(results[i].SearchName) < strings.ToLower(results[j].SearchName)
	})

	return results
}
