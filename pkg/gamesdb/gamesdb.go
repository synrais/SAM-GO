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

//
// ---------------------------------------------------
// Helpers
// ---------------------------------------------------
//

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

//
// ---------------------------------------------------
// Indexed Systems
// ---------------------------------------------------
//

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

//
// ---------------------------------------------------
// File Indexing
// ---------------------------------------------------
//

type FileInfo struct {
	SystemId string
	Path     string
}

// Update the names index with a batch of files.
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

// Exported: batch update after menu rebuild (kept for compatibility).
func UpdateNames(db *bolt.DB, files []FileInfo) error {
	if len(files) == 0 {
		return nil
	}
	if err := updateNames(db, files); err != nil {
		return err
	}

	// Track indexed systems
	systemIds := make([]string, 0, len(files))
	for _, f := range files {
		if !utils.Contains(systemIds, f.SystemId) {
			systemIds = append(systemIds, f.SystemId)
		}
	}
	if err := writeIndexedSystems(db, systemIds); err != nil {
		return err
	}

	// ✅ Force flush once at the end
	return db.Sync()
}

//
// ---------------------------------------------------
// Search
// ---------------------------------------------------
//

type SearchResult struct {
	SystemId string
	Name     string
	Path     string
}

// Generic search over all indexed names.
func searchNamesGeneric(
	systems []games.System,
	query string,
	test func(string) bool,
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
				if test(keyName) {
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

// Exact match (case-insensitive)
func SearchNamesExact(systems []games.System, query string) ([]SearchResult, error) {
	q := strings.ToLower(query)
	return searchNamesGeneric(systems, query, func(keyName string) bool {
		return strings.ToLower(keyName) == q
	})
}

// Partial substring match (case-insensitive)
func SearchNamesPartial(systems []games.System, query string) ([]SearchResult, error) {
	q := strings.ToLower(query)
	return searchNamesGeneric(systems, query, func(keyName string) bool {
		return strings.Contains(strings.ToLower(keyName), q)
	})
}

// Match all words in query (case-insensitive)
func SearchNamesWords(systems []games.System, query string) ([]SearchResult, error) {
	words := strings.Fields(strings.ToLower(query))
	return searchNamesGeneric(systems, query, func(keyName string) bool {
		lowerName := strings.ToLower(keyName)
		for _, w := range words {
			if !strings.Contains(lowerName, w) {
				return false
			}
		}
		return true
	})
}

// Regex search
func SearchNamesRegexp(systems []games.System, query string) ([]SearchResult, error) {
	r, err := regexp.Compile(query)
	if err != nil {
		return nil, err
	}
	return searchNamesGeneric(systems, query, func(keyName string) bool {
		return r.MatchString(keyName)
	})
}

//
// ---------------------------------------------------
// Indexing with progress (Wizzo-style sequential Bolt writes)
// ---------------------------------------------------
//

type IndexStatus struct {
	Total    int
	Step     int
	SystemId string
	Files    int
}

// NewNamesIndex scans systems sequentially and writes each system immediately.
func NewNamesIndex(
	cfg *config.UserConfig,
	systems []games.System,
	update func(IndexStatus),
) ([]FileInfo, error) {
	status := IndexStatus{Total: len(systems)}
	var out []FileInfo

	db, err := OpenForWrite()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	for i, sys := range systems {
		paths := games.GetSystemPaths(cfg, []games.System{sys})

		var sysFiles []FileInfo
		for _, p := range paths {
			files, err := games.GetFiles(sys.Id, p.Path)
			if err != nil {
				return nil, fmt.Errorf("error getting files for %s: %w", sys.Id, err)
			}
			for _, f := range files {
				sysFiles = append(sysFiles, FileInfo{SystemId: sys.Id, Path: f})
			}
		}

		// Write this system immediately into Bolt
		if err := updateNames(db, sysFiles); err != nil {
			return nil, err
		}
		if err := writeIndexedSystems(db, []string{sys.Id}); err != nil {
			return nil, err
		}

		out = append(out, sysFiles...)

		// Report progress
		status.Step = i + 1
		status.SystemId = sys.Id
		status.Files = len(sysFiles)
		update(status)
	}

	// ✅ Final flush
	_ = db.Sync()

	return out, nil
}

//
// ---------------------------------------------------
// Public system index queries
// ---------------------------------------------------
//

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
