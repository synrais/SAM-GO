package attract

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/synrais/SAM-GO/pkg/cache"
	"github.com/synrais/SAM-GO/pkg/input"
	"github.com/synrais/SAM-GO/pkg/utils"
)

// gamelistFilename returns the filename for a system's gamelist.
func gamelistFilename(systemId string) string {
	return systemId + "_gamelist.txt"
}

// writeGamelist saves a gamelist both in cache and optionally on disk.
func writeGamelist(gamelistDir, systemId string, files []string, ramOnly bool) {
	cache.SetList(gamelistFilename(systemId), files)
	if ramOnly {
		return
	}
	var sb strings.Builder
	for _, file := range files {
		sb.WriteString(file)
		sb.WriteByte('\n')
	}
	data := []byte(sb.String())
	gamelistPath := filepath.Join(gamelistDir, gamelistFilename(systemId))
	_ = os.MkdirAll(filepath.Dir(gamelistPath), 0755)
	_ = os.WriteFile(gamelistPath, data, 0644)
}

// writeSimpleList writes a plain list of files to disk.
func writeSimpleList(path string, files []string) {
	var sb strings.Builder
	for _, f := range files {
		sb.WriteString(f)
		sb.WriteByte('\n')
	}
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	_ = os.WriteFile(path, []byte(sb.String()), 0644)
}

// fileExists checks if a path exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// removeSystemBlock strips all lines of a system's block from a masterlist.
func removeSystemBlock(list []string, systemId string) []string {
	var out []string
	skip := false
	for _, line := range list {
		if strings.HasPrefix(line, "# SYSTEM: ") {
			if strings.Contains(line, systemId) {
				skip = true
				continue
			} else {
				skip = false
			}
		}
		if !skip {
			out = append(out, line)
		}
	}
	return out
}

// countGames counts non-comment lines in a masterlist.
func countGames(list []string) int {
	n := 0
	for _, line := range list {
		if strings.HasPrefix(line, "# SYSTEM:") {
			continue
		}
		n++
	}
	return n
}

// updateGameIndex refreshes the global GameIndex with deduped file entries.
func updateGameIndex(systemId string, deduped []string) {
	// Drop old entries for this system
	newIndex := make([]input.GameEntry, 0, len(input.GameIndex))
	for _, e := range input.GameIndex {
		if !strings.Contains(e.Path, "/"+systemId+"/") {
			newIndex = append(newIndex, e)
		}
	}
	input.GameIndex = newIndex

	// Add fresh entries
	seenSearch := make(map[string]struct{})
	for _, f := range deduped {
		name, ext := utils.NormalizeEntry(f)
		if name == "" {
			continue
		}
		if _, ok := seenSearch[name]; ok {
			continue
		}
		input.GameIndex = append(input.GameIndex, input.GameEntry{
			Name: name,
			Ext:  ext,
			Path: f,
		})
		seenSearch[name] = struct{}{}
	}
}
