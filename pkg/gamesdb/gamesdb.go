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

// GobIndex maps base names -> slice of entries (supports duplicates).
type GobIndex map[string][]GobEntry

// -------------------------
// Progress reporting
// -------------------------

type IndexStatus struct {
	Total    int    // total systems
	Step     int    // current system number (1-based)
	SystemId string // current system ID
	System   string // current system name
}

// -------------------------
// Save/Load
// -------------------------

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
	return enc.Encode(idx)
}

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
// Build Index
// -------------------------

func BuildGobIndex(
	cfg *config.UserConfig,
	systems []games.System,
	update func(IndexStatus),
) (GobIndex, error) {
	idx := make(GobIndex)
	status := IndexStatus{Total: len(systems)}

	for i, sys := range systems {
		status.Step = i + 1
		status.SystemId = sys.Id
		status.System = sys.Name
		if update != nil {
			update(status)
		}

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

				// --- Build MenuPath with TXT + ZIP rules ---
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
	}

	return idx, nil
}

// -------------------------
// Searching
// -------------------------

func (idx GobIndex) SearchWords(query string) []GobEntry {
	var results []GobEntry
	words := strings.Fields(strings.ToLower(query))
	seen := make(map[string]bool)

	for _, entries := range idx {
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
			if seen[e.SearchName] {
				continue
			}
			seen[e.SearchName] = true
			results = append(results, e)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return strings.ToLower(results[i].SearchName) < strings.ToLower(results[j].SearchName)
	})

	return results
}
