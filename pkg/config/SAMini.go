package config

// --- SAM Config Structs ---
//
// These are specific to SAM’s features (Attract Mode, StaticDetector, etc.).
// They are layered on top of the base repo’s config system but remain separate
// from the original `config.go` to avoid collisions.

// SystemsConfig defines core/system folder mappings.
type SystemsConfig struct {
	GamesFolder []string `ini:"games_folder,omitempty,allowshadow"`
	SetCore     []string `ini:"set_core,omitempty,allowshadow"`
}

// AttractConfig controls the Attract Mode feature.
type AttractConfig struct {
	PlayTime          string   `ini:"playtime,omitempty"`
	Random            bool     `ini:"random,omitempty"`
	Include           []string `ini:"include,omitempty" delim:","`
	Exclude           []string `ini:"exclude,omitempty" delim:","`
	UseStaticDetector bool     `ini:"usestaticdetector,omitempty"`
}

// ListConfig controls filtering and list-building rules.
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

// DisableRules allow filtering by folder, file, or extension.
type DisableRules struct {
	Folders    []string `ini:"folders,omitempty" delim:","`
	Files      []string `ini:"files,omitempty" delim:","`
	Extensions []string `ini:"extensions,omitempty" delim:","`
}

// StaticDetectorOverride allows per-system overrides of thresholds/flags.
type StaticDetectorOverride struct {
	BlackThreshold  *float64 `ini:"blackthreshold,omitempty"`
	StaticThreshold *float64 `ini:"staticthreshold,omitempty"`
	SkipBlack       *bool    `ini:"skipblack,omitempty"`
	WriteBlackList  *bool    `ini:"writeblacklist,omitempty"`
	SkipStatic      *bool    `ini:"skipstatic,omitempty"`
	WriteStaticList *bool    `ini:"writestaticlist,omitempty"`
	Grace           *float64 `ini:"grace,omitempty"`
}

// StaticDetectorConfig holds defaults plus per-system overrides.
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

// InputDetectorConfig maps devices/buttons to actions.
type InputDetectorConfig struct {
	Mouse       bool              `ini:"mouse,omitempty"`
	Keyboard    bool              `ini:"keyboard,omitempty"`
	Joystick    bool              `ini:"joystick,omitempty"`
	KeyboardMap map[string]string `ini:"-"`
	MouseMap    map[string]string `ini:"-"`
	JoystickMap map[string]string `ini:"-"`
}

// UserConfig is the root struct SAM uses for runtime config.
// It combines Attract Mode, StaticDetector, InputDetector, Lists, etc.
type UserConfig struct {
	AppPath        string
	IniPath        string
	Systems        SystemsConfig           `ini:"systems,omitempty"`
	Attract        AttractConfig           `ini:"attract,omitempty"`
	StaticDetector StaticDetectorConfig    `ini:"staticdetector,omitempty"`
	InputDetector  InputDetectorConfig     `ini:"inputdetector,omitempty"`
	List           ListConfig              `ini:"list,omitempty"`
	Disable        map[string]DisableRules `ini:"-"`
}

// --- Default Config Constructor ---

// NewDefaultConfig returns a fresh UserConfig seeded with safe defaults.
// These act as fallbacks if the SAM.ini is missing or contains bogus values.
// The defaults are later overlaid by LoadSAMConfig in SAMini.go.
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
