package config

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/ini.v1"
)

// LoadSAMConfig loads SAM.ini into a SAMConfig, falling back to defaults.
func LoadSAMConfig(name string, defaults *SAMConfig) (*SAMConfig, error) {
	iniPath := os.Getenv(UserConfigEnv)

	exePath, err := os.Executable()
	if err != nil {
		return defaults, err
	}
	appPath := os.Getenv(UserAppPathEnv)
	if appPath != "" {
		exePath = appPath
	}
	if iniPath == "" {
		iniPath = filepath.Join(filepath.Dir(exePath), name+".ini")
	}

	// Seed with defaults
	defaults.AppPath = exePath
	defaults.IniPath = iniPath

	// Load INI
	cfg, err := ini.ShadowLoad(iniPath)
	if err != nil {
		return defaults, err
	}

	// Normalize keys/sections to lowercase
	for _, section := range cfg.Sections() {
		origName := section.Name()
		lowerName := strings.ToLower(origName)
		if lowerName != origName {
			dest := cfg.Section(lowerName)
			for _, key := range section.Keys() {
				dest.NewKey(strings.ToLower(key.Name()), key.Value())
			}
		}
		for _, key := range section.Keys() {
			lowerKey := strings.ToLower(key.Name())
			if lowerKey != key.Name() {
				section.NewKey(lowerKey, key.Value())
			}
		}
	}

	// Overlay INI values onto defaults
	if err := cfg.MapTo(defaults); err != nil {
		return defaults, err
	}

	// Normalize include/exclude lists
	normalizeList := func(raw []string) []string {
		var result []string
		for _, v := range raw {
			for _, p := range strings.Split(v, ",") {
				if trimmed := strings.TrimSpace(p); trimmed != "" {
					result = append(result, trimmed)
				}
			}
		}
		return result
	}
	defaults.Attract.Include = normalizeList(defaults.Attract.Include)
	defaults.Attract.Exclude = normalizeList(defaults.Attract.Exclude)
	defaults.List.Exclude = normalizeList(defaults.List.Exclude)
	defaults.List.BlacklistInclude = normalizeList(defaults.List.BlacklistInclude)
	defaults.List.BlacklistExclude = normalizeList(defaults.List.BlacklistExclude)
	defaults.List.StaticlistInclude = normalizeList(defaults.List.StaticlistInclude)
	defaults.List.StaticlistExclude = normalizeList(defaults.List.StaticlistExclude)
	defaults.List.WhitelistInclude = normalizeList(defaults.List.WhitelistInclude)
	defaults.List.WhitelistExclude = normalizeList(defaults.List.WhitelistExclude)

	// Input device overrides
	if sec, err := cfg.GetSection("inputdetector.keyboard"); err == nil {
		for _, key := range sec.Keys() {
			defaults.InputDetector.KeyboardMap[strings.ToLower(key.Name())] = key.Value()
		}
	}
	if sec, err := cfg.GetSection("inputdetector.mouse"); err == nil {
		for _, key := range sec.Keys() {
			defaults.InputDetector.MouseMap[strings.ToLower(key.Name())] = key.Value()
		}
	}
	if sec, err := cfg.GetSection("inputdetector.joystick"); err == nil {
		for _, key := range sec.Keys() {
			defaults.InputDetector.JoystickMap[strings.ToLower(key.Name())] = key.Value()
		}
	}

	// Parse disable.* and staticdetector.* overrides
	for _, section := range cfg.Sections() {
		secName := strings.ToLower(section.Name())
		switch {
		case strings.HasPrefix(secName, "disable."):
			sys := strings.TrimPrefix(secName, "disable.")
			var rules DisableRules
			_ = section.MapTo(&rules)
			defaults.Disable[sys] = rules
		case strings.HasPrefix(secName, "staticdetector."):
			sys := strings.TrimPrefix(secName, "staticdetector.")
			var sc StaticDetectorOverride
			if section.HasKey("blackthreshold") {
				v, _ := section.Key("blackthreshold").Float64()
				sc.BlackThreshold = &v
			}
			if section.HasKey("staticthreshold") {
				v, _ := section.Key("staticthreshold").Float64()
				sc.StaticThreshold = &v
			}
			if section.HasKey("skipblack") {
				v, _ := section.Key("skipblack").Bool()
				sc.SkipBlack = &v
			}
			if section.HasKey("writeblacklist") {
				v, _ := section.Key("writeblacklist").Bool()
				sc.WriteBlackList = &v
			}
			if section.HasKey("skipstatic") {
				v, _ := section.Key("skipstatic").Bool()
				sc.SkipStatic = &v
			}
			if section.HasKey("writestaticlist") {
				v, _ := section.Key("writestaticlist").Bool()
				sc.WriteStaticList = &v
			}
			if section.HasKey("grace") {
				v, _ := section.Key("grace").Float64()
				sc.Grace = &v
			}
			defaults.StaticDetector.Systems[sys] = sc
		}
	}

	return defaults, nil
}

// --- Directory helpers ---

func BaseDir() string {
	exe, err := os.Executable()
	if err != nil {
		cwd, _ := os.Getwd()
		return cwd
	}
	return filepath.Dir(exe)
}

func GamelistDir() string   { return filepath.Join(BaseDir(), "SAM_Gamelists") }
func FilterlistDir() string { return filepath.Join(GamelistDir(), "SAM_Filterlists") }
