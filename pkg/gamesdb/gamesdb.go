package gamesdb

import (
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/games"
)

// -------------------------
// Gob-backed index
// -------------------------

// GobEntry stores full details for a single game file.
type GobEntry struct {
	SystemId   string
	Name       string
	Ext        string
	Path       string
	MenuPath   string
	Search     string // 🔑 lowercase "name + ext" for matching
	SearchName string // 🔑 "[SystemName] Name.ext" for display
}

// GobIndex maps base names -> slice of entries (supports duplicates).
type GobIndex map[string][]GobEntry

// SaveGobIndex encodes the index to disk.
func SaveGobIndex(idx GobIndex, filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := gob.NewEncoder(f)
	return enc.Encode(idx)
}

// LoadGobIndex decodes the index from disk.
func LoadGobIndex(filename string) (GobIndex, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var idx GobIndex
	dec := gob.NewDecoder(f)
	err = dec.Decode(&idx)
	if err != nil {
		return nil, err
	}
	return idx, nil
}

// -------------------------
// Building
// -------------------------

// BuildGobIndex scans systems and builds the index fully in memory,
// reporting progress once per completed system via the optional update callback.
func BuildGobIndex(
	cfg *config.UserConfig,
	systems []games.System,
	update func(systemName string, done, total int),
) (GobIndex, error) {
	idx := make(GobIndex)
	total := len(systems)
	done := 0

	for _, sys := range systems {
		paths := games.GetSystemPaths(cfg, []games.System{sys})
		for _, sp := range paths {
			files, err := games.GetFiles(sys.Id, sp.Path)
			if err != nil {
				return nil, fmt.Errorf("error getting files for %s: %w", sys.Id, err)
			}
			for _, fullPath := range files {
				base := filepath.Base(fullPath)
				ext := strings.TrimPrefix(filepath.Ext(base), ".")
				name := strings.TrimSuffix(base, filepath.Ext(base))

				// --- Build MenuPath with old TXT + ZIP logic ---
				rel, _ := filepath.Rel(sp.Path, fullPath)
				relParts := strings.Split(filepath.ToSlash(rel), "/")

				// Case 1: collapse fake .zip folder
				if len(relParts) > 0 && strings.HasSuffix(relParts[0], ".zip") {
					relParts = relParts[1:]
				}

				// Case 2: listings/*.txt collapse to label
				if len(relParts) > 1 && relParts[0] == "listings" && strings.HasSuffix(relParts[1], ".txt") {
					label := strings.TrimSuffix(relParts[1], ".txt")
					if len(label) > 0 {
						label = strings.ToUpper(label[:1]) + label[1:]
					}
					relParts = append([]string{label}, relParts[2:]...)
				}

				menuPath := filepath.Join(append([]string{sys.Name}, relParts...)...)

				// Precompute search fields
				search := strings.ToLower(fmt.Sprintf("%s %s", name, ext))
				searchName := fmt.Sprintf("[%s] %s", sys.Name, base)

				entry := GobEntry{
					SystemId:   sys.Id,
					Name:       name,
					Ext:        ext,
					Path:       fullPath,
					MenuPath:   filepath.ToSlash(menuPath),
					Search:     search,
					SearchName: searchName,
				}
				idx[name] = append(idx[name], entry)
			}
		}

		// Update after finishing each system
		done++
		if update != nil {
			update(sys.Name, done, total)
		}
	}

	return idx, nil
}

// -------------------------
// Searching
// -------------------------

// SearchWords returns only the *first entry per unique name+ext* that matches query.
func (idx GobIndex) SearchWords(query string) []GobEntry {
	var results []GobEntry
	words := strings.Fields(strings.ToLower(query))
	seen := make(map[string]bool)

outer:
	for _, entries := range idx {
		if len(entries) == 0 {
			continue
		}
		first := entries[0]
		for _, w := range words {
			if !strings.Contains(first.Search, w) {
				continue outer
			}
		}
		key := fmt.Sprintf("%s.%s", first.Name, first.Ext)
		if !seen[key] {
			seen[key] = true
			results = append(results, first)
		}
	}
	return results
}
