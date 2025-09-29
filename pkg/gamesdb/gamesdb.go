package gamesdb

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	bolt "go.etcd.io/bbolt"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/games"
	"github.com/synrais/SAM-GO/pkg/utils"
)

const (
	BucketNames       = "names"
	indexedSystemsKey = "meta:indexedSystems"
)

// -------------------------
// Helpers
// -------------------------

// Return the key for a name in the names index.
// Format: systemId:name:ext
func NameKey(systemId, name, ext string) string {
	return fmt.Sprintf("%s:%s:%s", systemId, name, ext)
}

// Check if the games.db exists on disk.
func DbExists() bool {
	_, err := os.Stat(config.GamesDb)
	return err == nil
}

// -------------------------
// DB Management
// -------------------------

// Open the games.db with the given options. Creates the DB if missing.
func open(options *bolt.Options) (*bolt.DB, error) {
	err := os.MkdirAll(filepath.Dir(config.GamesDb), 0755)
	if err != nil {
		return nil, err
	}

	db, err := bolt.Open(config.GamesDb, 0600, options)
	if err != nil {
		return nil, err
	}

	// Ensure required buckets exist
	db.Update(func(txn *bolt.Tx) error {
		for _, bucket := range []string{BucketNames} {
			_, err := txn.CreateBucketIfNotExists([]byte(bucket))
			if err != nil {
				return err
			}
		}
		return nil
	})

	return db, nil
}

// Open the games.db with default options for generating names index.
func openNames() (*bolt.DB, error) {
	return open(&bolt.Options{
		NoSync:         true,
		NoFreelistSync: true,
	})
}

func readIndexedSystems(db *bolt.DB) ([]string, error) {
	var systems []string

	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(BucketNames))
		v := b.Get([]byte(indexedSystemsKey))
		if v != nil {
			systems = strings.Split(string(v), ",")
		}
		return nil
	})

	return systems, err
}

func writeIndexedSystems(db *bolt.DB, systems []string) error {
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(BucketNames))
		v := b.Get([]byte(indexedSystemsKey))
		if v == nil {
			v = []byte(strings.Join(systems, ","))
			return b.Put([]byte(indexedSystemsKey), v)
		} else {
			existing := strings.Split(string(v), ",")
			for _, s := range systems {
				if !utils.Contains(existing, s) {
					existing = append(existing, s)
				}
			}
			return b.Put([]byte(indexedSystemsKey), []byte(strings.Join(existing, ",")))
		}
	})
}

// -------------------------
// Indexing
// -------------------------

// Update the names index with the given files.
func updateNames(db *bolt.DB, files []fileinfo) error {
	return db.Batch(func(tx *bolt.Tx) error {
		bns := tx.Bucket([]byte(BucketNames))

		for _, file := range files {
			nk := NameKey(file.SystemId, file.Name, file.Ext)
			err := bns.Put([]byte(nk), []byte(file.Path))
			if err != nil {
				return err
			}
		}
		return nil
	})
}

type IndexStatus struct {
	Total    int
	Step     int
	SystemId string
	Files    int
}

// Enriched file information (also written into menu.db Gob).
type fileinfo struct {
	SystemId     string // Internal system ID
	SystemName   string // Friendly system name (e.g. "Arcadia 2001")
	SystemFolder string // Root folder on disk for this system
	Name         string // Base name without extension
	Ext          string // File extension (e.g. "nes", "gg")
	Path         string // Full path to file
	MenuPath     string // "SystemName/<relative path under SystemFolder>"
}

