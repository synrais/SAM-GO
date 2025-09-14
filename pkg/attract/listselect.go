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

func toNameSet(lines []string) map[string]struct{} {
	set := make(map[string]struct{})
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		set[strings.ToLower(line)] = struct{}{}
	}
	return set
}

func toStaticMap(lines []string) map[string]string {
	m := make(map[string]string)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}
		ts := strings.TrimSpace(parts[0])
		name := strings.ToLower(strings.TrimSpace(parts[1]))
		m[name] = ts
	}
	return m
}

// ProcessLists applies blacklist, staticlist, and ratedlist filtering.
// (Include/Exclude is now handled in list.go, so no file deletions here.)
func ProcessLists(tmpDir, fullDir string, cfg *config.UserConfig) {
	files := cache.AllLists() // all *_gamelist.txt already loaded
	for _, fname := range files {
		if !strings.HasSuffix(fname, "_gamelist.txt") {
			continue
		}
		system := strings.TrimSuffix(fname, "_gamelist.txt")

		lines := cache.GetList(fname)
		if len(lines) == 0 {
			continue
		}

		// Rated list (whitelist)
		if cfg.Attract.UseRatedlist && allowedFor(system,
			cfg.Attract.RatedlistInclude, cfg.Attract.RatedlistExclude) {

			rated := toNameSet(cache.GetList(system + "_ratedlist.txt"))
			if rated != nil {
				var kept []string
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

			bl := toNameSet(cache.GetList(system + "_blacklist.txt"))
			if bl != nil {
				var kept []string
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

			sm := toStaticMap(cache.GetList(system + "_staticlist.txt"))
			if sm != nil {
				for i, l := range lines {
					name := normalizeName(l)
					if ts, ok := sm[name]; ok {
						lines[i] = "<" + ts + ">" + l
					}
				}
			}
		}

		// Save back into cache (and disk stays in sync when needed)
		cache.SetList(fname, lines)
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
	lines := cache.GetList(system + "_staticlist.txt")
	for _, line := range lines {
		parts := strings.SplitN(strings.TrimSpace(line), " ", 2)
		if len(parts) != 2 {
			continue
		}
		if normalizeName(parts[1]) == game {
			ts, _ := strconv.ParseFloat(parts[0], 64)
			return ts
		}
	}
	return 0
}
