package config

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/ini.v1"
)

// --- Config Structs ---
type LaunchSyncConfig struct{}
type PlayLogConfig struct {
	SaveEvery   int    `ini:"save_every,omitempty"`
	OnCoreStart string `ini:"on_core_start,omitempty"`
	OnCoreStop  string `ini:"on_core_stop,omitempty"`
	OnGameStart string `ini:"on_game_start,omitempty"`
	OnGameStop  string `ini:"on_game_stop,omitempty"`
}
type RandomConfig struct{}
type LastPlayedConfig struct {
	Name                string `ini:"name,omitempty"`
	LastPlayedName      string `ini:"last_played_name,omitempty"`
	DisableLastPlayed   bool   `ini:"disable_last_played,omitempty"`
	RecentFolderName    string `ini:"recent_folder_name,omitempty"`
	DisableRecentFolder bool   `ini:"disable_recent_folder,omitempty"`
}
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

// Attract: runtime-only settings
type AttractConfig struct {
	PlayTime          string   `ini:"playtime,omitempty"`
	Random            bool     `ini:"random,omitempty"`
	Include           []string `ini:"include,omitempty" delim:","`
	Exclude           []string `ini:"exclude,omitempty" delim:","`
	UseStaticDetector bool     `ini:"usestaticdetector,omitempty"`
}

// List: list-building & filtering
type ListConfig struct {
	RamOnly           bool     `ini:"ramonly,omitempty"`
	Exclude           []string `ini:"exclude,omitempty" delim:","`

	UseBlacklist      bool     `ini:"useblacklist,omitempty"`
	BlacklistInclude  []string `ini:"blacklist_include,omitempty" delim:","`
	BlacklistExclude  []string `ini:"blacklist_exclude,omitempty" delim:","`

	UseStaticlist     bool     `ini:"usestaticlist,omitempty"`
	StaticlistInclude []string `ini:"staticlist_include,omitempty" delim:","`
	StaticlistExclude []string `ini:"staticlist_exclude,omitempty" delim:","`
	SkipafterStatic   int      `ini:"skipafterstatic,omitempty"`

	UseWhitelist      bool     `ini:"usewhitelist,omitempty"`
	WhitelistInclude  []string `ini:"whitelist_include,omitempty" delim:","`
	WhitelistExclude  []string `ini:"whitelist_exclude,omitempty" delim:","`
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
	LastPlayed     LastPlayedConfig        `ini:"lastplayed,omitempty"`
	Remote         RemoteConfig            `ini:"remote,omitempty"`
	Nfc            NfcConfig               `ini:"nfc,omitempty"`
	Systems        SystemsConfig           `ini:"systems,omitempty"`
	Attract        AttractConfig           `ini:"attract,omitempty"`
	StaticDetector StaticDetectorConfig    `ini:"staticdetector,omitempty"`
	InputDetector  InputDetectorConfig     `ini:"inputdetector,omitempty"`
	List           ListConfig              `ini:"list,omitempty"`
	Disable        map[string]DisableRules `ini:"-"`
}

// LoadUserConfig loads SAM.ini into a UserConfig
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
	defaultConfig.Disable = make(map[string]DisableRules)
	defaultConfig.StaticDetector.Systems = make(map[string]StaticDetectorOverride)
	defaultConfig.InputDetector.KeyboardMap = map[string]string{"left": "back", "right": "next", "`": "search"}
	defaultConfig.InputDetector.MouseMap = map[string]string{"swipeleft": "back", "swiperight": "next"}
	defaultConfig.InputDetector.JoystickMap = map[string]string{"dpleft": "back", "dpright": "next", "leftx-": "back", "leftx+": "next"}

	if defaultConfig.StaticDetector.BlackThreshold == 0 {
		defaultConfig.StaticDetector.BlackThreshold = 30
	}
	if defaultConfig.StaticDetector.StaticThreshold == 0 {
		defaultConfig.StaticDetector.StaticThreshold = 30
	}
	defaultConfig.StaticDetector.SkipBlack = true
	defaultConfig.StaticDetector.WriteBlackList = true
	defaultConfig.StaticDetector.SkipStatic = true
	defaultConfig.StaticDetector.WriteStaticList = true
	if defaultConfig.StaticDetector.Grace == 0 {
		defaultConfig.StaticDetector.Grace = 25
	}
	if defaultConfig.Attract.PlayTime == "" {
		defaultConfig.Attract.PlayTime = "40"
	}
	defaultConfig.Attract.Random = true
	if defaultConfig.List.SkipafterStatic == 0 {
		defaultConfig.List.SkipafterStatic = 10
	}

	// Parse INI
	cfg, err := ini.ShadowLoad(iniPath)
	if err != nil {
		return defaultConfig, err
	}

	// normalize keys case-insensitively
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

	// Per-device overrides
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

	// Parse disable.* and staticdetector.* overrides
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