// Build a new names index and Gob file from all systems and their game files.
func NewNamesIndex(
	cfg *config.UserConfig,
	systems []games.System,
	update func(IndexStatus),
) (int, error) {
	status := IndexStatus{
		Total: len(systems) + 2, // +1 for games.db write, +1 for menu.db write
		Step:  1,
	}

	db, err := openNames()
	if err != nil {
		return status.Files, fmt.Errorf("error opening games.db: %s", err)
	}
	defer db.Close()

	update(status)

	// Collect all paths per system
	systemPaths := make(map[string][]string, 0)
	for _, v := range games.GetSystemPaths(cfg, systems) {
		systemPaths[v.System.Id] = append(systemPaths[v.System.Id], v.Path)
	}

	var allFiles []fileinfo

	for _, k := range utils.AlphaMapKeys(systemPaths) {
		status.SystemId = k
		status.Step++
		update(status)

		sys, err := games.GetSystem(k)
		if err != nil {
			return status.Files, fmt.Errorf("unknown system: %s", k)
		}

		files := make([]fileinfo, 0)

		for _, path := range systemPaths[k] {
			pathFiles, err := games.GetFiles(k, path)
			if err != nil {
				return status.Files, fmt.Errorf("error getting files: %s", err)
			}

			if len(pathFiles) == 0 {
				continue
			}

			for _, fullPath := range pathFiles {
				base := filepath.Base(fullPath)
				ext := strings.TrimPrefix(filepath.Ext(base), ".")
				name := strings.TrimSuffix(base, filepath.Ext(base))

				// -------------------------
				// Build MenuPath
				// -------------------------
				menuPath := ""
				found := false
				parts := strings.Split(filepath.ToSlash(fullPath), "/")

				for i, part := range parts {
					for _, folder := range sys.Folder {
						if part == folder {
							// everything after the system folder
							relParts := parts[i+1:]

							if len(relParts) > 0 {
								// Case 1: collapse fake .zip folder
								if strings.HasSuffix(relParts[0], ".zip") {
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

				// Fallback if no folder matched
				if !found {
					menuPath = filepath.ToSlash(filepath.Join(sys.Name, base))
				}

				files = append(files, fileinfo{
					SystemId:     sys.Id,
					SystemName:   sys.Name,
					SystemFolder: sys.Folder[0],
					Name:         name,
					Ext:          ext,
					Path:         fullPath,
					MenuPath:     menuPath,
				})
			}
		}

		if len(files) == 0 {
			continue
		}

		status.Files += len(files)
		allFiles = append(allFiles, files...)

		// Update Bolt DB
		if err := updateNames(db, files); err != nil {
			return status.Files, err
		}
	}

	// --- Finalize Bolt ---
	status.Step++
	status.SystemId = fmt.Sprintf("writing %s", filepath.Base(config.GamesDb))
	update(status)

	if err := writeIndexedSystems(db, utils.AlphaMapKeys(systemPaths)); err != nil {
		return status.Files, fmt.Errorf("error writing indexed systems: %s", err)
	}

	if err := db.Sync(); err != nil {
		return status.Files, fmt.Errorf("error syncing database: %s", err)
	}

	// --- Write Gob (menu.db) ---
	status.Step++
	status.SystemId = fmt.Sprintf("writing %s", filepath.Base(config.MenuDb))
	update(status)

	gobFile, err := os.Create(config.MenuDb)
	if err != nil {
		return status.Files, fmt.Errorf("error creating gob file: %s", err)
	}
	defer gobFile.Close()

	encoder := gob.NewEncoder(gobFile)
	if err := encoder.Encode(allFiles); err != nil {
		return status.Files, fmt.Errorf("error writing gob file: %s", err)
	}

	return status.Files, nil
}

// -------------------------
// Searching
// -------------------------

type SearchResult struct {
	SystemId string
	Name     string
	Ext      string
	Path     string
}

func searchNamesGeneric(
	systems []games.System,
	query string,
	test func(string, string) bool,
) ([]SearchResult, error) {
	if !DbExists() {
		return nil, fmt.Errorf("games.db does not exist")
	}

	db, err := open(&bolt.Options{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var results []SearchResult

	err = db.View(func(tx *bolt.Tx) error {
		bn := tx.Bucket([]byte(BucketNames))

		for _, system := range systems {
			pre := []byte(system.Id + ":")
			c := bn.Cursor()
			for k, v := c.Seek(pre); k != nil && bytes.HasPrefix(k, pre); k, v = c.Next() {
				// key = systemId:name:ext
				parts := strings.SplitN(string(k), ":", 3)
				if len(parts) != 3 {
					continue
				}
				name := parts[1]
				ext := parts[2]

				if test(query, name) {
					results = append(results, SearchResult{
						SystemId: system.Id,
						Name:     name,
						Ext:      ext,
						Path:     string(v),
					})
				}
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return results, nil
}

// Exact match (case-insensitive).
func SearchNamesExact(systems []games.System, query string) ([]SearchResult, error) {
	return searchNamesGeneric(systems, query, func(query, name string) bool {
		return strings.EqualFold(query, name)
	})
}

// Partial substring match (case-insensitive).
func SearchNamesPartial(systems []games.System, query string) ([]SearchResult, error) {
	return searchNamesGeneric(systems, query, func(query, name string) bool {
		return strings.Contains(strings.ToLower(name), strings.ToLower(query))
	})
}

// Word-by-word match (all words must be present).
func SearchNamesWords(systems []games.System, query string) ([]SearchResult, error) {
	return searchNamesGeneric(systems, query, func(query, name string) bool {
		qWords := strings.Fields(strings.ToLower(query))
		for _, word := range qWords {
			if !strings.Contains(strings.ToLower(name), word) {
				return false
			}
		}
		return true
	})
}

// Regex-based match.
func SearchNamesRegexp(systems []games.System, query string) ([]SearchResult, error) {
	return searchNamesGeneric(systems, query, func(query, name string) bool {
		r, err := regexp.Compile(query)
		if err != nil {
			return false
		}
		return r.MatchString(name)
	})
}

// -------------------------
// System Index Helpers
// -------------------------

// Return true if a specific system is indexed.
func SystemIndexed(system games.System) bool {
	if !DbExists() {
		return false
	}

	db, err := open(&bolt.Options{ReadOnly: true})
	if err != nil {
		return false
	}
	defer db.Close()

	systems, err := readIndexedSystems(db)
	if err != nil {
		return false
	}

	return utils.Contains(systems, system.Id)
}

// Return all indexed system IDs.
func IndexedSystems() ([]string, error) {
	if !DbExists() {
		return nil, fmt.Errorf("games.db does not exist")
	}

	db, err := open(&bolt.Options{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer db.Close()

	systems, err := readIndexedSystems(db)
	if err != nil {
		return nil, err
	}

	return systems, nil
}
