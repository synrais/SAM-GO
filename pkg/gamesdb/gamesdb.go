package gamesdb

import (
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/games"
	"github.com/synrais/SAM-GO/pkg/utils"
)

// -------------------------
// Types
// -------------------------

type FileInfo struct {
	SystemId string
	Name     string
	Ext      string
	Path     string
	MenuPath string
}

type IndexStatus struct {
	Total    int
	Step     int
	SystemId string
	Files    int
}

type SearchResult struct {
	SystemId string
	Name     string
	Ext      string
	Path     string
}

// -------------------------
// Global in-memory cache
// -------------------------

var cachedFiles []FileInfo
var cacheLoaded bool

// -------------------------
// Helpers
// -------------------------

func DbExists() bool {
	_, err := os.Stat(config.MenuDb)
	return err == nil
}

// LoadAll returns every entry in the games database.
func LoadAll() ([]FileInfo, error) {
	return loadAll()
}

func loadAll() ([]FileInfo, error) {
	// If we've already loaded the Gob file once, return the cached version instantly.
	if cacheLoaded {
		return cachedFiles, nil
	}

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

	cachedFiles = files
	cacheLoaded = true
	return cachedFiles, nil
}

func saveAll(files []FileInfo) error {
	if err := os.MkdirAll(filepath.Dir(config.MenuDb), 0755); err != nil {
		return err
	}
	f, err := os.Create(config.MenuDb)
	if err != nil {
		return err
	}
	defer f.Close()
	return gob.NewEncoder(f).Encode(files)
}

// -------------------------
// Indexing
// -------------------------

func NewNamesIndex(cfg *config.Config, systems []games.System, update func(IndexStatus)) (int, error) {
	_ = os.Remove(config.MenuDb)
	status := IndexStatus{
		Total: len(systems) + 1,
		Step:  1,
	}
	update(status)

	var allFiles []FileInfo

	// [Database] Exclude drops whole systems; [Database.X] rules drop games.
	excluded, _ := games.ResolveSystems(cfg.Database.Exclude)
	rules := NewRuleSet(cfg.DatabaseRules)

	for _, sys := range systems {
		status.SystemId = sys.Id
		status.Step++
		update(status)

		if excluded[strings.ToLower(sys.Id)] {
			continue
		}

		sysPaths := games.GetSystemPaths(cfg, []games.System{sys})
		for _, sp := range sysPaths {
			pathFiles, err := games.GetFiles(sys.Id, sp.Path)
			if err != nil {
				return len(allFiles), fmt.Errorf("error getting files: %v", err)
			}

			for _, fullPath := range pathFiles {
				base := filepath.Base(fullPath)
				ext := strings.TrimPrefix(filepath.Ext(base), ".")
				name := strings.TrimSuffix(base, filepath.Ext(base))

				menuPath := menuPathFor(sys, sp.Path, fullPath)

				file := FileInfo{
					SystemId: sys.Id,
					Name:     name,
					Ext:      ext,
					Path:     fullPath,
					MenuPath: menuPath,
				}
				if rules.Excludes(file) {
					continue
				}
				allFiles = append(allFiles, file)
			}
		}
		status.Files = len(allFiles)
	}

	status.Step++
	update(status)

	if err := saveAll(allFiles); err != nil {
		return len(allFiles), err
	}

	// Update in-memory cache immediately after building
	cachedFiles = allFiles
	cacheLoaded = true

	return len(allFiles), nil
}

// menuPathFor builds a game's menu path, e.g. "SNES/RPG/Chrono Trigger.sfc":
// the system's name, then the game's folders inside the system folder that
// was scanned (root), so any folder layout keeps its subfolders.
func menuPathFor(sys games.System, root, fullPath string) string {
	rel, err := filepath.Rel(root, fullPath)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(filepath.Join(sys.Name, filepath.Base(fullPath)))
	}
	relParts := strings.Split(filepath.ToSlash(rel), "/")

	// Case 1: collapse a fake .zip folder (e.g. "N64.zip/...")
	if len(relParts) > 1 && strings.HasSuffix(strings.ToLower(relParts[0]), ".zip") {
		relParts = relParts[1:]
	}

	// Case 2: listings/*.txt → label
	if len(relParts) > 1 && relParts[0] == "listings" && strings.HasSuffix(relParts[1], ".txt") {
		label := strings.TrimSuffix(relParts[1], ".txt")
		if len(label) > 0 {
			label = strings.ToUpper(label[:1]) + label[1:]
		}
		relParts = append([]string{label}, relParts[2:]...)
	}

	return filepath.ToSlash(filepath.Join(append([]string{sys.Name}, relParts...)...))
}

// -------------------------
// Searching
// -------------------------

func searchGeneric(query string, test func(name, ext string) bool) ([]SearchResult, error) {
	files, err := loadAll()
	if err != nil {
		return nil, err
	}

	results := make([]SearchResult, 0, 128)
	seen := make(map[string]bool) // key = system|name|ext

	for _, f := range files {
		if test(f.Name, f.Ext) {
			// The same game on two systems is listed under both; the same
			// file on two drives (same system) only once.
			key := strings.ToLower(fmt.Sprintf("%s|%s|%s", f.SystemId, f.Name, f.Ext))
			if seen[key] {
				continue
			}
			seen[key] = true

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

func SearchNamesWords(_ []games.System, query string) ([]SearchResult, error) {
	words := strings.Fields(strings.ToLower(query))

	// Extract any explicit extension filters (e.g. ".nes")
	var extFilters []string
	for _, w := range words {
		if strings.HasPrefix(w, ".") && len(w) > 1 {
			extFilters = append(extFilters, strings.TrimPrefix(w, "."))
		}
	}

	return searchGeneric(query, func(name, ext string) bool {
		nameLow := strings.ToLower(name)
		extLow := strings.ToLower(ext)

		// If user included ".ext" in the query → strict extension filter
		if len(extFilters) > 0 {
			match := false
			for _, e := range extFilters {
				if e == extLow {
					match = true
					break
				}
			}
			if !match {
				return false
			}
		}

		// Must match all remaining non-extension words in the name
		for _, w := range words {
			if strings.HasPrefix(w, ".") {
				continue // skip explicit extensions
			}
			if !strings.Contains(nameLow, w) {
				return false
			}
		}

		return true
	})
}

// -------------------------
// System Index Helpers
// -------------------------

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
