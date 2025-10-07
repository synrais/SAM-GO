package config

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/ini.v1"
)

// --------------------------------------------------
//  Embed default.ini
// --------------------------------------------------

//go:embed default.ini
var defaultINI []byte

// --------------------------------------------------
//  Structs
// --------------------------------------------------

type AttractConfig struct {
	PlayTime          string   `ini:"playtime"`
	Random            bool     `ini:"random"`
	Include           []string `ini:"include" delim:","`
	Exclude           []string `ini:"exclude" delim:","`
	UseStaticDetector bool     `ini:"usestaticdetector"`
}

type ListConfig struct {
	UseBlacklist      bool     `ini:"useblacklist"`
	BlacklistInclude  []string `ini:"blacklistinclude" delim:","`
	BlacklistExclude  []string `ini:"blacklistexclude" delim:","`
	UseStaticlist     bool     `ini:"usestaticlist"`
	StaticlistInclude []string `ini:"staticlistinclude" delim:","`
	StaticlistExclude []string `ini:"staticlistexclude" delim:","`
	SkipAfterStatic   int      `ini:"skipafterstatic"`
	UseWhitelist      bool     `ini:"usewhitelist"`
	WhitelistInclude  []string `ini:"whitelistinclude" delim:","`
	WhitelistExclude  []string `ini:"whitelistexclude" delim:","`
}

type DisableRules struct {
	Folders    []string `ini:"folders" delim:","`
	Files      []string `ini:"files" delim:","`
	Extensions []string `ini:"extensions" delim:","`
}

type Config struct {
	Path    string
	Attract AttractConfig
	List    ListConfig
	Disable map[string]DisableRules
}

// --------------------------------------------------
//  Loader
// --------------------------------------------------

// LoadINI loads SAM.ini (next to the executable).
// If it's missing, it writes the embedded default.ini first.
func LoadINI() (*Config, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("cannot locate executable: %w", err)
	}
	baseDir := filepath.Dir(exe)
	userPath := filepath.Join(baseDir, "SAM.ini")

	// If SAM.ini missing → write embedded default.ini
	if _, err := os.Stat(userPath); os.IsNotExist(err) {
		if err := os.WriteFile(userPath, defaultINI, 0644); err != nil {
			return nil, fmt.Errorf("failed to write default ini: %w", err)
		}
		fmt.Printf("Created %s from embedded default.ini\n", userPath)
	}

	cfg := &Config{
		Path:    userPath,
		Disable: make(map[string]DisableRules),
	}

	file, err := ini.Load(userPath)
	if err != nil {
		return cfg, err
	}

	// Map main sections
	_ = file.Section("Attract").MapTo(&cfg.Attract)
	_ = file.Section("List").MapTo(&cfg.List)

	// Map Disable.* sections
	for _, sec := range file.Sections() {
		name := strings.ToLower(sec.Name())
		if strings.HasPrefix(name, "disable.") {
			sys := strings.TrimPrefix(name, "disable.")
			var rules DisableRules
			_ = sec.MapTo(&rules)
			cfg.Disable[sys] = rules
		}
	}

	return cfg, nil
}
