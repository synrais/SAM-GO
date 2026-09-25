package main

import (
	"fmt"
	"strconv"
	"strings"

	gc "github.com/rthornton128/goncurses"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/curses"
	"github.com/synrais/SAM-GO/pkg/games"
)

// -------------------------
// SAM.ini Settings Screens
// -------------------------
//
// Options -> Attract Mode (settings, Detector & list settings) and
// Options -> Game Database (Database
// systems). They edit [Attract], [StaticDetector], [List] and [Database]
// in SAM.ini; the rest of those sections stays as it is.

func onOffOption(name string, on bool) labelOption {
	o := labelOption{name, []string{"Off", "On"}, 0}
	o.setBool(on)
	return o
}

// -------- Attract mode settings --------

var playTimePresets = []string{"30", "45-60", "60-90", "90-120", "120-180"}
var idlePresets = []string{"1 min", "2 min", "5 min", "10 min", "15 min", "30 min", "60 min"}

// otherButtons are the "Other buttons" choices, and the [Attract]
// OtherInput value each one saves.
var otherButtons = []struct{ label, ini string }{
	{"Do nothing", "Ignore"},
	{"Play the game", "Play"},
	{"Back to menu", "Stop"},
}

func attractSettingsScreen(stdscr *gc.Window, cfg *config.Config, sysNames []string) {
	a := &cfg.Attract

	playTime := labelOption{"Play time (seconds)", append([]string(nil), playTimePresets...), 1}
	if pt := strings.ReplaceAll(strings.TrimSpace(a.PlayTime), " ", ""); pt != "" {
		playTime.set(pt)
		if playTime.value() != pt { // a custom value from SAM.ini: keep it
			playTime.values = append([]string{pt}, playTimePresets...)
			playTime.index = 0
		}
	}

	ids := systemIDs(sysNames)
	systems := labelOption{"Systems", []string{""}, 0}
	systemsText := func() {
		on := attractTicks(cfg, ids)
		systems.values[0] = fmt.Sprintf("%d of %d  (select to choose)", countTicks(on, ids), len(ids))
	}
	systemsText()

	// Other buttons: presses with no action in Options -> Controls.
	other := labelOption{"Other buttons", nil, 0}
	for i, ob := range otherButtons {
		other.values = append(other.values, ob.label)
		if strings.EqualFold(ob.ini, strings.TrimSpace(a.OtherInput)) {
			other.index = i
		}
	}

	// Restart if idle (after Play), and how long counts as idle.
	restart := onOffOption("Restart if idle", a.IdleRestart > 0)
	idle := labelOption{"Idle time", append([]string(nil), idlePresets...), 2} // 5 min
	if a.IdleRestart > 0 {
		v := fmt.Sprintf("%d min", a.IdleRestart)
		idle.set(v)
		if idle.value() != v {
			idle.values = append([]string{v}, idlePresets...)
			idle.index = 0
		}
	}

	runOptionsScreen(stdscr, cfg, optionsScreen{
		title:     "Attract Mode Settings",
		noPreview: true,
		options: func() []*labelOption {
			opts := []*labelOption{&playTime, &systems, &other, &restart}
			if restart.isOn() {
				opts = append(opts, &idle)
			}
			return opts
		},
		changed: func(o *labelOption) {
			if o == &systems {
				tickSystems(stdscr, "Attract Mode Systems", ids, attractTicks(cfg, ids), func(on map[string]bool) {
					a.Include = nil
					a.Exclude = unticked(ids, on)
				})
				systemsText()
			}
		},
		save: func() error {
			a.PlayTime = playTime.value()
			a.OtherInput = otherButtons[other.index].ini
			a.IdleRestart = 0
			if restart.isOn() {
				a.IdleRestart, _ = strconv.Atoi(strings.TrimSuffix(idle.value(), " min"))
			}
			return config.SaveValues(cfg.Path, "Attract", [][2]string{
				{"PlayTime", a.PlayTime},
				{"Include", strings.Join(a.Include, ", ")},
				{"Exclude", strings.Join(a.Exclude, ", ")},
				{"OtherInput", a.OtherInput},
				{"IdleRestart", strconv.Itoa(a.IdleRestart)},
			})
		},
	})
}

// attractTicks is which systems attract mode plays now, from Include and
// Exclude (which may use group names).
func attractTicks(cfg *config.Config, ids []string) map[string]bool {
	inc, _ := games.ResolveSystems(cfg.Attract.Include)
	exc, _ := games.ResolveSystems(cfg.Attract.Exclude)
	on := map[string]bool{}
	for _, id := range ids {
		l := strings.ToLower(id)
		on[id] = (len(inc) == 0 || inc[l]) && !exc[l]
	}
	return on
}

// -------- Detector & list settings --------

