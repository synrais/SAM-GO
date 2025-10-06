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

				// --- Original MenuPath logic restored ---
				menuPath := ""
				found := false
				parts := strings.Split(filepath.ToSlash(fullPath), "/")

				for i, part := range parts {
					for _, folder := range sys.Folder {
						if part == folder {
							relParts := parts[i+1:]

							if len(relParts) > 0 {
								// Case 1: collapse fake .zip folder
								if strings.HasSuffix(relParts[0], ".zip") {
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
							}

							menuPath = filepath.ToSlash(filepath.Join(append([]string{sys.Name}, relParts...)...))
							found = true
							break
						}
					}
					if found {
						break
					}
				}

				// Fallback if no system folder matched
				if !found {
					menuPath = filepath.ToSlash(filepath.Join(sys.Name, base))
				}

				allFiles = append(allFiles, FileInfo{
					SystemId: sys.Id,
					Name:     name,
					Ext:      ext,
					Path:     fullPath,
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

	// Update in-memory cache immediately after building
	cachedFiles = allFiles
	cacheLoaded = true

	return len(allFiles), nil
}

// -------------------------
// Searching
// -------------------------

func searchGeneric(query string, test func(string, string) bool) ([]SearchResult, error) {
	files, err := loadAll()
	if err != nil {
		return nil, err
	}

	results := make([]SearchResult, 0, 128)
	seen := make(map[string]bool) // key = name|ext

	for _, f := range files {
		if test(query, f.Name) {
			// Build a normalized key for deduplication
			key := strings.ToLower(fmt.Sprintf("%s|%s", f.Name, f.Ext))
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
	return searchGeneric(query, func(_ string, n string) bool {
		low := strings.ToLower(n)
		for _, w := range words {
			if !strings.Contains(low, w) {
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
