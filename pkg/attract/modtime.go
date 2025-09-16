package attract

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
	"io/ioutil"
)

// Struct for storing system folder modification timestamps
type FolderTimestamps map[string]time.Time

// Function to get the last modified timestamp of a system folder
func getFolderTimestamp(folderPath string) (time.Time, error) {
	info, err := os.Stat(folderPath)
	if err != nil {
		return time.Time{}, fmt.Errorf("unable to get folder info for %s: %w", folderPath, err)
	}
	return info.ModTime(), nil
}

// Function to load the saved timestamps from the Modtime file
func loadSavedTimestamps(gamelistDir string) (FolderTimestamps, error) {
	modtimeFilePath := filepath.Join(gamelistDir, "Modtime")
	data, err := ioutil.ReadFile(modtimeFilePath)
	if err != nil {
		// If the file doesn't exist, return an empty map (this is fine for first time use)
		if os.IsNotExist(err) {
			return FolderTimestamps{}, nil
		}
		return nil, fmt.Errorf("unable to read Modtime file: %w", err)
	}

	// Parse the saved timestamps from the Modtime file (JSON format)
	var timestamps FolderTimestamps
	err = json.Unmarshal(data, &timestamps)
	if err != nil {
		return nil, fmt.Errorf("invalid timestamp data in Modtime file: %w", err)
	}
	return timestamps, nil
}

// Function to save the current timestamps to the Modtime file
func saveTimestamps(gamelistDir string, timestamps FolderTimestamps) error {
	modtimeFilePath := filepath.Join(gamelistDir, "Modtime")
	data, err := json.MarshalIndent(timestamps, "", "  ")
	if err != nil {
		return fmt.Errorf("unable to marshal timestamps: %w", err)
	}

	// Save the serialized timestamps to the Modtime file
	err = ioutil.WriteFile(modtimeFilePath, data, 0644)
	if err != nil {
		return fmt.Errorf("unable to save Modtime file: %w", err)
	}
	return nil
}

// Function to check if the system folder has changed since the last scan
func isFolderModified(systemId, folderPath, gamelistDir string, savedTimestamps FolderTimestamps) (bool, error) {
	// Get the current timestamp of the system folder
	currentTimestamp, err := getFolderTimestamp(folderPath)
	if err != nil {
		return false, err
	}

	// Check if we have a saved timestamp for this system
	savedTimestamp, exists := savedTimestamps[systemId]
	if !exists || !savedTimestamp.Equal(currentTimestamp) {
		// If the timestamp is different or doesn't exist, folder has been modified
		return true, nil
	}

	// No changes detected, return false
	return false, nil
}

// Function to handle folder modification checks and timestamp updates during gamelist generation
func checkAndHandleModifiedFolder(systemId, folderPath, gamelistDir string, savedTimestamps FolderTimestamps) (bool, error) {
	// Check if the system folder has been modified
	modified, err := isFolderModified(systemId, folderPath, gamelistDir, savedTimestamps)
	if err != nil {
		return false, err
	}

	// If modified, update the timestamp file after the scan
	if modified {
		// Save the current timestamp for future comparison
		if savedTimestamps == nil {
			savedTimestamps = make(FolderTimestamps)
		}
		currentTimestamp, err := getFolderTimestamp(folderPath)
		if err != nil {
			return false, err
		}
		// Update the timestamp for the specific system
		savedTimestamps[systemId] = currentTimestamp

		// Save the updated timestamps to the Modtime file
		err = saveTimestamps(gamelistDir, savedTimestamps)
		if err != nil {
			return false, err
		}
	}

	return modified, nil
}
