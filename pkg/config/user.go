package config

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/ini.v1"
)

type LaunchSyncConfig struct{}

type PlayLogConfig struct {
	SaveEvery   int    `ini:"save_every,omitempty"`
	OnCoreStart string `ini:"on_core_start,omitempty"`
	OnCoreStop  string `ini:"on_core_stop,omitempty"`
	OnGameStart string `ini:"on_game_start,omitempty"`
	OnGameStop  string `ini:"on_game_stop,omitempty"`
}

type RandomConfig struct{}

type SearchConfig struct {
	Filter []string `ini:"filter,omitempty" delim:","`
	Sort   string   `ini:"sort,omitempty"`
}

type RemoteConfig struct {
	MdnsService     bool   `ini:"mdns_service,omitempty"`
	SyncSSHKeys     bool   `ini:"sync_ssh_keys,omitempty"`
	CustomLogo      string `ini:"custom_logo,omitempty"`
	AnnounceGameUrl string `ini:"announce_game_url,omitempty"`
}

type NfcConfig struct {
	ConnectionString string `ini:"connection_string,omitempty"`
	AllowCommands    bool   `ini:"allow_commands,omitempty"`
	DisableSounds    bool   `ini:"disable_sounds,omitempty"`
	ProbeDevice      bool   `ini:"probe_device,omitempty"`
}

type SystemsConfig struct {
	GamesFolder []string `ini:"games_folder,omitempty,allowshadow"`
	SetCore     []string `ini:"set_core,omitempty,allowshadow"`
}

type AttractConfig struct {
	PlayTime           string   `ini:"playtime,omitempty"`
	Random             bool     `ini:"random,omitempty"`
	Include            []string `ini:"include,omitempty" delim:","`
	Exclude            []string `ini:"exclude,omitempty" delim:","`
	FreshListsEachLoad bool     `ini:"freshlistseachload,omitempty"`
	UseBlacklist       bool     `ini:"useblacklist,omitempty"`
	BlacklistInclude   []string `ini:"blacklist_include,omitempty" delim:","`
	BlacklistExclude   []string `ini:"blacklist_exclude,omitempty" delim:","`
	SkipafterStatic    int      `ini:"skipafterstatic,omitempty"`
	UseStaticDetector  bool     `ini:"usestaticdetector,omitempty"`
	UseRatedlist       bool     `ini:"useratedlist,omitempty"`
	RatedlistInclude   []string `ini:"ratedlist_include,omitempty" delim:","`
	RatedlistExclude   []string `ini:"ratedlist_exclude,omitempty" delim:","`
}

type ListConfig struct {
	Exclude            []string `ini:"exclude,omitempty" delim:","`
	UseStaticlist      bool     `ini:"usestaticlist,omitempty"`
	StaticlistInclude  []string `ini:"staticlist_include,omitempty" delim:","`
	StaticlistExclude  []string `ini:"staticlist_exclude,omitempty" delim:","`
}

type DisableRules struct {
	Folders    []string `ini:"folders,omitempty" delim:","`
	Files      []string `ini:"files,omitempty" delim:","`
	Extensions []string `ini:"extensions,omitempty" delim:","`
}

type StaticDetectorOverride struct {
	BlackThreshold  *float64 `ini:"blackthreshold,omitempty"`
	StaticThreshold *float64 `ini:"staticthreshold,omitempty"`
	SkipBlack       *bool    `ini:"skipblack,omitempty"`
	WriteBlackList  *bool    `ini:"writeblacklist,omitempty"`
	SkipStatic      *bool    `ini:"skipstatic,omitempty"`
	WriteStaticList *bool    `ini:"writestaticlist,omitempty"`
	Grace           *float64 `ini:"grace,omitempty"`
}

type StaticDetectorConfig struct {
	BlackThreshold  float64                           `ini:"blackthreshold,omitempty"`
	StaticThreshold float64                           `ini:"staticthreshold,omitempty"`
	SkipBlack       bool                              `ini:"skipblack,omitempty"`
	WriteBlackList  bool                              `ini:"writeblacklist,omitempty"`
	SkipStatic      bool                              `ini:"skipstatic,omitempty"`
	WriteStaticList bool                              `ini:"writestaticlist,omitempty"`
	Grace           float64                           `ini:"grace,omitempty"`
	Systems         map[string]StaticDetectorOverride `ini:"-"`
}

type InputDetectorConfig struct {
	Mouse       bool              `ini:"mouse,omitempty"`
	Keyboard    bool              `ini:"keyboard,omitempty"`
	Joystick    bool              `ini:"joystick,omitempty"`
	KeyboardMap map[string]string `ini:"-"`
	MouseMap    map[string]string `ini:"-"`
	JoystickMap map[string]string `ini:"-"`
}

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
	Disable        map[string]DisableRules `ini:"-"`
}

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
	// default skip/write options
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
		defaultConfig.Attract.PlayTime = "40" // default 40 seconds
	}
	// NOTE: Random is a bool (false by default). We force true here.
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

	// Map INI → struct (overrides defaults if provided)
	if err := cfg.MapTo(defaultConfig); err != nil {
		return defaultConfig, err
	}

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
