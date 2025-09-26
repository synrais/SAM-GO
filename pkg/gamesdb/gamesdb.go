package gamesdb

import (
	"bytes"
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

type fileInfo struct {
	SystemId string
	Path     string
}

// Update the names index with the given files (deduped by SystemId+Name).
func updateNames(db *bolt.DB, files []fileInfo) error {
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
func UpdateNames(db *bolt.DB, files []fileInfo) error {
	return updateNames(db, files)
}

type IndexStatus struct {
	Total    int
	Step     int
	SystemId string
	Files    int
}

// Old path: still available if you want to directly build games.db off disk.
func NewNamesIndex(
	cfg *config.UserConfig,
	systems []games.System,
	update func(IndexStatus),
) (int, error) {
	status := IndexStatus{Total: len(systems) + 1, Step: 1}

	db, err := openNames()
	if err != nil {
		return status.Files, fmt.Errorf("error opening gamesdb: %s", err)
	}
	defer db.Close()

	update(status)
	systemPaths := make(map[string][]string, 0)
	for _, v := range games.GetSystemPaths(cfg, systems) {
		systemPaths[v.System.Id] = append(systemPaths[v.System.Id], v.Path)
	}

	g := new(errgroup.Group)
	for _, k := range utils.AlphaMapKeys(systemPaths) {
		status.SystemId = k
		status.Step++
		update(status)

		var files []fileInfo
		for _, path := range systemPaths[k] {
			pathFiles, err := games.GetFiles(k, path)
			if err != nil {
				return status.Files, fmt.Errorf("error getting files: %s", err)
			}
			for pf := range pathFiles {
				files = append(files, fileInfo{SystemId: k, Path: pathFiles[pf]})
			}
		}

		if len(files) == 0 {
			continue
		}
		status.Files += len(files)

		g.Go(func() error {
			return updateNames(db, files)
		})
	}

	status.Step++
	status.SystemId = ""
	update(status)

	if err := g.Wait(); err != nil {
		return status.Files, fmt.Errorf("error updating names index: %s", err)
	}
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
