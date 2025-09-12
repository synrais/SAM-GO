package attract

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/synrais/SAM-GO/pkg/config"
)

const listSourceDir = "/media/fat/Scripts/.MiSTer_SAM/SAM_Gamelists"

func normalizeName(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(strings.ToLower(base), strings.ToLower(ext))
}

func containsSystem(list []string, system string) bool {
	for _, s := range list {
		if strings.EqualFold(strings.TrimSpace(s), system) {
			return true
		}
	}
	return false
}

func systemAllowed(include, exclude []string, system string) bool {
	if len(include) > 0 && !containsSystem(include, system) {
		return false
	}
	if containsSystem(exclude, system) {
		return false
	}
	return true
}

func loadNameSet(path string) map[string]bool {
	lines, err := readLines(path)
	if err != nil {
		return nil
	}
	set := make(map[string]bool)
	for _, l := range lines {
		set[normalizeName(l)] = true
	}
	return set
}

func loadStaticMap(path string) map[string]string {
	lines, err := readLines(path)
	if err != nil {
		return nil
	}
	m := make(map[string]string)
	for _, l := range lines {
		parts := strings.SplitN(l, " ", 2)
		if len(parts) < 2 {
			continue
		}
		ts := strings.TrimSpace(parts[0])
		name := strings.TrimSpace(parts[1])
		m[normalizeName(name)] = ts
	}
	return m
}

// ProcessLists filters gamelists in tmpDir according to config.
func ProcessLists(tmpDir string, cfg *config.UserConfig) {
	ls := cfg.ListSelect
	files, _ := filepath.Glob(filepath.Join(tmpDir, "*_gamelist.txt"))

	for _, file := range files {
		system := strings.TrimSuffix(filepath.Base(file), "_gamelist.txt")

		if len(ls.Include) > 0 && !containsSystem(ls.Include, system) {
			_ = os.Remove(file)
			continue
		}
		if containsSystem(ls.Exclude, system) {
			_ = os.Remove(file)
			continue
		}

		lines, err := readLines(file)
		if err != nil {
			continue
		}

		if ls.UseBlacklist && systemAllowed(ls.BlacklistInclude, ls.BlacklistExclude, system) {
			bl := loadNameSet(filepath.Join(listSourceDir, system+"_blacklist.txt"))
			if len(bl) > 0 {
				var out []string
				for _, line := range lines {
					if !bl[normalizeName(line)] {
						out = append(out, line)
					}
				}
				lines = out
			}
		}

		if ls.UseRatedlist && systemAllowed(ls.RatedlistInclude, ls.RatedlistExclude, system) {
			rl := loadNameSet(filepath.Join(listSourceDir, system+"_ratedlist.txt"))
			if len(rl) > 0 {
				var out []string
				for _, line := range lines {
					if rl[normalizeName(line)] {
						out = append(out, line)
					}
				}
				lines = out
			}
		}

		if ls.UseStaticlist && systemAllowed(ls.StaticlistInclude, ls.StaticlistExclude, system) {
			sm := loadStaticMap(filepath.Join(listSourceDir, system+"_staticlist.txt"))
			if len(sm) > 0 {
				for i, line := range lines {
					if ts, ok := sm[normalizeName(line)]; ok {
						lines[i] = fmt.Sprintf("<%s>%s", ts, line)
					}
				}
			}
		}

		_ = writeLines(file, lines)
	}
}

// ParseTimestamp extracts timestamp and clean path from a list entry.
func ParseTimestamp(line string) (string, int64) {
	if strings.HasPrefix(line, "<") {
		if idx := strings.Index(line, ">"); idx > 1 {
			ts := line[1:idx]
			rest := strings.TrimSpace(line[idx+1:])
			if t, err := strconv.ParseInt(ts, 10, 64); err == nil {
				return rest, t
			}
			return rest, 0
		}
	}
	return line, 0
}
