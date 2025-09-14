package attract

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/cache"
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

func readNameSet(path string) map[string]struct{} {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	set := make(map[string]struct{})
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		set[strings.ToLower(line)] = struct{}{}
	}
	return set
}

func readStaticMap(path string) map[string]string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	m := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
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

// ProcessLists applies ratedlist, blacklist, and staticlist filtering.
// Does NOT modify disk gamelists – only updates in-memory cache.
func ProcessLists(fullDir string, cfg *config.UserConfig) {
	files, _ := filepath.Glob(filepath.Join(fullDir, "*_gamelist.txt"))
	for _, f := range files {
		system := strings.TrimSuffix(filepath.Base(f), "_gamelist.txt")

		lines, err := readLines(f)
		if err != nil {
			continue
		}

		// Rated list (whitelist)
		if cfg.Attract.UseRatedlist && allowedFor(system,
			cfg.Attract.RatedlistInclude, cfg.Attract.RatedlistExclude) {

			rated := readNameSet(filepath.Join(fullDir, system+"_ratedlist.txt"))
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

			bl := readNameSet(filepath.Join(fullDir, system+"_blacklist.txt"))
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

			sm := readStaticMap(filepath.Join(fullDir, system+"_staticlist.txt"))
			if sm != nil {
				for i, l := range lines {
					name := normalizeName(l)
					if ts, ok := sm[name]; ok {
						lines[i] = "<" + ts + ">" + l
					}
				}
			}
		}

		// Update only cache (runtime view)
		cache.SetList(filepath.Base(f), lines)
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
	path := filepath.Join(fullDir, system+"_staticlist.txt")
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
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
