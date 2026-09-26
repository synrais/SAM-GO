package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/ini.v1"

	"github.com/synrais/SAMenu/pkg/assets"
)

// SAMFolder is where SAMenu and its SAMenu.ini live.
const SAMFolder = ScriptsFolder + "/.MiSTer_SAMenu"

// SAMIniFile is SAMenu's one config file, used by every part of it.
// Set SAMENU_CONFIG to use a different file.
const SAMIniFile = SAMFolder + "/SAMenu.ini"

// --------------------------------------------------
//  Structs
// --------------------------------------------------

type SystemsConfig struct {
	GamesFolder  []string `ini:"games_folder,omitempty,allowshadow"`
	SystemFolder []string `ini:"system_folder,omitempty,allowshadow"`
	SetCore      []string `ini:"set_core,omitempty,allowshadow"`
}

type AttractConfig struct {
	PlayTime          string   `ini:"playtime"`
	Include           []string `ini:"include" delim:","`
	Exclude           []string `ini:"exclude" delim:","`
	UseStaticDetector bool     `ini:"usestaticdetector"`
	// What input with no binding does: Ignore, Stop or Play.
	OtherInput string `ini:"otherinput"`
	// Mute MiSTer's sound while attract mode plays.
	Mute bool `ini:"mute"`
	// How a game is chosen (see pkg/attract/picker.go): Selection is one
	// of attract.SelectionModes; NoRepeats plays every game once before
	// any repeats; MixSystems never picks the same system twice in a row;
	// OneVersion counts a title's regions, revisions and discs as one.
	Selection  string `ini:"selection"`
	NoRepeats  bool   `ini:"norepeats"`
	MixSystems bool   `ini:"mixsystems"`
	OneVersion bool   `ini:"oneversion"`
	// Never pick games with these name tags (gamesdb.Tags).
	SkipTags []string `ini:"skiptags" delim:","`
	// Arcade games' screen orientation to play: Both, Horizontal, Vertical,
	// Vertical CW or Vertical CCW (from each MRA's <rotation>). Games whose
	// orientation isn't known are skipped unless Both.
	Orientation string `ini:"orientation"`
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

// DatabaseConfig is the [Database] section: whole systems or groups to
// leave out of the games database.
type DatabaseConfig struct {
	Exclude []string `ini:"exclude" delim:","`
}

// Rules are the Folders/Files/Extensions/Paths lines of a [Database.X] or
// [Attract.X] section, where X is ALL, a system ID or a group.
type Rules struct {
	Folders    []string `ini:"folders" delim:","`
	Files      []string `ini:"files" delim:","`
	Extensions []string `ini:"extensions" delim:","`
	Paths      []string `ini:"paths" delim:","`
}

// Add appends another set of rules to these.
func (r *Rules) Add(other Rules) {
	r.Folders = append(r.Folders, other.Folders...)
	r.Files = append(r.Files, other.Files...)
	r.Extensions = append(r.Extensions, other.Extensions...)
	r.Paths = append(r.Paths, other.Paths...)
}

// StaticDetectorConfig is the [StaticDetector] section. Times are seconds.
type StaticDetectorConfig struct {
	BlackThreshold  float64 `ini:"blackthreshold"`
	StaticThreshold float64 `ini:"staticthreshold"`
	SkipBlack       bool    `ini:"skipblack"`
	WriteBlackList  bool    `ini:"writeblacklist"`
	SkipStatic      bool    `ini:"skipstatic"`
	WriteStaticList bool    `ini:"writestaticlist"`
	Grace           float64 `ini:"grace"`
	// The screen is split into a 16x12 grid of cells. It counts as moving
	// only if at least MinSpread cells changed within the last SpreadTime
	// seconds, so change stuck in one small spot (a flashing cursor, a
	// blinking "PRESS START") counts as static, while small sprites that
	// travel around the screen count as moving. 1 = any change at all.
	MinSpread  int     `ini:"minspread"`
	SpreadTime float64 `ini:"spreadtime"`
}

// MusicConfig is [Music]: the background music player.
type MusicConfig struct {
	Playback     string `ini:"playback"`     // Random, In order
	Playlist     string `ini:"playlist"`     // "" = the music folder, All, or a folder in it
	PauseInGames bool   `ini:"pauseingames"` // pause while a game core is loaded
}

// VideoConfig is [Video]: the video player and videos in attract mode.
type VideoConfig struct {
	Playback     string `ini:"playback"`     // Random, In order
	Playlist     string `ini:"playlist"`     // "" = the video folder, All, or a folder in it
	AttractEvery int    `ini:"attractevery"` // play a video every N attract games (0 = never)

	// MPlayer sync settings (mostly for keeping sound in step after seeking)
	AutoSync   bool `ini:"autosync"`   // all files: correct audio/video drift quickly (-autosync 30)
	CorrectPts bool `ini:"correctpts"` // MP4/MKV: use their timestamps (-correct-pts)
	Mp3Seek    bool `ini:"mp3seek"`    // AVI/other with MP3 audio: accurate seeking (-hr-mp3-seek)
	AviIndex   bool `ini:"aviindex"`   // AVI without an index: build one (-idx)
}

// StartupConfig is [Startup]: what happens when the MiSTer starts (kept
// in a marked block of /media/fat/linux/user-startup.sh).
type StartupConfig struct {
	Start string `ini:"start"` // Nothing, SAMenu, Attract mode
	Music bool   `ini:"music"` // start the music player
	// Attract mode on boot: seconds to wait after the MiSTer has started
	// (0 = straight away), and what a press during that wait does:
	// "Restarts the countdown", "Cancels it" or "Is ignored".
	AttractDelay int    `ini:"attractdelay"`
	AttractPress string `ini:"attractpress"`
	// When attract mode starts: "Instantly", "After a delay" (AttractDelay)
	// or "When idle": whenever nothing has been pressed for IdleTime
	// minutes, in IdleWhere ("MiSTer menu", "Games" or "Menu+Games"), from boot on.
	AttractWhen string `ini:"attractwhen"`
	IdleTime    int    `ini:"idletime"`
	IdleWhere   string `ini:"idlewhere"`
}

// AutoInputConfig is [BiosSkip]: button presses after a game loads, to
// get past a BIOS screen, from [BiosSkip.<system>] Sequence lines.
type AutoInputConfig struct {
	Attract   bool              `ini:"attract"` // after attract mode launches
	Menu      bool              `ini:"menu"`    // after SAMenu launches
	Sequences map[string]string `ini:"-"`       // lower-case system ID -> sequence
}

// InputDetectorConfig is the [InputDetector] section: which kinds of
// input device are watched while attract mode runs.
type InputDetectorConfig struct {
	Mouse    bool `ini:"mouse"`
	Keyboard bool `ini:"keyboard"`
	Joystick bool `ini:"joystick"`
}

// MenuConfig holds SAMenu's list label settings. Values are stored
// by name (e.g. "Center", "Colon") so new choices never shift old ones.
type MenuConfig struct {
	// Show [Pick Random Game] at the top of the systems list and every
	// folder.
	RandomEntry bool `ini:"randomentry"`
	// Hide games with these name tags from the menu (gamesdb.Tags), and
	// from search too with SearchHidden.
	HideTags     []string `ini:"hidetags" delim:","`
	SearchHidden bool     `ini:"searchhidden"`

	LabelAlign        string `ini:"labelalign"`
	LabelDivider      string `ini:"labeldivider"`
	LabelAlignDivider bool   `ini:"labelaligndivider"`
	LabelEncapsulate  string `ini:"labelencapsulate"`
	ShowManufacturer  bool   `ini:"showmanufacturer"`
	TextSize          string `ini:"textsize"` // Auto, Normal, Large, Extra large, Huge
	RememberPosition  bool   `ini:"rememberposition"`

	// Menu list sorting
	SystemGroup      string `ini:"systemgroup"`      // Manufacturer, Category, None
	SystemGroupOrder string `ini:"systemgrouporder"` // Alphabetical, Oldest first, Custom
	CategoryOrder    string `ini:"categoryorder"`    // e.g. "Arcade, Console, Handheld, Computer, Other"
	SystemSubgroup   string `ini:"systemsubgroup"`   // Manufacturer, Category, None
	GroupHeaders     string `ini:"groupheaders"`     // Off, Line, Double, Brackets, Dots, Minimal
	GroupView        string `ini:"groupview"`        // List, Folders
	SystemOrder      string `ini:"systemorder"`      // Release date, A-Z
	ArcadeFirst      bool   `ini:"arcadefirst"`

	// Game list sorting (folders and search results)
	FolderPosition string `ini:"folderposition"` // First, Last, Mixed
	GameOrder      string `ini:"gameorder"`      // Alphabetical, Natural
	IgnoreThe      bool   `ini:"ignorethe"`
	HideExtensions bool   `ini:"hideextensions"`
	GroupDiscs     bool   `ini:"groupdiscs"`

	// Systems (IDs or groups) that get a virtual [Games A-Z] folder.
	VirtualAZFolders []string `ini:"virtualazfolders" delim:","`
}

type Config struct {
	Path    string
	Created bool // SAMenu.ini didn't exist and was written from the default
	// [Weights]: lowercase system ID or group -> how much more (or less)
	// often it's picked, e.g. snes = 3, computer = 0.5 (0 = never).
	Weights  map[string]float64
	Systems  SystemsConfig
	Database DatabaseConfig
	Attract  AttractConfig
	List     ListConfig
	Menu     MenuConfig

	InputDetector InputDetectorConfig
	AutoInput     AutoInputConfig
	Music         MusicConfig
	Video         VideoConfig
	Startup       StartupConfig
	// Menu shortcuts (fixed defaults). [InputDetector.X]: kind -> input -> action.
	MenuControls    map[string][]string
	MenuLayout      string // "Western" (A confirms) or "Japanese" (B confirms)
	AttractControls map[string]map[string]string
	StaticDetector  StaticDetectorConfig
	// [StaticDetector.X] overrides: lowercase X (a system ID or group) ->
	// lowercase key -> value, for only the keys that section sets.
	StaticDetectorOverrides map[string]map[string]string

	// Per-target rules, keyed by the lowercase part after the dot in the
	// section name ("all", a system ID or a group).
	DatabaseRules map[string]Rules
	AttractRules  map[string]Rules
}

// --------------------------------------------------
//  Loading
// --------------------------------------------------

// IniPath returns the SAMenu.ini in use: SAMENU_CONFIG if set, otherwise SAMIniFile.
func IniPath() string {
	if p := os.Getenv(UserConfigEnv); p != "" {
		return p
	}
	return SAMIniFile
}

// Load reads SAMenu.ini, writing the embedded default first if it's missing.
// Section and key names are matched ignoring case, and sections the code
// doesn't use (old or future ones) are simply skipped.
func Load() (*Config, error) {
	cfg := &Config{
		Path:                    IniPath(),
		DatabaseRules:           make(map[string]Rules),
		AttractRules:            make(map[string]Rules),
		StaticDetectorOverrides: make(map[string]map[string]string),
		InputDetector:           InputDetectorConfig{Mouse: true, Keyboard: true, Joystick: true},
		AutoInput:               AutoInputConfig{Attract: true, Menu: true},
		Music:                   MusicConfig{Playback: "Random", PauseInGames: true},
		Video:                   VideoConfig{AutoSync: true, Playback: "Random"},
		Startup:                 StartupConfig{Start: "Nothing", AttractWhen: "Instantly", AttractDelay: 60, AttractPress: "Restarts the countdown", IdleTime: 5, IdleWhere: IdleMenu},
		Menu:                    MenuConfig{ShowManufacturer: true, ArcadeFirst: true, RandomEntry: true, SearchHidden: true},
		Attract: AttractConfig{
			Selection: "Balanced", NoRepeats: true, MixSystems: true, OneVersion: true,
		},
		Weights: map[string]float64{},
		StaticDetector: StaticDetectorConfig{
			BlackThreshold: 30, StaticThreshold: 30, Grace: 25,
			MinSpread: 8, SpreadTime: 5,
		},
	}

	if _, err := os.Stat(cfg.Path); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(cfg.Path), 0755); err != nil {
			return cfg, fmt.Errorf("create config folder: %w", err)
		}
		if err := os.WriteFile(cfg.Path, assets.DefaultSAMIni, 0644); err != nil {
			return cfg, fmt.Errorf("write default SAMenu.ini: %w", err)
		}
		cfg.Created = true
	}

	file, err := ini.LoadSources(ini.LoadOptions{Insensitive: true, AllowShadows: true}, cfg.Path)
	if err != nil {
		return cfg, err
	}

	for name, dest := range map[string]interface{}{
		"systems":        &cfg.Systems,
		"database":       &cfg.Database,
		"attract":        &cfg.Attract,
		"list":           &cfg.List,
		"menu":           &cfg.Menu,
		"staticdetector": &cfg.StaticDetector,
		"inputdetector":  &cfg.InputDetector,
		"biosskip":       &cfg.AutoInput,
		"music":          &cfg.Music,
		"video":          &cfg.Video,
		"startup":        &cfg.Startup,
	} {
		if err := file.Section(name).MapTo(dest); err != nil {
			return cfg, fmt.Errorf("read [%s]: %w", name, err)
		}
	}

	loadControls(cfg, file)

	// [Weights] name = number
	if sec, err := file.GetSection("weights"); err == nil {
		for _, k := range sec.Keys() {
			if w, err := k.Float64(); err == nil && w >= 0 {
				cfg.Weights[strings.ToLower(strings.TrimSpace(k.Name()))] = w
			}
		}
	}

	// [BiosSkip.<system>] Sequence = ...
	cfg.AutoInput.Sequences = map[string]string{}
	for _, sec := range file.Sections() {
		name := strings.ToLower(sec.Name())
		if id, ok := strings.CutPrefix(name, "biosskip."); ok && id != "" {
			if seq := strings.TrimSpace(sec.Key("sequence").String()); seq != "" {
				cfg.AutoInput.Sequences[id] = seq
			}
		}
	}

	// [Database.X] and [Attract.X] rule sections. Old [Disable.X] sections
	// are read as Database rules so existing SAMenu.ini files keep working.
	for _, sec := range file.Sections() {
		var dest map[string]Rules
		name := sec.Name()
		switch {
		case strings.HasPrefix(name, "database."):
			dest, name = cfg.DatabaseRules, strings.TrimPrefix(name, "database.")
		case strings.HasPrefix(name, "disable."):
			dest, name = cfg.DatabaseRules, strings.TrimPrefix(name, "disable.")
		case strings.HasPrefix(name, "attract."):
			dest, name = cfg.AttractRules, strings.TrimPrefix(name, "attract.")
		case strings.HasPrefix(name, "staticdetector."):
			keys := make(map[string]string)
			for _, k := range sec.Keys() {
				keys[strings.ToLower(k.Name())] = k.String()
			}
			cfg.StaticDetectorOverrides[strings.TrimPrefix(name, "staticdetector.")] = keys
			continue
		default:
			continue
		}

		var rules Rules
		if err := sec.MapTo(&rules); err != nil {
			return cfg, fmt.Errorf("read [%s]: %w", sec.Name(), err)
		}
		existing := dest[name]
		existing.Add(rules)
		dest[name] = existing
	}

	return cfg, nil
}

