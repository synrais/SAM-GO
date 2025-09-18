package attract

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/synrais/SAM-GO/pkg/cache"
)

// UpdateGameIndex refreshes the global search index for a system.
func UpdateGameIndex(systemID string, deduped []string) {
    newIndex := make([]input.GameEntry, 0, len(input.GameIndex))
    for _, e := range input.GameIndex {
        if !strings.Contains(e.Path, "/"+systemID+"/") {
            newIndex = append(newIndex, e)
        }
    }
    input.GameIndex = newIndex

    seen := make(map[string]struct{})
    for _, f := range deduped {
        name, ext := utils.NormalizeEntry(f)
        if name == "" {
            continue
        }
        if _, ok := seen[name]; ok {
            continue
        }
        input.GameIndex = append(input.GameIndex, input.GameEntry{
            Name: name,
            Ext:  ext,
            Path: f,
        })
        seen[name] = struct{}{}
    }
}

// DedupeFiles removes duplicate entries based on normalized names.
func DedupeFiles(files []string) []string {
    seen := make(map[string]struct{})
    deduped := make([]string, 0, len(files))
    for _, f := range files {
        name, _ := utils.NormalizeEntry(f)
        if _, ok := seen[name]; ok {
            continue
        }
        seen[name] = struct{}{}
        deduped = append(deduped, f)
    }
    return deduped
}

// gamelistFilename returns the standard filename for a system's gamelist.
func gamelistFilename(systemId string) string {
	return systemId + "_gamelist.txt"
}

// writeGamelist saves a gamelist both to cache and (unless ramOnly) to disk.
func writeGamelist(gamelistDir string, systemId string, files []string, ramOnly bool) {
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
	if err := os.MkdirAll(filepath.Dir(gamelistPath), 0755); err != nil {
		panic(err)
	}
	if err := os.WriteFile(gamelistPath, data, 0644); err != nil {
		panic(err)
	}
}

// writeSimpleList writes a plain newline-delimited list to disk.
func writeSimpleList(path string, files []string) {
	var sb strings.Builder
	for _, f := range files {
		sb.WriteString(f)
		sb.WriteByte('\n')
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		panic(err)
	}
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		panic(err)
	}
}

// fileExists checks if a path exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// removeSystemBlock removes a system block from a master list.
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

// countGames counts non-system lines (i.e., actual games).
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
