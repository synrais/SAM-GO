package attract

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SavedTimestamp holds last modified time of a system path
type SavedTimestamp struct {
	SystemID string    `json:"system_id"`
	Path     string    `json:"path"`
	ModTime  time.Time `json:"mod_time"`
}

// Save timestamps to file
func saveTimestamps(gamelistDir string, timestamps []SavedTimestamp) error {
	data, err := json.MarshalIndent(timestamps, "", "  ")
	if err != nil {
		return fmt.Errorf("[Modtime] Failed to encode timestamps: %w", err)
	}
	path := gamelistDir + "/Modtime"
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("[Modtime] Failed to save timestamps: %w", err)
	}
	return nil
}

// Load timestamps from file
func loadSavedTimestamps(gamelistDir string) ([]SavedTimestamp, error) {
	path := gamelistDir + "/Modtime"
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

// Check if folder or any subfolder was modified compared to saved timestamps.
// NOTE: does NOT write; caller should update and save once after all systems.
func isFolderModified(systemID, rootPath string, saved []SavedTimestamp) (bool, time.Time, error) {
	latest := time.Time{}

	// Walk only directories, skip files
	err := filepath.WalkDir(rootPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			// skip bad paths, but keep walking
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		info, err := os.Stat(path)
		if err != nil {
			return nil // ignore broken dirs
		}
		mod := info.ModTime()
		if mod.After(latest) {
			latest = mod
		}
		return nil
	})
	if err != nil {
		if os.IsNotExist(err) {
			return false, time.Time{}, nil
		}
		return false, time.Time{}, fmt.Errorf("[Modtime] Failed to walk %s: %w", rootPath, err)
	}

	// Fallback: if nothing found, stat root directly
	if latest.IsZero() {
		info, err := os.Stat(rootPath)
		if err != nil {
			if os.IsNotExist(err) {
				return false, time.Time{}, nil
			}
			return false, time.Time{}, fmt.Errorf("[Modtime] Failed to stat %s: %w", rootPath, err)
		}
		latest = info.ModTime()
	}

	// Compare to saved
	for _, ts := range saved {
		if ts.SystemID == systemID && ts.Path == rootPath {
			return latest.After(ts.ModTime), latest, nil
		}
	}

	// no record yet → treat as modified
	return true, latest, nil
}

// Update or insert timestamp
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
