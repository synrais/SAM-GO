package gamesdb

import (
	"bytes"
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

// Return the key for a name in the names index.
func NameKey(systemId string, name string) string {
	return systemId + ":" + name
}

// Check if the gamesdb exists on disk.
func DbExists() bool {
	_, err := os.Stat(config.GamesDb)
	return err == nil
}

// Open the gamesdb with the given options. If the database does not exist it
// will be created and the buckets will be initialized.
func open(options *bolt.Options) (*bolt.DB, error) {
	err := os.MkdirAll(filepath.Dir(config.GamesDb), 0755)
	if err != nil {
		return nil, err
	}

	db, err := bolt.Open(config.GamesDb, 0600, options)
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

// Open the gamesdb with default options for generating names index.
func openNames() (*bolt.DB, error) {
	return open(&bolt.Options{
		NoSync:         true,
		NoFreelistSync: true,
	})
}

// Exported: open gamesdb for write from main/menu rebuild.
func OpenForWrite() (*bolt.DB, error) {
	return openNames()
}

// Read indexed systems
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

// Write list of indexed systems
func writeIndexedSystems(db *bolt.DB, systems []string) error {
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(BucketNames))
		v := b.Get([]byte(indexedSystemsKey))
		if v == nil {
			v = []byte(strings.Join(systems, ","))
			return b.Put([]byte(indexedSystemsKey), v)
		}
		existing := strings.Split(string(v), ",")
		for _, s := range systems {
			if !utils.Contains(existing, s) {
				existing = append(existing, s)
			}
		}
		return b.Put([]byte(indexedSystemsKey), []byte(strings.Join(existing, ",")))
	})
}

// ✅ Exported so main can use it
type FileInfo struct {
	SystemId string
	Path     string
}

// Update the names index with the given files (deduped by SystemId+Name).
func updateNames(db *bolt.DB, files []FileInfo) error {
	return db.Batch(func(tx *bolt.Tx) error {
		bns := tx.Bucket([]byte(BucketNames))
		for _, file := range files {
			base := filepath.Base(file.Path)
			name := strings.TrimSuffix(base, filepath.Ext(base))
			nk := NameKey(file.SystemId, name)
			if err := bns.Put([]byte(nk), []byte(file.Path)); err != nil {
				return err
			}
		}
		return nil
	})
}

// Exported so main can call after menu.db rebuild.
func UpdateNames(db *bolt.DB, files []FileInfo) error {
	if len(files) == 0 {
		return nil
	}
	if err := updateNames(db, files); err != nil {
		return err
	}
	systemIds := make([]string, 0, len(files))
	for _, f := range files {
		if !utils.Contains(systemIds, f.SystemId) {
			systemIds = append(systemIds, f.SystemId)
		}
	}
	return writeIndexedSystems(db, systemIds)
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
		return nil, fmt.Errorf("gamesdb does not exist")
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
			for k, v := c.Seek(pre); k != nil && bytes.HasPrefix(k, pre); k, v = c.Next() {
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

func SearchNamesExact(systems []games.System, query string) ([]SearchResult, error) {
	return searchNamesGeneric(systems, query, func(query, keyName string) bool {
		return strings.EqualFold(query, keyName)
	})
}
func SearchNamesPartial(systems []games.System, query string) ([]SearchResult, error) {
	return searchNamesGeneric(systems, query, func(query, keyName string) bool {
		return strings.Contains(strings.ToLower(keyName), strings.ToLower(query))
	})
}
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
func SearchNamesRegexp(systems []games.System, query string) ([]SearchResult, error) {
	return searchNamesGeneric(systems, query, func(query, keyName string) bool {
		r, err := regexp.Compile(query)
		if err != nil {
			return false
		}
		return r.MatchString(keyName)
	})
}

// Return true if a specific system is indexed in the gamesdb
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

// Return all systems indexed in the gamesdb
func IndexedSystems() ([]string, error) {
	if !DbExists() {
		return nil, fmt.Errorf("gamesdb does not exist")
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

// -------------------------
// Full Indexer with progress
// -------------------------

type IndexStatus struct {
	Total    int
	Step     int
	SystemId string
	Files    int
}

// NewNamesIndex scans all systems, returning all FileInfo while
// calling the update callback for progress display.
func NewNamesIndex(
	cfg *config.UserConfig,
	systems []games.System,
	update func(IndexStatus),
) ([]FileInfo, error) {
	status := IndexStatus{Total: len(systems), Step: 0}
	var out []FileInfo

	for _, sys := range systems {
		status.Step++
		status.SystemId = sys.Id
		update(status)

		paths := games.GetSystemPaths(cfg, []games.System{sys})
		for _, p := range paths {
			pathFiles, err := games.GetFiles(sys.Id, p.Path)
			if err != nil {
				return nil, fmt.Errorf("error getting files for %s: %v", sys.Id, err)
			}
			for pf := range pathFiles {
				out = append(out, FileInfo{SystemId: sys.Id, Path: pathFiles[pf]})
			}
		}
		status.Files = len(out)
		update(status)
	}
	return out, nil
}
