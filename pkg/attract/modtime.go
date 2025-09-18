package attract

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SavedTimestamp holds last modified time of a system path.
type SavedTimestamp struct {
	SystemID string    `json:"system_id"`
	Path     string    `json:"path"`
	ModTime  time.Time `json:"mod_time"`
}

// saveTimestamps writes all tracked system path modtimes to disk.
func saveTimestamps(gamelistDir string, timestamps []SavedTimestamp) error {
	data, err := json.MarshalIndent(timestamps, "", "  ")
	if err != nil {
		return fmt.Errorf("[Modtime] Failed to encode timestamps: %w", err)
	}
	path := filepath.Join(gamelistDir, "Modtime")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("[Modtime] Failed to save timestamps: %w", err)
	}
	return nil
}

// loadSavedTimestamps loads all saved modtimes from disk.
// Returns an empty slice if no file exists.
func loadSavedTimestamps(gamelistDir string) ([]SavedTimestamp, error) {
	path := filepath.Join(gamelistDir, "Modtime")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []SavedTimestamp{}, nil
		}
		return nil, fmt.Errorf("[Modtime] Failed to read file: %w", err)
	}
	var timestamps []SavedTimestamp
	if err := json.Unmarshal(data, &timestamps); err != nil {
		return nil, fmt.Errorf("[Modtime] Failed to parse JSON: %w", err)
	}
	return timestamps, nil
}

// isFolderModified checks if a folder or any subfolder was modified
// since the last saved timestamp. Only considers directories, not files.
func isFolderModified(systemID, path string, saved []SavedTimestamp) (bool, time.Time, error) {
	var latestMod time.Time

	err := filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			// skip problematic subdirs
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		mod := info.ModTime()
		if mod.After(latestMod) {
			latestMod = mod
		}
		return nil
	})
	if err != nil {
		if os.IsNotExist(err) {
			return false, time.Time{}, nil
		}
		return false, time.Time{}, fmt.Errorf("[Modtime] Walk failed for %s: %w", path, err)
	}

	// Compare against saved record
	for _, ts := range saved {
		if ts.SystemID == systemID && ts.Path == path {
			return latestMod.After(ts.ModTime), latestMod, nil
		}
	}

	// No record yet → treat as modified
	return true, latestMod, nil
}

// updateTimestamp updates or inserts a system path record with the given modtime.
func updateTimestamp(list []SavedTimestamp, systemID, path string, mod time.Time) []SavedTimestamp {
	for i, ts := range list {
		if ts.SystemID == systemID && ts.Path == path {
			list[i].ModTime = mod
			return list
		}
	}
	return append(list, SavedTimestamp{
		SystemID: systemID,
		Path:     path,
		ModTime:  mod,
	})
}
