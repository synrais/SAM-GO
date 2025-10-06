package gamesdb

import (
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/games"
	"github.com/synrais/SAM-GO/pkg/utils"
)

// -------------------------
// Types
// -------------------------

// FileInfo represents one indexed game file.
type FileInfo struct {
	SystemId string // Internal system ID
	Name     string // Base name without extension
	Ext      string // File extension (e.g. "nes", "gg")
	Path     string // Full path to file
	MenuPath string // "SystemName/<relative path>"
}

// IndexStatus is used for progress reporting during rebuild.
type IndexStatus struct {
	Total    int
	Step     int
	SystemId string
	Files    int
}

// SearchResult mirrors FileInfo but is used for search returns.
type SearchResult struct {
	SystemId string
	Name     string
	Ext      string
	Path     string
}

// -------------------------
// Helpers
// -------------------------

// DbExists checks if the Gob database (menu.db) exists.
func DbExists() bool {
	_, err := os.Stat(config.MenuDb)
	return err == nil
}

// loadAll loads the full Gob index into memory.
func loadAll() ([]FileInfo, error) {
	f, err := os.Open(config.MenuDb)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var files []FileInfo
	dec := gob.NewDecoder(f)
	if err := dec.Decode(&files); err != nil {
		return nil, err
	}
	return files, nil
}

// saveAll overwrites menu.db with the full set of indexed files.
func saveAll(files []FileInfo) error {
	if err := os.MkdirAll(filepath.Dir(config.MenuDb), 0755); err != nil {
		return err
	}
	f, err := os.Create(config.MenuDb)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := gob.NewEncoder(f)
	return enc.Encode(files)
}

// -------------------------
// Indexing (Gob-only)
// -------------------------

// NewNamesIndex scans all system folders, collects games, and writes menu.db.
func NewNamesIndex(cfg *config.UserConfig, systems []games.System, update func(IndexStatus)) (int, error) {
	status := IndexStatus{
		Total: len(systems) + 1,
		Step:  1,
	}
	update(status)

	var allFiles []FileInfo
	for _, sys := range systems {
		status.SystemId = sys.Id
		status.Step++
		update(status)

		paths := games.GetSystemPaths(cfg, []games.System{sys})
		for _, sp := range paths {
			files, err := games.GetFiles(sys.Id, sp.Path)
			if err != nil {
				return len(allFiles), fmt.Errorf("error reading files for %s: %v", sys.Id, err)
			}
			for _, full := range files {
				base := filepath.Base(full)
				ext := strings.TrimPrefix(filepath.Ext(base), ".")
				name := strings.TrimSuffix(base, filepath.Ext(base))
				menuPath := filepath.ToSlash(filepath.Join(sys.Name, base))

				allFiles = append(allFiles, FileInfo{
					SystemId: sys.Id,
					Name:     name,
					Ext:      ext,
					Path:     full,
					MenuPath: menuPath,
				})
			}
		}
		status.Files = len(allFiles)
	}

	status.Step++
	update(status)

	if err := saveAll(allFiles); err != nil {
		return len(allFiles), err
	}
	return len(allFiles), nil
}

// -------------------------
// Searching (in-memory)
// -------------------------

// searchGeneric runs a flexible in-memory search over all files.
func searchGeneric(query string, test func(string, string) bool) ([]SearchResult, error) {
	files, err := loadAll()
	if err != nil {
		return nil, err
	}

	var results []SearchResult
	for _, f := range files {
		if test(query, f.Name) {
			results = append(results, SearchResult{
				SystemId: f.SystemId,
				Name:     f.Name,
				Ext:      f.Ext,
				Path:     f.Path,
			})
		}
	}
	return results, nil
}

// SearchNamesExact — case-insensitive exact match.
func SearchNamesExact(_ []games.System, query string) ([]SearchResult, error) {
	return searchGeneric(query, func(q, n string) bool {
		return strings.EqualFold(q, n)
	})
}

// SearchNamesPartial — substring match, case-insensitive.
func SearchNamesPartial(_ []games.System, query string) ([]SearchResult, error) {
	q := strings.ToLower(query)
	return searchGeneric(query, func(_ string, n string) bool {
		return strings.Contains(strings.ToLower(n), q)
	})
}

// SearchNamesWords — all words must appear in the name.
func SearchNamesWords(_ []games.System, query string) ([]SearchResult, error) {
	words := strings.Fields(strings.ToLower(query))
	return searchGeneric(query, func(_ string, n string) bool {
		lower := strings.ToLower(n)
		for _, w := range words {
			if !strings.Contains(lower, w) {
				return false
			}
		}
		return true
	})
}

// SearchNamesRegexp — regex-based name match.
func SearchNamesRegexp(_ []games.System, query string) ([]SearchResult, error) {
	return searchGeneric(query, func(q, n string) bool {
		r, err := regexp.Compile(q)
		if err != nil {
			return false
		}
		return r.MatchString(n)
	})
}

// -------------------------
// System Index Helpers
// -------------------------

// IndexedSystems returns all system IDs found in the Gob index.
func IndexedSystems() ([]string, error) {
	files, err := loadAll()
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	for _, f := range files {
		seen[f.SystemId] = true
	}
	return utils.AlphaMapKeys(seen), nil
}

// SystemIndexed returns true if any file in menu.db belongs to the system.
func SystemIndexed(system games.System) bool {
	files, err := loadAll()
	if err != nil {
		return false
	}
	for _, f := range files {
		if f.SystemId == system.Id {
			return true
		}
	}
	return false
}
