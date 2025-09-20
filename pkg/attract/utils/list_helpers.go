package attract

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/synrais/SAM-GO/pkg/utils"
)

// fileExists checks if path exists and is not a dir.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// gamelistFilename returns standard system gamelist filename.
func gamelistFilename(systemID string) string {
	return systemID + "_gamelist.txt"
}

// writeGamelist saves a gamelist (unless RAM-only).
func writeGamelist(dir, systemID string, files []string, ramOnly bool) {
	if ramOnly {
		return
	}
	path := filepath.Join(dir, gamelistFilename(systemID))
	_ = os.WriteFile(path, []byte(strings.Join(files, "\n")), 0644)
}

// writeSimpleList saves a text file list.
func writeSimpleList(path string, lines []string) {
	_ = os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
}

// removeSystemBlock strips existing # SYSTEM: block for a system.
func removeSystemBlock(master []string, systemID string) []string {
	var out []string
	skip := false
	for _, line := range master {
		if strings.HasPrefix(line, "# SYSTEM: ") {
			if strings.Contains(line, systemID) {
				skip = true
				continue
			}
			skip = false
		}
		if !skip {
			out = append(out, line)
		}
	}
	return out
}

// countGames counts all non-#SYSTEM lines in masterlist.
func countGames(master []string) int {
	count := 0
	for _, line := range master {
		if !strings.HasPrefix(line, "# SYSTEM:") {
			count++
		}
	}
	return count
}

// updateGameIndex builds GameEntry objects and pushes to cache.
func updateGameIndex(systemID string, files []string) {
	unique := utils.DedupeFiles(files)
	for _, f := range unique {
		name, ext := utils.NormalizeEntry(f)
		entry := GameEntry{
			SystemID: systemID,
			Name:     name,
			Ext:      ext,
			Path:     f,
		}
		AppendGameIndex(entry)
	}
}
