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
	"golang.org/x/sync/errgroup"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/games"
	"github.com/synrais/SAM-GO/pkg/utils"
)

const (
	BucketNames       = "names"
	indexedSystemsKey = "meta:indexedSystems"
)

// Return the key for a name in the names index.
func NameKey(systemId string, name string) string {
	return systemId + ":" + name
}

// Check if the search.db exists on disk.
func DbExists() bool {
	_, err := os.Stat(config.SearchDb)
	return err == nil
}

// Open the search.db with the given options. If the database does not exist it
// will be created and the buckets will be initialized.
func open(options *bolt.Options) (*bolt.DB, error) {
	err := os.MkdirAll(filepath.Dir(config.SearchDb), 0755)
	if err != nil {
		return nil, err
	}

	db, err := bolt.Open(config.SearchDb, 0600, options)
	if err != nil {
		return nil, err
	}

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

// Open the search.db with default options for generating names index.
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

// Enriched file information.
type fileInfo struct {
	SystemId   string
	Name       string // base name without extension
	NameExt    string // filename with extension
	Path       string // full path
	FolderName string // parent folder name
}

// Update the names index with the given files and also piggyback write into gob.
func updateNames(db *bolt.DB, encoder *gob.Encoder, files []fileInfo) error {
    return db.Batch(func(tx *bolt.Tx) error {
        bns := tx.Bucket([]byte(BucketNames))

        for _, file := range files {
            nk := NameKey(file.SystemId, file.Name)
            if err := bns.Put([]byte(nk), []byte(file.Path)); err != nil {
                return err
            }

            // Piggyback into gob
            if err := encoder.Encode(file); err != nil {
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

// Given a list of systems, index all valid game files on disk and write a
// names index to the DB. Overwrites any existing names index, but does not
// clean up old missing files.
func NewNamesIndex(
    cfg *config.UserConfig,
    systems []games.System,
    update func(IndexStatus),
) (int, error) {
    status := IndexStatus{
        Total: len(systems) + 1,
        Step:  1,
    }

    db, err := openNames()
    if err != nil {
        return status.Files, fmt.Errorf("error opening search.db: %s", err)
    }
    defer db.Close()

    // Open gob file ONCE up front
    gobFile, err := os.Create(config.GamesDb)
    if err != nil {
        return status.Files, fmt.Errorf("error creating gob file: %s", err)
    }
    defer gobFile.Close()
    encoder := gob.NewEncoder(gobFile)

    update(status)
    systemPaths := make(map[string][]string, 0)
    for _, v := range games.GetSystemPaths(cfg, systems) {
        systemPaths[v.System.Id] = append(systemPaths[v.System.Id], v.Path)
    }

    for _, k := range utils.AlphaMapKeys(systemPaths) {
        status.SystemId = k
        status.Step++
        update(status)

        files := make([]fileInfo, 0)

        for _, path := range systemPaths[k] {
            pathFiles, err := games.GetFiles(k, path)
            if err != nil {
                return status.Files, fmt.Errorf("error getting files: %s", err)
            }

            for _, fullPath := range pathFiles {
                base := filepath.Base(fullPath)
                ext := filepath.Ext(base)
                name := strings.TrimSuffix(base, ext)
                folder := filepath.Base(filepath.Dir(fullPath))

                files = append(files, fileInfo{
                    SystemId:   k,
                    Name:       name,
                    NameExt:    base,
                    Path:       fullPath,
                    FolderName: folder,
                })
            }
        }

        if len(files) == 0 {
            continue
        }

        status.Files += len(files)

        // Write both Bolt + Gob sequentially
        if err := updateNames(db, encoder, files); err != nil {
            return status.Files, err
        }
    }

    status.Step++
    status.SystemId = ""
    update(status)

    if err := writeIndexedSystems(db, utils.AlphaMapKeys(systemPaths)); err != nil {
        return status.Files, fmt.Errorf("error writing indexed systems: %s", err)
    }

    if err := db.Sync(); err != nil {
        return status.Files, fmt.Errorf("error syncing database: %s", err)
    }

    return status.Files, nil
}

type SearchResult struct {
	SystemId string
	Name     string
	Path     string
}

// Iterate all indexed names and return matches to test func against query.
func searchNamesGeneric(
	systems []games.System,
	query string,
	test func(string, string) bool,
) ([]SearchResult, error) {
	if !DbExists() {
		return nil, fmt.Errorf("search.db does not exist")
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
			nameIdx := bytes.Index(pre, []byte(":"))

			c := bn.Cursor()
			for k, v := c.Seek([]byte(pre)); k != nil && bytes.HasPrefix(k, pre); k, v = c.Next() {
				keyName := string(k[nameIdx+1:])

				if test(query, keyName) {
					results = append(results, SearchResult{
						SystemId: system.Id,
						Name:     keyName,
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

// Return indexed names matching exact query (case insensitive).
func SearchNamesExact(systems []games.System, query string) ([]SearchResult, error) {
	return searchNamesGeneric(systems, query, func(query, keyName string) bool {
		return strings.EqualFold(query, keyName)
	})
}

// Return indexed names partially matching query (case insensitive).
func SearchNamesPartial(systems []games.System, query string) ([]SearchResult, error) {
	return searchNamesGeneric(systems, query, func(query, keyName string) bool {
		return strings.Contains(strings.ToLower(keyName), strings.ToLower(query))
	})
}

// Return indexed names that include every word in query (case insensitive).
func SearchNamesWords(systems []games.System, query string) ([]SearchResult, error) {
	return searchNamesGeneric(systems, query, func(query, keyName string) bool {
		qWords := strings.Fields(strings.ToLower(query))

		for _, word := range qWords {
			if !strings.Contains(strings.ToLower(keyName), word) {
				return false
			}
		}

		return true
	})
}

// Return indexed names matching query using regular expression.
func SearchNamesRegexp(systems []games.System, query string) ([]SearchResult, error) {
	return searchNamesGeneric(systems, query, func(query, keyName string) bool {
		r, err := regexp.Compile(query)
		if err != nil {
			return false
		}

		return r.MatchString(keyName)
	})
}

// Return true if a specific system is indexed in the search.db
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

// Return all systems indexed in the search.db
func IndexedSystems() ([]string, error) {
	if !DbExists() {
		return nil, fmt.Errorf("search.db does not exist")
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
