package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/synrais/SAM-GO/pkg/assets"
	"gopkg.in/ini.v1"
)

// ---- Config Structs ----

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
	SkipafterStatic   int      `ini:"skipafterstatic,omitempty"`
	UseStaticDetector bool     `ini:"usestaticdetector,omitempty"`
	UseWhitelist      bool     `ini:"usewhitelist,omitempty"`
	WhitelistInclude  []string `ini:"whitelistinclude,omitempty" delim:","`
	WhitelistExclude  []string `ini:"whitelistexclude,omitempty" delim:","`
}

type ListConfig struct {
	Exclude           []string `ini:"exclude,omitempty" delim:","`
	UseStaticlist     bool     `ini:"usestaticlist,omitempty"`
	StaticlistInclude []string `ini:"staticlist_include,omitempty" delim:","`
	StaticlistExclude []string `ini:"staticlist_exclude,omitempty" delim:","`
	RamOnly           bool     `ini:"ramonly,omitempty"`
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

// ---- Ensure SAM.ini exists, load & debug ----

func EnsureUserConfig(name string) (*UserConfig, error) {
	exePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get exe path: %w", err)
	}
	iniPath := filepath.Join(filepath.Dir(exePath), name+".ini")

	// Ensure ini exists
	if _, err := os.Stat(iniPath); os.IsNotExist(err) {
		if err := os.WriteFile(iniPath, []byte(assets.DefaultSAMIni), 0644); err != nil {
			return nil, fmt.Errorf("failed to write default ini: %w", err)
		}
		fmt.Println("[CONFIG] No ini found. Copying default")
	} else if err == nil {
		fmt.Println("[CONFIG] Found SAM.ini")
	} else {
		return nil, fmt.Errorf("failed to check ini: %w", err)
	}

	// Load config
	cfg, err := LoadUserConfig(name, &UserConfig{})
	if err != nil {
		return nil, err
	}

	// ---- Debug summary ----
	fmt.Printf("[CONFIG] Loaded config from: %s\n", cfg.IniPath)
	fmt.Println("[CONFIG] Ini settings:")
	fmt.Printf("  Attract:\n")
	fmt.Printf("    PlayTime=%s | Random=%v\n", cfg.Attract.PlayTime, cfg.Attract.Random)
	fmt.Printf("    Include=%v | Exclude=%v\n", cfg.Attract.Include, cfg.Attract.Exclude)
	fmt.Printf("    UseBlacklist=%v | UseWhitelist=%v | UseStaticlist=%v | UseStaticDetector=%v\n",
		cfg.Attract.UseBlacklist, cfg.Attract.UseWhitelist, cfg.List.UseStaticlist, cfg.Attract.UseStaticDetector)
	fmt.Printf("  List: Exclude=%v | RamOnly=%v\n", cfg.List.Exclude, cfg.List.RamOnly)
	fmt.Printf("  InputDetector: Mouse=%v | Keyboard=%v | Joystick=%v\n",
		cfg.InputDetector.Mouse, cfg.InputDetector.Keyboard, cfg.InputDetector.Joystick)
	fmt.Printf("    KeyboardMap=%v\n", cfg.InputDetector.KeyboardMap)
	fmt.Printf("    MouseMap=%v\n", cfg.InputDetector.MouseMap)
	fmt.Printf("    JoystickMap=%v\n", cfg.InputDetector.JoystickMap)
	fmt.Printf("  StaticDetector:\n")
	fmt.Printf("    BlackThreshold=%v | StaticThreshold=%v | Grace=%v\n",
		cfg.StaticDetector.BlackThreshold, cfg.StaticDetector.StaticThreshold, cfg.StaticDetector.Grace)
	fmt.Printf("    SkipBlack=%v | WriteBlackList=%v | SkipStatic=%v | WriteStaticList=%v\n",
		cfg.StaticDetector.SkipBlack, cfg.StaticDetector.WriteBlackList, cfg.StaticDetector.SkipStatic, cfg.StaticDetector.WriteStaticList)

	if len(cfg.Disable) > 0 {
		fmt.Println("  Disable Rules:")
		for sys, rules := range cfg.Disable {
			if len(rules.Folders) == 0 && len(rules.Files) == 0 && len(rules.Extensions) == 0 {
				fmt.Printf("    %s -> (no rules)\n", sys)
			} else {
				fmt.Printf("    %s -> Folders=%v | Files=%v | Extensions=%v\n",
					sys, rules.Folders, rules.Files, rules.Extensions)
			}
		}
	} else {
		fmt.Println("  Disable Rules: none")
	}

	return cfg, nil
}

// ---- Global helpers ----

func BaseDir() string {
	exe, err := os.Executable()
	if err != nil {
		cwd, _ := os.Getwd()
		return cwd
	}
	return filepath.Dir(exe)
}

func GamelistDir() string {
	return filepath.Join(BaseDir(), "SAM_Gamelists")
}

func FilterlistDir() string {
	return filepath.Join(GamelistDir(), "SAM_Filterlists")
}
