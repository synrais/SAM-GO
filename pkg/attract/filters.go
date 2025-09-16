package attract

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/utils"
)

// FilterUniqueWithMGL filters out duplicate files based on their name (ignores extension) and prioritizes `.mgl` files.
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

// FilterExtensions filters out files with specific extensions based on configuration.
func FilterExtensions(files []string, systemId string, cfg *config.UserConfig) []string {
	rules, ok := cfg.Disable[systemId]
	if !ok || len(rules.Extensions) == 0 {
		return files
	}

	extMap := make(map[string]struct{})
	for _, e := range rules.Extensions {
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
			continue
		}
		filtered = append(filtered, f)
	}

	return filtered
}

// ApplyFilterlists applies whitelist, blacklist, and staticlist filtering to the gamelist files.
func ApplyFilterlists(gamelistDir string, systemId string, files []string, cfg *config.UserConfig) []string {
	filterBase := config.FilterlistDir()

	// Whitelist (formerly ratedlist)
	if cfg.Attract.UseWhitelist {
		whitelistPath := filepath.Join(filterBase, systemId+"_whitelist.txt")
		if f, err := os.Open(whitelistPath); err == nil {
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
	if cfg.Attract.UseBlacklist {
		blacklistPath := filepath.Join(filterBase, systemId+"_blacklist.txt")
		if f, err := os.Open(blacklistPath); err == nil {
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

	// Staticlist (timestamps)
	if cfg.List.UseStaticlist {
		staticPath := filepath.Join(filterBase, systemId+"_staticlist.txt")
		if f, err := os.Open(staticPath); err == nil {
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

	return files
}
