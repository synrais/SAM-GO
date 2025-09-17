package attract

import (
	"os"
	"path/filepath"
	"time"
)

// TimestampInfo stores last modification times
type TimestampInfo map[string]time.Time

// loadSavedTimestamps reads timestamps from file (JSON or similar)
func loadSavedTimestamps(gamelistDir string) (TimestampInfo, error) {
	// TODO: implement persistent storage if needed (JSON, TOML, etc.)
	// For now, return an empty map (so first run rebuilds everything).
	return make(TimestampInfo), nil
}

// checkAndHandleModifiedFolder checks if folder has been modified since last run.
func checkAndHandleModifiedFolder(systemID, path, gamelistDir string, savedTimestamps TimestampInfo) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	// Get the folder’s last mod time
	modTime := info.ModTime()
	last, ok := savedTimestamps[path]

	if !ok {
		// First time we see this path → mark as modified
		savedTimestamps[path] = modTime
		return true, nil
	}

	if modTime.After(last) {
		// Folder updated since last run
		savedTimestamps[path] = modTime
		return true, nil
	}

	// No changes
	return false, nil
}