// --------------------------------------------------
//  Saving
// --------------------------------------------------

// SaveMenu writes cfg.Menu into the [Menu] section of SAMenu.ini. Only those
// lines are touched: every other line, comment and blank line in the file
// stays exactly as it was. A missing [Menu] section or key is added.
func SaveMenu(cfg *Config) error {
	values := [][2]string{
		{"LabelAlign", cfg.Menu.LabelAlign},
		{"LabelDivider", cfg.Menu.LabelDivider},
		{"LabelAlignDivider", fmt.Sprint(cfg.Menu.LabelAlignDivider)},
		{"LabelEncapsulate", cfg.Menu.LabelEncapsulate},
		{"ShowManufacturer", fmt.Sprint(cfg.Menu.ShowManufacturer)},
		{"TextSize", cfg.Menu.TextSize},
		{"RememberPosition", fmt.Sprint(cfg.Menu.RememberPosition)},
		{"SystemGroup", cfg.Menu.SystemGroup},
		{"SystemGroupOrder", cfg.Menu.SystemGroupOrder},
		{"CategoryOrder", cfg.Menu.CategoryOrder},
		{"SystemSubgroup", cfg.Menu.SystemSubgroup},
		{"GroupHeaders", cfg.Menu.GroupHeaders},
		{"GroupView", cfg.Menu.GroupView},
		{"SystemOrder", cfg.Menu.SystemOrder},
		{"ArcadeFirst", fmt.Sprint(cfg.Menu.ArcadeFirst)},
		{"FolderPosition", cfg.Menu.FolderPosition},
		{"GameOrder", cfg.Menu.GameOrder},
		{"IgnoreThe", fmt.Sprint(cfg.Menu.IgnoreThe)},
		{"HideExtensions", fmt.Sprint(cfg.Menu.HideExtensions)},
		{"GroupDiscs", fmt.Sprint(cfg.Menu.GroupDiscs)},
		{"HideTags", strings.Join(cfg.Menu.HideTags, ", ")},
		{"SearchHidden", fmt.Sprint(cfg.Menu.SearchHidden)},
	}
	return SaveValues(cfg.Path, "Menu", values)
}

