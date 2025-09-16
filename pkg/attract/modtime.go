package attract

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
	"io/ioutil"
)

// Function to get the last modified timestamp of a system folder
func getFolderTimestamp(folderPath string) (time.Time, error) {
    // Debugging: Log the folder path being checked
    fmt.Printf("Checking folder: %s\n", folderPath)

    if _, err := os.Stat(folderPath); os.IsNotExist(err) {
        // Provide detailed debug output if folder doesn't exist
        fmt.Printf("Debug: Folder %s does not exist. Skipping...\n", folderPath)
        return time.Time{}, fmt.Errorf("folder %s does not exist: %w", folderPath, err)
    }

    // Get the information about the folder
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
    // Debugging: Log the folder being checked
    fmt.Printf("Checking if folder '%s' has been modified...\n", folderPath)

    // Check if the system folder has been modified
    modified, err := isFolderModified(systemId, folderPath, gamelistDir, savedTimestamps)
    if err != nil {
        // Debugging: Output error with additional info
        fmt.Printf("Error: %s\n", err)
        return false, err
    }

    // Handle folder modification and timestamp updates if necessary
    if modified {
        fmt.Printf("Folder '%s' was modified. Updating timestamp...\n", folderPath)

        if savedTimestamps == nil {
            savedTimestamps = make(FolderTimestamps)
        }
        currentTimestamp, err := getFolderTimestamp(folderPath)
        if err != nil {
            return false, err
        }

        // Save the new timestamp for future comparisons
        savedTimestamps[systemId] = currentTimestamp

        // Save updated timestamps to Modtime file
        err = saveTimestamps(gamelistDir, savedTimestamps)
        if err != nil {
            return false, err
        }

        // Debugging: Output confirmation of the update
        fmt.Printf("Timestamp for folder '%s' updated to: %s\n", folderPath, currentTimestamp.Format(time.RFC3339))
    } else {
        // No modification detected, log the current status
        fmt.Printf("No modification detected for folder '%s'. Timestamp remains the same.\n", folderPath)
    }

    return modified, nil
}
