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

// ProcessLists applies whitelist, blacklist, and staticlist filtering
// to lists already in cache (disk-prepared lists). Returns filtered lines
// and counts per filter type.
func ProcessLists(system string, cfg *config.UserConfig) ([]string, map[string]int) {
	lines := cache.GetList(gamelistFilename(system))
	if len(lines) == 0 {
		return nil, nil
	}

	filterBase := config.FilterlistDir()
	counts := map[string]int{"white": 0, "black": 0, "static": 0}

	// Whitelist
	if cfg.List.UseWhitelist && allowedFor(system,
		cfg.List.WhitelistInclude, cfg.List.WhitelistExclude) {

		whitelist := readNameSet(filepath.Join(filterBase, system+"_whitelist.txt"))
		if whitelist != nil {
			var kept []string
			for _, l := range lines {
				name, _ := utils.NormalizeEntry(l)
				if _, ok := whitelist[name]; ok {
					kept = append(kept, l)
				} else {
					counts["white"]++
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
			var kept []string
			for _, l := range lines {
				name, _ := utils.NormalizeEntry(l)
				if _, bad := bl[name]; bad {
					counts["black"]++
					continue
				}
				kept = append(kept, l)
			}
			lines = kept
		}
	}

	// Static list timestamps
	if cfg.List.UseStaticlist && allowedFor(system,
		cfg.List.StaticlistInclude, cfg.List.StaticlistExclude) {

		sm := readStaticMap(filepath.Join(filterBase, system+"_staticlist.txt"))
		if sm != nil {
			for i, l := range lines {
				name, _ := utils.NormalizeEntry(l)
				if ts, ok := sm[name]; ok {
					lines[i] = "<" + ts + ">" + l
					counts["static"]++
				}
			}
		}
	}

	// Update cache
	cache.SetList(gamelistFilename(system), lines)
	return lines, counts
}

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