func detectorSettingsScreen(stdscr *gc.Window, cfg *config.Config) {
	d, l := &cfg.StaticDetector, &cfg.List
	opts := []struct {
		o   labelOption
		dst *bool
	}{
		{onOffOption("Static detector", cfg.Attract.UseStaticDetector), &cfg.Attract.UseStaticDetector},
		{onOffOption("Skip black screens", d.SkipBlack), &d.SkipBlack},
		{onOffOption("Blacklist black screens", d.WriteBlackList), &d.WriteBlackList},
		{onOffOption("Skip static screens", d.SkipStatic), &d.SkipStatic},
		{onOffOption("Staticlist static screens", d.WriteStaticList), &d.WriteStaticList},
		{onOffOption("Use blacklist", l.UseBlacklist), &l.UseBlacklist},
		{onOffOption("Use staticlist", l.UseStaticlist), &l.UseStaticlist},
		{onOffOption("Use whitelist", l.UseWhitelist), &l.UseWhitelist},
	}
	runOptionsScreen(stdscr, cfg, optionsScreen{
		title:     "Detector & List Settings",
		noPreview: true,
		options: func() []*labelOption {
			out := make([]*labelOption, len(opts))
			for i := range opts {
				out[i] = &opts[i].o
			}
			return out
		},
		save: func() error {
			for i := range opts {
				*opts[i].dst = opts[i].o.isOn()
			}
			if err := config.SaveValues(cfg.Path, "Attract", [][2]string{
				{"UseStaticDetector", boolText(cfg.Attract.UseStaticDetector)},
			}); err != nil {
				return err
			}
			if err := config.SaveValues(cfg.Path, "StaticDetector", [][2]string{
				{"SkipBlack", boolText(d.SkipBlack)}, {"WriteBlackList", boolText(d.WriteBlackList)},
				{"SkipStatic", boolText(d.SkipStatic)}, {"WriteStaticList", boolText(d.WriteStaticList)},
			}); err != nil {
				return err
			}
			return config.SaveValues(cfg.Path, "List", [][2]string{
				{"UseBlacklist", boolText(l.UseBlacklist)}, {"UseStaticlist", boolText(l.UseStaticlist)},
				{"UseWhitelist", boolText(l.UseWhitelist)},
			})
		},
	})
}

// -------- Database systems --------

// databaseSystemsScreen ticks which systems go in the games database. It
// reports whether the user asked to rebuild the database now.
func databaseSystemsScreen(stdscr *gc.Window, cfg *config.Config) bool {
	var names []string
	for _, s := range games.Systems {
		names = append(names, s.Name)
	}
	sortSystems(names)
	ids := systemIDs(names)

	exc, _ := games.ResolveSystems(cfg.Database.Exclude)
	on := map[string]bool{}
	for _, id := range ids {
		on[id] = !exc[strings.ToLower(id)]
	}

	changed := false
	tickSystems(stdscr, "Games Database Systems", ids, on, func(on map[string]bool) {
		cfg.Database.Exclude = unticked(ids, on)
		if err := config.SaveValues(cfg.Path, "Database", [][2]string{
			{"Exclude", strings.Join(cfg.Database.Exclude, ", ")},
		}); err != nil {
			message(stdscr, fmt.Sprintf("Couldn't save: %v", err))
			return
		}
		changed = true
	})
	if !changed {
		return false
	}

	stdscr.Clear()
	stdscr.Refresh()
	button, sel, err := curses.ListPicker(stdscr, curses.ListPickerOpts{
		Shortcuts:     menuShortcuts(),
		Title:         "Rebuild the games database now?",
		Buttons:       []string{"Select", "Back"},
		ActionButton:  0,
		DefaultButton: 0,
		Width:         60,
		Height:        6,
	}, []string{"Rebuild now", "Later"})
	return err == nil && button == 0 && sel == 0
}

// -------- System tick list --------

// tickSystems shows a tick list of systems. Changes are handed to save
// when leaving, only if anything changed.
func tickSystems(stdscr *gc.Window, title string, ids []string, on map[string]bool, save func(map[string]bool)) {
	labels := systemLabels(namesOf(ids), optionsWidth-10)
	changed := false
	selected := 0
	for {
		items := make([]string, len(ids))
		for i, id := range ids {
			box := "[ ]"
			if on[id] {
				box = "[x]"
			}
			items[i] = box + " " + strings.TrimLeft(labels[i], " ")
		}
		stdscr.Clear()
		stdscr.Refresh()
		button, sel, err := curses.ListPicker(stdscr, curses.ListPickerOpts{
			Shortcuts:     menuShortcuts(),
			Title:         fmt.Sprintf("%s (%d of %d)", title, countTicks(on, ids), len(ids)),
			Buttons:       []string{"Toggle", "All", "None", "Back"},
			ActionButton:  0,
			DefaultButton: 0,
			ShowTotal:     true,
			Width:         optionsWidth,
			Height:        listHeight,
			InitialIndex:  selected,
		}, items)
		if err != nil {
			break
		}
		if sel >= 0 {
			selected = sel
		}
		switch button {
		case 0:
			if sel >= 0 && sel < len(ids) {
				on[ids[sel]] = !on[ids[sel]]
				changed = true
			}
			continue
		case 1, 2:
			for _, id := range ids {
				on[id] = button == 1
			}
			changed = true
			continue
		}
		break
	}
	stdscr.Clear()
	stdscr.Refresh()
	if changed {
		save(on)
	}
}

func countTicks(on map[string]bool, ids []string) int {
	n := 0
	for _, id := range ids {
		if on[id] {
			n++
		}
	}
	return n
}

// unticked lists the IDs that are off, for an Exclude line.
func unticked(ids []string, on map[string]bool) []string {
	var out []string
	for _, id := range ids {
		if !on[id] {
			out = append(out, id)
		}
	}
	return out
}

// systemIDs turns menu display names into system IDs, keeping the order.
func systemIDs(names []string) []string {
	byName := map[string]string{}
	for _, s := range games.Systems {
		byName[s.Name] = s.Id
	}
	var ids []string
	for _, n := range names {
		if id, ok := byName[n]; ok {
			ids = append(ids, id)
		}
	}
	return ids
}

func namesOf(ids []string) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id
		if s, err := games.GetSystem(id); err == nil {
			out[i] = s.Name
		}
	}
	return out
}

func boolText(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
