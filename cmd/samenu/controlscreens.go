package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	gc "github.com/rthornton128/goncurses"

	"github.com/synrais/SAMenu/pkg/config"
	"github.com/synrais/SAMenu/pkg/curses"
	"github.com/synrais/SAMenu/pkg/input"
)

// -------------------------
// Control Binding Screens
// -------------------------
//
// Options -> Controls: the menu's button layout, then attract mode's
// controls. Pick an attract action, then press what you want for it: a new
// key or button is added, one that is already bound to it is removed.
// Waiting bindTimeout cancels. The menu's own Search (Y) and Options (X)
// buttons are fixed and not listed.

const bindTimeout = 10 * time.Second

var layoutText = map[string]string{
	"Western":  "Western (A confirms, B backs out)",
	"Japanese": "Japanese (B confirms, A backs out)",
}

var attractActionNames = map[string]string{
	"next": "Next game", "back": "Previous game", "play": "Play this game",
	"stop": "Stop attract", "blacklist": "Blacklist game", "stay": "Stay on game",
	"menu": "Open SAMenu", "search": "Search (x2: menu)",
}

// bindingPicker shows a list of actions plus "Restore defaults" and returns
// the chosen index (len(actions) for defaults), or -1 for Back.
func bindingPicker(stdscr *gc.Window, title string, items []string, selected int) int {
	clearScreen(stdscr)
	items = append(items, "Restore defaults")
	button, sel, err := curses.ListPicker(stdscr, curses.ListPickerOpts{
		Shortcuts:     menuShortcuts(),
		Title:         title,
		Buttons:       []string{"Change", "Back"},
		DefaultButton: 0,
		ActionButton:  0,
		Width:         optionsWidth,
		Height:        len(items) + 4,
		InitialIndex:  selected,
	}, items)
	if err != nil || button != 0 {
		return -1
	}
	return sel
}

// message shows a short note for a moment.
func message(stdscr *gc.Window, text string) {
	_ = curses.InfoBox(stdscr, "", text, true, false)
	gc.Nap(1500)
}

// -------- Controls --------

// menuInputs are the input detectors, started the first time they're
// needed and kept running (they can't be stopped).
var menuInputs <-chan input.Event

// controlsScreen is Options -> Controls: Menu Layout first, then the input
// switches and attract mode's actions.
func controlsScreen(stdscr *gc.Window, cfg *config.Config) {
	onOff := func(b bool) string {
		if b {
			return "On"
		}
		return "Off"
	}
	// The switches: each kind of input for attract controls, and BIOS skip
	// (button presses after a game loads) for attract and menu launches.
	switches := []struct {
		name string
		on   *bool
	}{
		{"Keyboard", &cfg.InputDetector.Keyboard},
		{"Controller", &cfg.InputDetector.Joystick},
		{"Mouse", &cfg.InputDetector.Mouse},
		{"BIOS skip, attract", &cfg.AutoInput.Attract},
		{"BIOS skip, menu", &cfg.AutoInput.Menu},
	}

	selected := 0
	for {
		items := []string{fmt.Sprintf("%-20s %s", "Menu Layout:", layoutText[cfg.MenuLayout])}
		for _, sw := range switches {
			items = append(items, fmt.Sprintf("%-20s %s", sw.name+":", onOff(*sw.on)))
		}
		for _, act := range config.AttractActions {
			items = append(items, fitText(fmt.Sprintf("%-20s %s", attractActionNames[act]+":", attractBindingText(cfg, act)), optionsWidth-6))
		}
		sel := bindingPicker(stdscr, "Controls", items, selected)
		if sel < 0 {
			break
		}
		selected = sel

		if sel == 0 {
			if cfg.MenuLayout == "Japanese" {
				cfg.MenuLayout = "Western"
			} else {
				cfg.MenuLayout = "Japanese"
			}
			curses.SwapConfirmBack = cfg.MenuLayout == "Japanese"
			if err := config.SaveMenuLayout(cfg); err != nil {
				message(stdscr, fmt.Sprintf("Couldn't save: %v", err))
			}
			continue
		}

		if sel <= len(switches) {
			sw := switches[sel-1]
			*sw.on = !*sw.on
			err := config.SaveInputDetector(cfg)
			if err == nil {
				err = config.SaveValues(cfg.Path, "BiosSkip", [][2]string{
					{"Attract", strconv.FormatBool(cfg.AutoInput.Attract)}, {"Menu", strconv.FormatBool(cfg.AutoInput.Menu)},
				})
			}
			if err != nil {
				message(stdscr, fmt.Sprintf("Couldn't save: %v", err))
			}
			continue
		}

		previous := copyBinds(cfg.AttractControls)
		if sel == len(items) {
			// Restore defaults: layout and attract controls.
			cfg.AttractControls = config.DefaultAttractControlsCopy()
			cfg.MenuLayout = "Western"
			curses.SwapConfirmBack = false
			if err := config.SaveMenuLayout(cfg); err != nil {
				message(stdscr, fmt.Sprintf("Couldn't save: %v", err))
			}
		} else {
			act := config.AttractActions[sel-1-len(switches)]
			ev, ok := waitForInput(stdscr, fmt.Sprintf("Press a key, button or mouse button for\n%s (a bound one removes it, wait %ds to cancel)",
				attractActionNames[act], int(bindTimeout.Seconds())))
			if !ok {
				continue
			}
			binds := cfg.AttractControls[ev.Kind]
			if binds == nil {
				binds = map[string]string{}
				cfg.AttractControls[ev.Kind] = binds
			}
			name := strings.ToLower(ev.Name)
			if binds[name] == act {
				delete(binds, name)
			} else {
				binds[name] = act // also takes it off any other action
			}
		}
		if err := config.SaveAttractControls(cfg, previous); err != nil {
			message(stdscr, fmt.Sprintf("Couldn't save: %v", err))
		}
	}
	clearScreen(stdscr)
}

