package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/ini.v1"
)

// UserConfig holds configuration values from the SAM.ini file
type UserConfig struct {
	AppPath        string
	IniPath        string
	LaunchSync     LaunchSyncConfig        `ini:"launchsync,omitempty"`
	PlayLog        PlayLogConfig           `ini:"playlog,omitempty"`
	Random         RandomConfig            `ini:"random,omitempty"`
	Search         SearchConfig            `ini:"search,omitempty"`
	Remote         RemoteConfig            `ini:"remote,omitempty"`
	Nfc            NfcConfig               `ini:"nfc,omitempty"`
	Systems        SystemsConfig           `ini:"systems,omitempty"`
	Attract        AttractConfig           `ini:"attract,omitempty"`
	StaticDetector StaticDetectorConfig    `ini:"staticdetector,omitempty"`
	InputDetector  InputDetectorConfig     `ini:"inputdetector,omitempty"`
	List           ListConfig              `ini:"list,omitempty"`
	Disable        map[string]DisableRules `ini:"-"` // Disable rules for systems
}

// LoadUserConfig loads SAM.ini into UserConfig
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

	// Bake in defaults BEFORE mapping from INI
	defaultConfig.AppPath = exePath
	defaultConfig.IniPath = iniPath
	defaultConfig.Disable = make(map[string]DisableRules)
	defaultConfig.StaticDetector.Systems = make(map[string]StaticDetectorOverride)
	defaultConfig.InputDetector.KeyboardMap = map[string]string{
		"left":  "back",
		"right": "next",
		"`":     "search",
	}
	defaultConfig.InputDetector.MouseMap = map[string]string{
		"swipeleft":  "back",
		"swiperight": "next",
	}
	defaultConfig.InputDetector.JoystickMap = map[string]string{
		"dpleft":  "back",
		"dpright": "next",
		"leftx-":  "back",
		"leftx+":  "next",
	}

	// ---- Default Static Detector settings ----
	if defaultConfig.StaticDetector.BlackThreshold == 0 {
		defaultConfig.StaticDetector.BlackThreshold = 30
	}
	if defaultConfig.StaticDetector.StaticThreshold == 0 {
		defaultConfig.StaticDetector.StaticThreshold = 30
	}
	if !defaultConfig.StaticDetector.SkipBlack {
		defaultConfig.StaticDetector.SkipBlack = true
	}
	if !defaultConfig.StaticDetector.WriteBlackList {
		defaultConfig.StaticDetector.WriteBlackList = true
	}
	if !defaultConfig.StaticDetector.SkipStatic {
		defaultConfig.StaticDetector.SkipStatic = true
	}
	if !defaultConfig.StaticDetector.WriteStaticList {
		defaultConfig.StaticDetector.WriteStaticList = true
	}
	if defaultConfig.StaticDetector.Grace == 0 {
		defaultConfig.StaticDetector.Grace = 25
	}

	// ---- Default Attract settings ----
	if defaultConfig.Attract.PlayTime == "" {
		defaultConfig.Attract.PlayTime = "40"
	}
	defaultConfig.Attract.Random = true
	if defaultConfig.Attract.SkipafterStatic == 0 {
		defaultConfig.Attract.SkipafterStatic = 10
	}

	// Return early if INI file doesn’t exist
	if _, err := os.Stat(iniPath); os.IsNotExist(err) {
		return defaultConfig, nil
	}

	cfg, err := ini.ShadowLoad(iniPath)
	if err != nil {
		return defaultConfig, err
	}

	// Case-insensitive normalize
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

	// Map INI → struct
	if err := cfg.MapTo(defaultConfig); err != nil {
		return defaultConfig, err
	}

	// --- FIX: Normalize Include/Exclude ---
	normalizeList := func(raw []string) []string {
		var result []string
		for _, v := range raw {
			// split in case parser didn't respect delim
			parts := strings.Split(v, ",")
			for _, p := range parts {
				trimmed := strings.TrimSpace(p)
				if trimmed != "" {
					result = append(result, trimmed)
				}
			}
		}
		return result
	}
	defaultConfig.Attract.Include = normalizeList(defaultConfig.Attract.Include)
	defaultConfig.Attract.Exclude = normalizeList(defaultConfig.Attract.Exclude)

	// Input detector overrides...
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

	// Parse disable.* and staticdetector.* rules
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
