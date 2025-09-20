package attract

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/utils"
)

// ApplyFilterlists applies whitelist, blacklist, staticlist, and folder/file filters.
// It also updates counters for debugging/logging.
func ApplyFilterlists(gamelistDir, systemID string, lines []string, cfg *config.UserConfig) ([]string, map[string]int, bool) {
	filterBase := config.FilterlistDir()
	hadLists := false
	counts := map[string]int{"White": 0, "Black": 0, "Static": 0, "Folder": 0, "File": 0}

	// Whitelist
	if cfg.List.UseWhitelist && AllowedFor(systemID, cfg.List.WhitelistInclude, cfg.List.WhitelistExclude) {
		whitelist := ReadNameSet(filepath.Join(filterBase, systemID+"_whitelist.txt"))
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
	if cfg.List.UseBlacklist && AllowedFor(systemID, cfg.List.BlacklistInclude, cfg.List.BlacklistExclude) {
		bl := ReadNameSet(filepath.Join(filterBase, systemID+"_blacklist.txt"))
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

	// Staticlist
	if cfg.List.UseStaticlist && AllowedFor(systemID, cfg.List.StaticlistInclude, cfg.List.StaticlistExclude) {
		sm := ReadStaticMap(filepath.Join(filterBase, systemID+"_staticlist.txt"))
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

	// Folders/files filters
	before := len(lines)
	lines = FilterFoldersAndFiles(lines, systemID, cfg)
	counts["Folder"] = before - len(lines)

	// Deduplication / .mgl precedence
	before = len(lines)
	lines = FilterUniqueWithMGL(lines)
	counts["File"] = before - len(lines)

	SetList(systemID+"_gamelist.txt", lines)
	return lines, counts, hadLists
}
