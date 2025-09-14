package attract

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/synrais/SAM-GO/pkg/cache"
	"github.com/synrais/SAM-GO/pkg/config"
)

// normalizeName converts a file path or name to lowercase base name without extension.
func normalizeName(p string) string {
	base := filepath.Base(p)
	ext := filepath.Ext(base)
	return strings.ToLower(strings.TrimSuffix(base, ext))
}

func containsInsensitive(list []string, item string) bool {
	for _, v := range list {
		if strings.EqualFold(strings.TrimSpace(v), item) {
			return true
		}
	}
	return false
}

// matchesSystem checks if a system name appears in a list, accounting for
// AmigaVision aliases.
func matchesSystem(list []string, system string) bool {
	if containsInsensitive(list, system) {
		return true
	}
	if strings.HasPrefix(strings.ToLower(system), "amigavision") {
		return containsInsensitive(list, "AmigaVision")
	}
	return false
}

func allowedFor(system string, include, exclude []string) bool {
	if len(include) > 0 && !matchesSystem(include, system) {
		return false
	}
	if matchesSystem(exclude, system) {
		return false
	}
	return true
}

// --- Cached helpers ---

func readNameSetCached(filename string) map[string]struct{} {
	lines := cache.GetList(filename)
	if lines == nil {
		return nil
	}
	set := make(map[string]struct{}, len(lines))
	for _, l := range lines {
		l = strings.ToLower(strings.TrimSpace(l))
		if l != "" {
			set[l] = struct{}{}
		}
	}
	return set
}

func readStaticMapCached(filename string) map[string]string {
	lines := cache.GetList(filename)
	if lines == nil {
		return nil
	}
	m := make(map[string]string, len(lines))
	for _, line := range lines {
		parts := strings.SplitN(strings.TrimSpace(line), " ", 2)
		if len(parts) == 2 {
			ts := strings.TrimSpace(parts[0])
			name := strings.ToLower(strings.TrimSpace(parts[1]))
			m[name] = ts
		}
	}
	return m
}

// ProcessLists applies blacklist, staticlist, and ratedlist filtering.
// (Include/Exclude is now handled in list.go, so no file deletions here.)
func ProcessLists(tmpDir, fullDir string, cfg *config.UserConfig) {
	files, _ := filepath.Glob(filepath.Join(tmpDir, "*_gamelist.txt"))
	for _, f := range files {
		system := strings.TrimSuffix(filepath.Base(f), "_gamelist.txt")

		lines, err := readLines(f)
		if err != nil {
			continue
		}

		// Rated list (whitelist)
		if cfg.Attract.UseRatedlist && allowedFor(system,
			cfg.Attract.RatedlistInclude, cfg.Attract.RatedlistExclude) {

			rated := readNameSetCached(system + "_ratedlist.txt")
			if rated != nil {
				kept := make([]string, 0, len(lines))
				for _, l := range lines {
					if _, ok := rated[normalizeName(l)]; ok {
						kept = append(kept, l)
					}
				}
				lines = kept
			}
		}

		// Blacklist
		if cfg.Attract.UseBlacklist && allowedFor(system,
			cfg.Attract.BlacklistInclude, cfg.Attract.BlacklistExclude) {

			bl := readNameSetCached(system + "_blacklist.txt")
			if bl != nil {
				kept := make([]string, 0, len(lines))
				for _, l := range lines {
					if _, ok := bl[normalizeName(l)]; !ok {
						kept = append(kept, l)
					}
				}
				lines = kept
			}
		}

		// Static list timestamps
		if cfg.Attract.UseStaticlist && allowedFor(system,
			cfg.Attract.StaticlistInclude, cfg.Attract.StaticlistExclude) {

			sm := readStaticMapCached(system + "_staticlist.txt")
			if sm != nil {
				for i, l := range lines {
					if ts, ok := sm[normalizeName(l)]; ok {
						lines[i] = "<" + ts + ">" + l
					}
				}
			}
		}

		_ = writeLines(f, lines)
	}
}

// ParseLine separates an optional <timestamp> prefix from a gamelist line.
func ParseLine(line string) (float64, string) {
	if strings.HasPrefix(line, "<") {
		if idx := strings.Index(line, ">"); idx > 1 {
			tsStr := line[1:idx]
			rest := line[idx+1:]
			ts, _ := strconv.ParseFloat(tsStr, 64)
			return ts, rest
		}
	}
	return 0, line
}

// ReadStaticTimestamp returns the timestamp for a game from the static list.
func ReadStaticTimestamp(fullDir, system, game string) float64 {
	sm := readStaticMapCached(system + "_staticlist.txt")
	if sm == nil {
		return 0
	}
	if ts, ok := sm[game]; ok {
		val, _ := strconv.ParseFloat(ts, 64)
		return val
	}
	return 0
}