// SaveVirtualAZFolders writes [Menu] VirtualAZFolders.
func SaveVirtualAZFolders(cfg *Config) error {
	return SaveValues(cfg.Path, "Menu", [][2]string{{"VirtualAZFolders", strings.Join(cfg.Menu.VirtualAZFolders, ", ")}})
}

// setSectionValues sets each key in the named section (both matched
// ignoring case), keeping the file's own "key = value" spacing.
func setSectionValues(lines []string, section string, values [][2]string) []string {
	start, end := -1, len(lines)
	for i, line := range lines {
		name, ok := sectionName(line)
		if !ok {
			continue
		}
		if start >= 0 {
			end = i
			break
		}
		if strings.EqualFold(name, section) {
			start = i
		}
	}

	// No section yet: add it at the end of the file.
	if start < 0 {
		for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
			lines = lines[:len(lines)-1]
		}
		lines = append(lines, "", "["+section+"]")
		for _, kv := range values {
			lines = append(lines, iniKey(kv[0])+" = "+kv[1])
		}
		return append(lines, "")
	}

	var missing []string
	for _, kv := range values {
		found := false
		for i := start + 1; i < end; i++ {
			if key, eq, ok := keyOnLine(lines[i]); ok && strings.EqualFold(key, kv[0]) {
				lines[i] = lines[i][:eq+1] + " " + kv[1]
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, iniKey(kv[0])+" = "+kv[1])
		}
	}

	// Add missing keys after the section's last non-blank line.
	if len(missing) > 0 {
		at := end
		for at > start+1 && strings.TrimSpace(lines[at-1]) == "" {
			at--
		}
		rest := append(missing, lines[at:]...)
		lines = append(lines[:at], rest...)
	}
	return lines
}

