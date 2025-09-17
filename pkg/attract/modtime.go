package attract

import (
	"encoding/json"
	"fmt"
	"os"
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
	path := gamelistDir + "/modtime.json"
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("[Modtime] Failed to save timestamps: %w", err)
	}
	return nil
}

// Load timestamps from file
func loadSavedTimestamps(gamelistDir string) ([]SavedTimestamp, error) {
	path := gamelistDir + "/modtime.json"
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

// Check if folder was modified compared to saved timestamps
func checkAndHandleModifiedFolder(systemID, path, gamelistDir string, saved []SavedTimestamp) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("[Modtime] Failed to stat %s: %w", path, err)
	}
	currentMod := info.ModTime()

	for _, ts := range saved {
		if ts.SystemID == systemID && ts.Path == path {
			if currentMod.After(ts.ModTime) {
				// Folder modified → update timestamp
				newList := updateTimestamp(saved, systemID, path, currentMod)
				_ = saveTimestamps(gamelistDir, newList)
				return true, nil
			}
			return false, nil
		}
	}

	// No record → new entry
	newList := append(saved, SavedTimestamp{
		SystemID: systemID,
		Path:     path,
		ModTime:  currentMod,
	})
	_ = saveTimestamps(gamelistDir, newList)
	return true, nil
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