// waitForInput waits for a press from the input detectors. Mouse movement
// and the wheel are ignored, so a nudged mouse can't bind by accident.
func waitForInput(stdscr *gc.Window, prompt string) (input.Event, bool) {
	if menuInputs == nil {
		menuInputs = input.Start(input.Options{Keyboard: true, Mouse: true, Joystick: true, Quiet: true})
		time.Sleep(300 * time.Millisecond) // let the devices be found
	}
	for len(menuInputs) > 0 { // drop anything pressed earlier
		<-menuInputs
	}
	_ = curses.InfoBox(stdscr, "", prompt, true, false)

	defer func() {
		// The same press usually also reaches the menu as a key (MiSTer
		// turns controller buttons into keys), so drop it.
		time.Sleep(150 * time.Millisecond)
		_ = gc.FlushInput()
	}()
	timeout := time.After(bindTimeout)
	for {
		select {
		case ev := <-menuInputs:
			if ev.Kind == "mouse" && (strings.HasPrefix(ev.Name, "swipe") || strings.HasPrefix(ev.Name, "wheel")) {
				continue
			}
			return ev, true
		case <-timeout:
			return input.Event{}, false
		}
	}
}

// attractBindingText lists an action's inputs, e.g.
// "key right | pad dpright, leftx+ | mouse right".
func attractBindingText(cfg *config.Config, act string) string {
	labels := map[string]string{"keyboard": "key", "joystick": "pad", "mouse": "mouse"}
	var parts []string
	for _, kind := range []string{"keyboard", "joystick", "mouse"} {
		var ins []string
		for in, a := range cfg.AttractControls[kind] {
			if a == act {
				ins = append(ins, in)
			}
		}
		if len(ins) > 0 {
			sort.Strings(ins)
			parts = append(parts, labels[kind]+" "+strings.Join(ins, ", "))
		}
	}
	if len(parts) == 0 {
		return "(none)"
	}
	return strings.Join(parts, " | ")
}

func copyBinds(m map[string]map[string]string) map[string]map[string]string {
	out := map[string]map[string]string{}
	for k, v := range m {
		out[k] = map[string]string{}
		for a, b := range v {
			out[k][a] = b
		}
	}
	return out
}