// sectionName returns the name of a "[Section]" line.
func sectionName(line string) (string, bool) {
	t := strings.TrimSpace(line)
	if len(t) < 2 || t[0] != '[' || t[len(t)-1] != ']' {
		return "", false
	}
	return strings.TrimSpace(t[1 : len(t)-1]), true
}

// keyOnLine returns the key of a "key = value" line and the index of its
// "=". Comments and other lines report false.
func keyOnLine(line string) (string, int, bool) {
	t := strings.TrimSpace(line)
	if t == "" || t[0] == ';' || t[0] == '#' {
		return "", 0, false
	}
	// A quoted key ("`" or `"`) can contain "=", so find its closing quote.
	lead := len(line) - len(strings.TrimLeft(line, " \t"))
	if q := t[0]; q == '"' || q == '`' {
		if close := strings.IndexByte(line[lead+1:], q); close >= 0 {
			keyEnd := lead + 1 + close
			if eq := strings.Index(line[keyEnd:], "="); eq >= 0 {
				return line[lead+1 : keyEnd], keyEnd + eq, true
			}
		}
	}
	eq := strings.Index(line, "=")
	if eq < 0 {
		return "", 0, false
	}
	return strings.TrimSpace(line[:eq]), eq, true
}

// loadControls reads [Controls.Menu] Layout and the [InputDetector.X]
// sections. A missing section uses the defaults. The menu's Search and
// Options buttons are fixed (Y = Search, X = Options): any Search or
// Options lines left in an older SAMenu.ini are ignored.
func loadControls(cfg *Config, file *ini.File) {
	cfg.MenuControls = DefaultMenuControlsCopy()
	cfg.MenuLayout = "Western"
	if sec, err := file.GetSection("controls.menu"); err == nil {
		if k, err := sec.GetKey("layout"); err == nil && strings.EqualFold(strings.TrimSpace(k.String()), "japanese") {
			cfg.MenuLayout = "Japanese"
		}
	}

	cfg.AttractControls = DefaultAttractControlsCopy()
	for _, kind := range AttractKinds {
		sec, err := file.GetSection("inputdetector." + kind)
		if err != nil {
			continue
		}
		binds := map[string]string{}
		for _, k := range sec.Keys() {
			if act := strings.ToLower(strings.TrimSpace(k.String())); act != "" {
				binds[strings.ToLower(k.Name())] = act
			}
		}
		cfg.AttractControls[kind] = binds
	}
}

// Idle places for [Startup] IdleWhere: where idle time counts.
const (
	IdleMenu  = "MiSTer menu"
	IdleGames = "Games"
	IdleBoth  = "Menu+Games" // the MiSTer menu, SAMenu and games
)

// StartsMenu reports whether SAMenu opens on boot ([Startup] Start).
func (c *Config) StartsMenu() bool { return c.Startup.Start == "SAMenu" }

// IdleWatch reports whether the idle watcher should run: attract mode
// starts "When idle" (from boot on).
func (c *Config) IdleWatch() bool {
	return c.Startup.Start == "Attract mode" && c.Startup.AttractWhen == "When idle"
}
