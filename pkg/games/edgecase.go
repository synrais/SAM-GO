package games

import (
	"os"
	"path/filepath"
	"strings"
)

// EdgeCase allows systems to define custom parsing of found files
// (e.g. turning a games.txt into individual game entries).
type EdgeCase func(filePath string) ([]string, error)

var edgeCases = map[string]EdgeCase{}

// RunEdgeCase checks if a system has a custom parser registered.
func RunEdgeCase(systemId, filePath string) ([]string, error, bool) {
	fn, ok := edgeCases[systemId]
	if !ok {
		return nil, nil, false
	}
	results, err := fn(filePath)
	return results, err, true
}

// RegisterEdgeCase registers a custom parser for a system.
func RegisterEdgeCase(systemId string, fn EdgeCase) {
	edgeCases[systemId] = fn
}

func init() {
	RegisterEdgeCase("AmigaVision", edgecaseAmigaVision)
}

// edgecaseAmigaVision only expands games.txt / demos.txt into pseudo-paths.
func edgecaseAmigaVision(txtPath string) ([]string, error) {
	base := strings.ToLower(filepath.Base(txtPath))
	if base != "games.txt" && base != "demos.txt" {
		// Not a file we care about, skip it
		return nil, nil
	}

	data, err := os.ReadFile(txtPath)
	if err != nil {
		return nil, err
	}

	var results []string
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Pseudo-path: txt file plus entry marker
		results = append(results, txtPath+"/"+line+".amiv")
	}
	return results, nil
}
