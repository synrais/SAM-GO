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
	PlayTime          string   `ini:"playtime,omitempty"`
	Random            bool     `ini:"random,omitempty"`
	Include           []string `ini:"include,omitempty" delim:","`
	Exclude           []string `ini:"exclude,omitempty" delim:","`
	UseBlacklist      bool     `ini:"useblacklist,omitempty"`
	BlacklistInclude  []string `ini:"blacklist_include,omitempty" delim:","`
	BlacklistExclude  []string `ini:"blacklist_exclude,omitempty" delim:","`
	UseStaticlist     bool     `ini:"usestaticlist,omitempty"`
	StaticlistInclude []string `ini:"staticlist_include,omitempty" delim:","`
	StaticlistExclude []string `ini:"staticlist_exclude,omitempty" delim:","`
	SkipafterStatic   int      `ini:"skipafterstatic,omitempty"`
	UseRatedlist      bool     `ini:"useratedlist,omitempty"`
	RatedlistInclude  []string `ini:"ratedlist_include,omitempty" delim:","`
	RatedlistExclude  []string `ini:"ratedlist_exclude,omitempty" delim:","`
}

type ListConfig struct {
	Exclude []string `ini:"exclude,omitempty" delim:","`
}

type DisableRules struct {
	Folders    []string `ini:"folders,omitempty" delim:","`
	Files      []string `ini:"files,omitempty" delim:","`
	Extensions []string `ini:"extensions,omitempty" delim:","`
}

type UserConfig struct {
	AppPath    string
	IniPath    string
	LaunchSync LaunchSyncConfig        `ini:"launchsync,omitempty"`
	PlayLog    PlayLogConfig           `ini:"playlog,omitempty"`
	Random     RandomConfig            `ini:"random,omitempty"`
	Search     SearchConfig            `ini:"search,omitempty"`
	Remote  RemoteConfig            `ini:"remote,omitempty"`
	Nfc     NfcConfig               `ini:"nfc,omitempty"`
	Systems SystemsConfig           `ini:"systems,omitempty"`
	Attract AttractConfig           `ini:"attract,omitempty"`
	List    ListConfig              `ini:"list,omitempty"`
	Disable map[string]DisableRules `ini:"-"`
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

	// Parse disable.* rules
	for _, section := range cfg.Sections() {
		secName := strings.ToLower(section.Name())
		if strings.HasPrefix(secName, "disable.") {
			sys := strings.TrimPrefix(secName, "disable.")
			var rules DisableRules
			_ = section.MapTo(&rules)
			defaultConfig.Disable[sys] = rules
		}
	}

	return defaultConfig, nil
}
