package config

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/ini.v1"
)

// LoadUserConfig loads SAM.ini into a UserConfig, applying defaults.
func LoadUserConfig(name string, defaultConfig *UserConfig) (*UserConfig, error) {
	iniPath := os.Getenv(UserConfigEnv)

	exePath, err := os.Executable()
	if err != nil {
		return defaultConfig, err
	}
	appPath := os.Getenv(UserAppPathEnv)
	if appPath != "" {
		exePath = appPath
	}
	if iniPath == "" {
		iniPath = filepath.Join(filepath.Dir(exePath), name+".ini")
	}

	// Defaults
	defaultConfig.AppPath = exePath
	defaultConfig.IniPath = iniPath
	if defaultConfig.Disable == nil {
		defaultConfig.Disable = make(map[string]DisableRules)
	}
	if defaultConfig.StaticDetector.Systems == nil {
		defaultConfig.StaticDetector.Systems = make(map[string]StaticDetectorOverride)
	}
	if defaultConfig.InputDetector.KeyboardMap == nil {
		defaultConfig.InputDetector.KeyboardMap = map[string]string{"left": "back", "right": "next", "`": "search"}
	}
	if defaultConfig.InputDetector.MouseMap == nil {
		defaultConfig.InputDetector.MouseMap = map[string]string{"swipeleft": "back", "swiperight": "next"}
	}
	if defaultConfig.InputDetector.JoystickMap == nil {
		defaultConfig.InputDetector.JoystickMap = map[string]string{"dpleft": "back", "dpright": "next", "leftx-": "back", "leftx+": "next"}
	}
	if defaultConfig.StaticDetector.BlackThreshold == 0 {
		defaultConfig.StaticDetector.BlackThreshold = 30
	}
	if defaultConfig.StaticDetector.StaticThreshold == 0 {
		defaultConfig.StaticDetector.StaticThreshold = 30
	}
	if defaultConfig.StaticDetector.Grace == 0 {
		defaultConfig.StaticDetector.Grace = 25
	}
	if defaultConfig.Attract.PlayTime == "" {
		defaultConfig.Attract.PlayTime = "40"
	}
	if defaultConfig.List.SkipafterStatic == 0 {
		defaultConfig.List.SkipafterStatic = 10
	}
	defaultConfig.Attract.Random = true
	defaultConfig.StaticDetector.SkipBlack = true
	defaultConfig.StaticDetector.WriteBlackList = true
	defaultConfig.StaticDetector.SkipStatic = true
	defaultConfig.StaticDetector.WriteStaticList = true

	// Parse INI
	cfg, err := ini.ShadowLoad(iniPath)
	if err != nil {
		return defaultConfig, err
	}

	// Normalize case-insensitive keys
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

	if err := cfg.MapTo(defaultConfig); err != nil {
		return defaultConfig, err
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
	defaultConfig.Attract.Include = normalizeList(defaultConfig.Attract.Include)
	defaultConfig.Attract.Exclude = normalizeList(defaultConfig.Attract.Exclude)
	defaultConfig.List.Exclude = normalizeList(defaultConfig.List.Exclude)
	defaultConfig.List.BlacklistInclude = normalizeList(defaultConfig.List.BlacklistInclude)
	defaultConfig.List.BlacklistExclude = normalizeList(defaultConfig.List.BlacklistExclude)
	defaultConfig.List.StaticlistInclude = normalizeList(defaultConfig.List.StaticlistInclude)
	defaultConfig.List.StaticlistExclude = normalizeList(defaultConfig.List.StaticlistExclude)
	defaultConfig.List.WhitelistInclude = normalizeList(defaultConfig.List.WhitelistInclude)
	defaultConfig.List.WhitelistExclude = normalizeList(defaultConfig.List.WhitelistExclude)

	// Device-specific overrides
	if sec, err := cfg.GetSection("inputdetector.keyboard"); err == nil {
		for _, key := range sec.Keys() {
			defaultConfig.InputDetector.KeyboardMap[strings.ToLower(key.Name())] = key.Value()
		}
	}
	if sec, err := cfg.GetSection("inputdetector.mouse"); err == nil {
		for _, key := range sec.Keys() {
			defaultConfig.InputDetector.MouseMap[strings.ToLower(key.Name())] = key.Value()
		}
	}
	if sec, err := cfg.GetSection("inputdetector.joystick"); err == nil {
		for _, key := range sec.Keys() {
			defaultConfig.InputDetector.JoystickMap[strings.ToLower(key.Name())] = key.Value()
		}
	}

	// Disable.* and staticdetector.* overrides
	for _, section := range cfg.Sections() {
		secName := strings.ToLower(section.Name())
		switch {
		case strings.HasPrefix(secName, "disable."):
			sys := strings.TrimPrefix(secName, "disable.")
			var rules DisableRules
			_ = section.MapTo(&rules)
			defaultConfig.Disable[sys] = rules
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
			defaultConfig.StaticDetector.Systems[sys] = sc
		}
	}

	return defaultConfig, nil
}
