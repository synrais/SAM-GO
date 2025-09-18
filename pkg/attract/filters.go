package attract

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/utils"
)

// -----------------------------
// Matching helper
// -----------------------------

// matchRule applies human-friendly pattern matching to a candidate string.
func matchRule(rule, candidate string) bool {
	rule = strings.ToLower(strings.TrimSpace(rule))
	candidate = strings.ToLower(strings.TrimSpace(candidate))

	if rule == "" || candidate == "" {
		return false
	}

	// Wildcard contains check
	if strings.HasPrefix(rule, "*") && strings.HasSuffix(rule, "*") && len(rule) > 2 {
		sub := strings.Trim(rule, "*")
		return strings.Contains(candidate, sub)
	}
	// Prefix match
	if strings.HasSuffix(rule, "*") {
		prefix := strings.TrimSuffix(rule, "*")
		return strings.HasPrefix(candidate, prefix)
	}
	// Suffix match
	if strings.HasPrefix(rule, "*") {
		suffix := strings.TrimPrefix(rule, "*")
		return strings.HasSuffix(candidate, suffix)
	}
	// Exact match (ignore extension for candidate if rule lacks one)
	if !strings.Contains(rule, ".") {
		candidate = strings.TrimSuffix(candidate, filepath.Ext(candidate))
	}
	return candidate == rule
}

// -----------------------------
// Filters
// -----------------------------

// FilterUniqueWithMGL filters out duplicate files based on their base name
// (ignores extension) and prioritizes `.mgl` files.
func FilterUniqueWithMGL(files []string) []string {
	chosen := make(map[string]string)
	for _, f := range files {
		base := strings.TrimSuffix(strings.ToLower(filepath.Base(f)), filepath.Ext(f))
		ext := strings.ToLower(filepath.Ext(f))
		if prev, ok := chosen[base]; ok {
			if strings.HasSuffix(prev, ".mgl") {
				continue
			}
			if ext == ".mgl" {
				chosen[base] = f
			}
		} else {
			chosen[base] = f
		}
	}
	result := []string{}
	for _, v := range chosen {
		result = append(result, v)
	}
	return result
}

// FilterFoldersAndFiles applies Disable.Folders and Disable.Files rules (per-system + global).
func FilterFoldersAndFiles(files []string, systemID string, cfg *config.UserConfig) []string {
	var folders, patterns []string

	// Global rules [Disable.ALL]
	if global, ok := cfg.Disable["all"]; ok {
		folders = append(folders, global.Folders...)
		patterns = append(patterns, global.Files...)
	}
	// System-specific rules
	if sys, ok := cfg.Disable[strings.ToLower(systemID)]; ok {
		folders = append(folders, sys.Folders...)
		patterns = append(patterns, sys.Files...)
	}

	if len(folders) == 0 && len(patterns) == 0 {
		return files
	}

	var filtered []string
	for _, f := range files {
		base := filepath.Base(f)
		dir := filepath.Dir(f)
		skip := false

		// --- Folder filter (check each segment) ---
		dirParts := strings.Split(dir, string(os.PathSeparator))
		for _, folderRule := range folders {
			for _, seg := range dirParts {
				if matchRule(folderRule, seg) {
					fmt.Printf("[Filters] Skipping %s (folder %s disabled)\n", base, folderRule)
					skip = true
					break
				}
			}
			if skip {
				break
			}
		}
		if skip {
			continue
		}

		// --- File pattern filter ---
		for _, fileRule := range patterns {
			if matchRule(fileRule, base) {
				fmt.Printf("[Filters] Skipping %s (pattern %s disabled)\n", base, fileRule)
				skip = true
				break
			}
		}

		if !skip {
			filtered = append(filtered, f)
		}
	}
	return filtered
}

// FilterExtensions removes files with specific extensions (per-system + global).
func FilterExtensions(files []string, systemID string, cfg *config.UserConfig) []string {
	var rules []string

	// Always normalize systemID to lowercase for lookup
	sysKey := strings.ToLower(systemID)

	// Global rules [Disable.ALL]
	if global, ok := cfg.Disable["all"]; ok {
		rules = append(rules, global.Extensions...)
	}
	// System-specific rules
	if sys, ok := cfg.Disable[sysKey]; ok {
		rules = append(rules, sys.Extensions...)
	}

	if len(rules) == 0 {
		return files
	}

	extMap := make(map[string]struct{})
	for _, e := range rules {
		e = strings.ToLower(e)
		if !strings.HasPrefix(e, ".") {
			e = "." + e
		}
		extMap[e] = struct{}{}
	}

	var filtered []string
	for _, f := range files {
		ext := strings.ToLower(filepath.Ext(f))
		if _, skip := extMap[ext]; skip {
			fmt.Printf("[Filters] Skipping %s (extension %s disabled)\n", filepath.Base(f), ext)
			continue
		}
		filtered = append(filtered, f)
	}
	return filtered
}

// ApplyFilterlists applies whitelist, blacklist, and staticlist filtering to gamelist files.
func ApplyFilterlists(gamelistDir string, systemID string, files []string, cfg *config.UserConfig) ([]string, bool) {
	filterBase := config.FilterlistDir()
	hadLists := false

	// Whitelist
	if cfg.List.UseWhitelist {
		whitelistPath := filepath.Join(filterBase, systemID+"_whitelist.txt")
		if f, err := os.Open(whitelistPath); err == nil {
			hadLists = true
			defer f.Close()
			whitelist := make(map[string]struct{})
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				name, _ := utils.NormalizeEntry(scanner.Text())
				if name != "" {
					whitelist[name] = struct{}{}
				}
			}
			var kept []string
			for _, file := range files {
				name, _ := utils.NormalizeEntry(filepath.Base(file))
				if _, ok := whitelist[name]; ok {
					kept = append(kept, file)
				}
			}
			files = kept
		}
	}

	// Blacklist
	if cfg.List.UseBlacklist {
		blacklistPath := filepath.Join(filterBase, systemID+"_blacklist.txt")
		if f, err := os.Open(blacklistPath); err == nil {
			hadLists = true
			defer f.Close()
			blacklist := make(map[string]struct{})
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				name, _ := utils.NormalizeEntry(scanner.Text())
				if name != "" {
					blacklist[name] = struct{}{}
				}
			}
			var kept []string
			for _, file := range files {
				name, _ := utils.NormalizeEntry(filepath.Base(file))
				if _, bad := blacklist[name]; !bad {
					kept = append(kept, file)
				}
			}
			files = kept
		}
	}

	// Staticlist
	if cfg.List.UseStaticlist {
		staticPath := filepath.Join(filterBase, systemID+"_staticlist.txt")
		if f, err := os.Open(staticPath); err == nil {
			hadLists = true
			defer f.Close()
			staticMap := make(map[string]string)
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
				staticMap[name] = ts
			}
			for i, f := range files {
				name, _ := utils.NormalizeEntry(filepath.Base(f))
				if ts, ok := staticMap[name]; ok {
					files[i] = "<" + ts + ">" + f
				}
			}
		}
	}

	return files, hadLists
}
