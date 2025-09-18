package config

import (
	"time"
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
	UseStaticDetector bool     `ini:"usestaticdetector,omitempty"`
}

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
	Remote         RemoteConfig            `ini:"remote,omitempty"`
	Nfc            NfcConfig               `ini:"nfc,omitempty"`
	Systems        SystemsConfig           `ini:"systems,omitempty"`
	Attract        AttractConfig           `ini:"attract,omitempty"`
	StaticDetector StaticDetectorConfig    `ini:"staticdetector,omitempty"`
	InputDetector  InputDetectorConfig     `ini:"inputdetector,omitempty"`
	List           ListConfig              `ini:"list,omitempty"`
	Disable        map[string]DisableRules `ini:"-"`
}

// --- Default Config Constructor ---

func NewDefaultConfig() *UserConfig {
	return &UserConfig{
		Attract: AttractConfig{
			PlayTime: "40",
			Random:   true,
		},
		StaticDetector: StaticDetectorConfig{
			BlackThreshold:  30,
			StaticThreshold: 30,
			SkipBlack:       true,
			WriteBlackList:  true,
			SkipStatic:      true,
			WriteStaticList: true,
			Grace:           25,
			Systems:         make(map[string]StaticDetectorOverride),
		},
		InputDetector: InputDetectorConfig{
			KeyboardMap: map[string]string{"left": "back", "right": "next", "`": "search"},
			MouseMap:    map[string]string{"swipeleft": "back", "swiperight": "next"},
			JoystickMap: map[string]string{"dpleft": "back", "dpright": "next", "leftx-": "back", "leftx+": "next"},
		},
		List: ListConfig{
			SkipafterStatic: 10,
		},
		Disable: make(map[string]DisableRules),
	}
}
