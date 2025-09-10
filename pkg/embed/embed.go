package embed

import (
	_ "embed" // Required to embed files into Go source
	"strings"
)

//go:embed gamecontrollerdb.txt
var gamecontrollerdb string

// Add more embedded files as needed
//go:embed otherfile.txt
var otherFile string

// Function to load the gamecontrollerdb as a slice of strings
func LoadGameControllerDB() ([]string, error) {
	lines := strings.Split(gamecontrollerdb, "\n")
	var controllers []string
	for _, line := range lines {
		if len(line) > 0 {
			controllers = append(controllers, line)
		}
	}
	return controllers, nil
}

// Another function to load other files, if necessary
func LoadOtherFile() string {
	return otherFile
}
