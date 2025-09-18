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
// Check if a folder or any subfolder was modified compared to saved timestamps.
// Only considers directories, not individual files.
// Check if a folder or any subfolder was modified compared to saved timestamps.
// Only considers directories, not individual files.
func isFolderModified(systemID, path string, saved []SavedTimestamp) (bool, time.Time, error) {
	var latestMod time.Time
	modified := false

	err := filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			// skip problematic subdirs
			return nil
		}
		if !d.IsDir() {
			// ignore files entirely
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

	// compare against saved record (only root path entry is stored)
	for _, ts := range saved {
		if ts.SystemID == systemID && ts.Path == path {
			modified = latestMod.After(ts.ModTime)
			return modified, latestMod, nil
		}
	}

	// no record yet → treat as modified
	return true, latestMod, nil
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
