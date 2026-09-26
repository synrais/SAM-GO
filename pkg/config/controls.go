package config

import (
	"os"
	"sort"
	"strings"
)

// Controls
//
// SAMenu's shortcuts are fixed: Y (tab) = Search, X (space) =
// Options. A controller works in the menu because MiSTer turns its buttons
// into keys (A = enter, B = esc, X = space, Y = tab, L/R = page up/down).
// [Controls.Menu] only holds the button Layout (Western or Japanese).
//
// [InputDetector.Keyboard], [InputDetector.Mouse] and [InputDetector.Joystick]
// bind attract mode actions to inputs, as input = action ("dpright = next"),
// using the names the input test (SAMenu.sh -inputs) prints.

// Menu actions, on fixed keys (DefaultMenuControls).
// Back is fixed on esc (B) and backspace, swapped with confirm by the
// Japanese button layout.
var MenuActions = []string{"search", "options"}

// DefaultMenuControls: Y = search, X = options.
var DefaultMenuControls = map[string][]string{
	"search":  {"tab"},
	"options": {"space"},
}

// Attract mode actions ("back" is the previous game).
var AttractActions = []string{"next", "back", "play", "stop", "blacklist", "stay", "menu", "search"}

// Attract input kinds, matching the [InputDetector.X] section names.
var AttractKinds = []string{"keyboard", "mouse", "joystick"}

// DefaultAttractControls: kind -> input -> action.
var DefaultAttractControls = map[string]map[string]string{
	"keyboard": {"right": "next", "left": "back", "enter": "play", "esc": "stop", "delete": "blacklist", "space": "stay", "`": "search"},
	"mouse":    {"right": "next", "left": "back"},
	"joystick": {"dpright": "next", "dpleft": "back", "leftx+": "next", "leftx-": "back", "start": "play", "back": "stop"},
}

func copyMenuControls(m map[string][]string) map[string][]string {
	out := make(map[string][]string, len(m))
	for k, v := range m {
		out[k] = append([]string(nil), v...)
	}
	return out
}

func copyAttractControls(m map[string]map[string]string) map[string]map[string]string {
	out := make(map[string]map[string]string, len(m))
	for kind, binds := range m {
		out[kind] = make(map[string]string, len(binds))
		for in, act := range binds {
			out[kind][in] = act
		}
	}
	return out
}

// DefaultMenuControlsCopy and DefaultAttractControlsCopy return fresh
// copies of the defaults, safe to change.
func DefaultMenuControlsCopy() map[string][]string { return copyMenuControls(DefaultMenuControls) }
func DefaultAttractControlsCopy() map[string]map[string]string {
	return copyAttractControls(DefaultAttractControls)
}

// SaveMenuLayout writes [Controls.Menu] Layout.
func SaveMenuLayout(cfg *Config) error {
	return SaveValues(cfg.Path, "Controls.Menu", [][2]string{{"Layout", cfg.MenuLayout}})
}

// SaveInputDetector writes the [InputDetector] on/off switches.
func SaveInputDetector(cfg *Config) error {
	d := cfg.InputDetector
	return SaveValues(cfg.Path, "InputDetector", [][2]string{
		{"Mouse", boolText(d.Mouse)}, {"Keyboard", boolText(d.Keyboard)}, {"Joystick", boolText(d.Joystick)},
	})
}

func boolText(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// SaveAttractControls writes the three [InputDetector.X] sections. Inputs
// that were bound before but aren't now are kept with an empty value, so
// the file's own list of buttons stays intact.
func SaveAttractControls(cfg *Config, previous map[string]map[string]string) error {
	for _, kind := range AttractKinds {
		var values [][2]string
		seen := map[string]bool{}
		for in, act := range cfg.AttractControls[kind] {
			values = append(values, [2]string{in, act})
			seen[in] = true
		}
		for in := range previous[kind] {
			if !seen[in] {
				values = append(values, [2]string{in, ""})
			}
		}
		sort.Slice(values, func(i, j int) bool { return values[i][0] < values[j][0] })
		if err := SaveValues(cfg.Path, "InputDetector."+titleCase(kind), values); err != nil {
			return err
		}
	}
	return nil
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// SaveValues sets keys in one section of an INI file, keeping everything
// else (comments, spacing, other sections) exactly as it is.
func SaveValues(path, section string, values [][2]string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	newline := "\n"
	if strings.Contains(string(data), "\r\n") {
		newline = "\r\n"
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	lines = setSectionValues(lines, section, values)

	// Write to a temp file first, so a failed write can't leave the file
	// half written.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(strings.Join(lines, newline)), 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// iniKey writes a key so the INI reader gets it back: keys with characters
// that mean something in INI files are quoted.
func iniKey(k string) string {
	if strings.ContainsAny(k, "=:;#[]\"` ") {
		if strings.Contains(k, "\"") {
			return "`" + k + "`"
		}
		return "\"" + k + "\""
	}
	return k
}
