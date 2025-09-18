package attract

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/synrais/SAM-GO/pkg/cache"
	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/utils"
)

func containsInsensitive(list []string, item string) bool {
	for _, v := range list {
		if strings.EqualFold(strings.TrimSpace(v), item) {
			return true
		}
	}
	return false
}

func matchesSystem(list []string, system string) bool {
	return containsInsensitive(list, system)
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
		name, _ := utils.NormalizeEntry(line)
		set[name] = struct{}{}
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
		ts := strings.Trim(parts[0], "<>")
		name, _ := utils.NormalizeEntry(parts[1])
		m[name] = ts
	}
	return m
}

// ApplyFilterlistsDetailed applies all cache-stage filters individually,
// returning the filtered lines + counts for logging.
func ApplyFilterlistsDetailed(gamelistDir, system string, lines []string, cfg *config.UserConfig) ([]string, map[string]int, bool) {
	filterBase := config.FilterlistDir()
	counts := map[string]int{
		"White":  0,
		"Black":  0,
		"Static": 0,
		"Folder": 0,
		"File":   0,
	}
	hadLists := false

	// Whitelist
	if cfg.List.UseWhitelist && allowedFor(system,
		cfg.List.WhitelistInclude, cfg.List.WhitelistExclude) {
		whitelist := readNameSet(filepath.Join(filterBase, system+"_whitelist.txt"))
		if whitelist != nil {
			hadLists = true
			var kept []string
			for _, l := range lines {
				name, _ := utils.NormalizeEntry(l)
				if _, ok := whitelist[name]; ok {
					kept = append(kept, l)
				} else {
					counts["White"]++
				}
			}
			lines = kept
		}
	}

	// Blacklist
	if cfg.List.UseBlacklist && allowedFor(system,
		cfg.List.BlacklistInclude, cfg.List.BlacklistExclude) {
		bl := readNameSet(filepath.Join(filterBase, system+"_blacklist.txt"))
		if bl != nil {
			hadLists = true
			var kept []string
			for _, l := range lines {
				name, _ := utils.NormalizeEntry(l)
				if _, bad := bl[name]; bad {
					counts["Black"]++
					continue
				}
				kept = append(kept, l)
			}
			lines = kept
		}
	}

	// Staticlist (timestamp injection only, count = matches)
	if cfg.List.UseStaticlist && allowedFor(system,
		cfg.List.StaticlistInclude, cfg.List.StaticlistExclude) {
		sm := readStaticMap(filepath.Join(filterBase, system+"_staticlist.txt"))
		if sm != nil {
			hadLists = true
			for i, l := range lines {
				name, _ := utils.NormalizeEntry(l)
				if ts, ok := sm[name]; ok {
					lines[i] = "<" + ts + ">" + l
					counts["Static"]++
				}
			}
		}
	}

	// Folder filters
	before := len(lines)
	lines = FilterFoldersAndFiles(lines, system, cfg)
	counts["Folder"] = before - len(lines)

	// File filters
	before = len(lines)
	lines = FilterUniqueWithMGL(lines)
	counts["File"] = before - len(lines)

	// Update cache
	cache.SetList(system+"_gamelist.txt", lines)

	return lines, counts, hadLists
}

// ParseLine extracts optional <timestamp> from a gamelist line.
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

func ReadStaticTimestamp(_ string, system, game string) float64 {
	filterBase := config.FilterlistDir()
	path := filepath.Join(filterBase, system+"_staticlist.txt")
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
		tsStr := strings.Trim(parts[0], "<>")
		name, _ := utils.NormalizeEntry(parts[1])
		if name == game {
			ts, _ := strconv.ParseFloat(tsStr, 64)
			return ts
		}
	}
	return 0
}
