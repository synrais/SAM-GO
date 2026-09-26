package games

import (
	"os"
	"path/filepath"
	"strings"
)

// --------------------------------------------------
// Registry
// --------------------------------------------------

// EdgeCase parses a scanned file into game entries (none to skip it).
type EdgeCase func(filePath string) ([]string, error)

var edgeCases = map[string]EdgeCase{}

func RegisterEdgeCase(systemId string, fn EdgeCase) {
	systemId = strings.ToLower(systemId)
	edgeCases[systemId] = fn
}

func init() {
	RegisterEdgeCase("AmigaVision", edgecaseAmigaVision)
}

// RunEdgeCase checks if systemId has an edge case
func RunEdgeCase(systemId, filePath string) ([]string, error, bool) {
	fn, ok := edgeCases[strings.ToLower(systemId)]
	if !ok {
		return nil, nil, false
	}
	results, err := fn(filePath)
	return results, err, true
}

// --------------------------------------------------
// AmigaVision
// --------------------------------------------------

// edgecaseAmigaVision expands games.txt / demos.txt into one .ags entry per line.
func edgecaseAmigaVision(txtPath string) ([]string, error) {
	base := strings.ToLower(filepath.Base(txtPath))
	if base != "games.txt" && base != "demos.txt" {
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
		results = append(results, txtPath+"/"+line+".ags")
	}
	return results, nil
}
