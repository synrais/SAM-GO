package main

import (
	"strings"

	gc "github.com/rthornton128/goncurses"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/games"
	"github.com/synrais/SAM-GO/pkg/gamesdb"
)

// -------------------------
// Sorting Options
// -------------------------
//
// Options -> Display & Sorting -> Systems list sorting and Options -> Display & Sorting -> Game list sorting, saved
// to SAM.ini's [Menu]. The defaults are how the menu has always sorted.

// categoryPresets are the category orders offered in the menu. Any other
// order can be written in SAM.ini, and is then offered too.
var categoryPresets = []string{
	"Arcade, Console, Handheld, Computer, Other",
	"Console, Handheld, Computer, Arcade, Other",
	"Console, Arcade, Handheld, Computer, Other",
	"Computer, Console, Handheld, Arcade, Other",
}

var (
	optGroup         = labelOption{"First group by", []string{"Manufacturer", "Category", "None"}, 0}
	optGroupOrder    = labelOption{"First group order", []string{"A-Z", "Oldest first"}, 0}
	optCategoryOrder = labelOption{"Category order", append([]string(nil), categoryPresets...), 0}
	optSubgroup      = labelOption{"Then group by", []string{"None", "Manufacturer", "Category"}, 0}
	optWithin        = labelOption{"Systems order", []string{"Release date", "A-Z"}, 0}
	optArcadeFirst   = labelOption{"Arcade Cores at top", []string{"Off", "On"}, 1}

	optFolders    = labelOption{"Folders", []string{"First", "Last", "Mixed"}, 0}
	optGameOrder  = labelOption{"Order", []string{"Alphabetical", "Natural"}, 0}
	optIgnoreThe  = labelOption{"Ignore leading \"The\"", []string{"Off", "On"}, 0}
	optGroupDiscs = labelOption{"Group disc sets", []string{"Off", "On"}, 0}
	optShowExt    = labelOption{"Extensions", []string{"Show", "Hide"}, 0}
)

// gameName is how a game is shown in lists: with or without its extension.
func gameName(name, ext string) string {
	if ext == "" || optShowExt.isOn() {
		return name
	}
	return name + "." + ext
}

// applySortConfig loads the sorting settings from SAM.ini's [Menu].
func applySortConfig(m config.MenuConfig) {
	optGroup.set(m.SystemGroup)
	optGroupOrder.set(azName(m.SystemGroupOrder))
	if order := normaliseOrder(m.CategoryOrder); order != "" {
		found := false
		for i, v := range optCategoryOrder.values {
			if strings.EqualFold(v, order) {
				optCategoryOrder.index, found = i, true
			}
		}
		if !found { // a custom order from SAM.ini: keep it, offered first
			optCategoryOrder.values = append([]string{order}, categoryPresets...)
			optCategoryOrder.index = 0
		}
	}
	optSubgroup.set(m.SystemSubgroup)
	optWithin.set(azName(m.SystemOrder))
	optHeaders.set(m.GroupHeaders)
	optGroupView.set(m.GroupView)
	optArcadeFirst.setBool(m.ArcadeFirst)
	fixSortOptions()

	optFolders.set(m.FolderPosition)
	optGameOrder.set(m.GameOrder)
	optIgnoreThe.setBool(m.IgnoreThe)
	optGroupDiscs.setBool(m.GroupDiscs)
	optShowExt.setBool(m.HideExtensions)
}

// storeSortConfig copies the sorting settings into m, for saving.
func storeSortConfig(m *config.MenuConfig) {
	m.SystemGroup = optGroup.value()
	m.SystemGroupOrder = groupOrder()
	m.CategoryOrder = optCategoryOrder.value()
	m.SystemSubgroup = optSubgroup.value()
	m.SystemOrder = optWithin.value()
	m.GroupHeaders = optHeaders.value()
	m.GroupView = optGroupView.value()
	m.ArcadeFirst = optArcadeFirst.isOn()
	m.FolderPosition = optFolders.value()
	m.GameOrder = optGameOrder.value()
	m.IgnoreThe = optIgnoreThe.isOn()
	m.GroupDiscs = optGroupDiscs.isOn()
	m.HideExtensions = optShowExt.isOn()
}

// normaliseOrder tidies a comma separated list to "A, B, C".
func normaliseOrder(s string) string {
	var parts []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, ", ")
}

// azName reads older SAM.ini files, which said "Alphabetical" for A-Z.
func azName(v string) string {
	if strings.EqualFold(strings.TrimSpace(v), "Alphabetical") {
		return "A-Z"
	}
	return v
}

// fixSortOptions keeps the settings consistent: the second grouping can't
// repeat the first (or exist without it).
func fixSortOptions() {
	if optGroup.value() == "None" || optSubgroup.value() == optGroup.value() {
		optSubgroup.set("None")
	}
}

// groupOrder is the first level's order: categories always follow the
// category order; other groupings use First group order.
func groupOrder() string {
	if optGroup.value() == "Category" {
		return "Custom"
	}
	return optGroupOrder.value()
}

// systemSortOptions returns the current systems list sorting.
func systemSortOptions() games.SortOptions {
	return games.SortOptions{
		Group:         optGroup.value(),
		GroupOrder:    groupOrder(),
		CategoryOrder: strings.Split(optCategoryOrder.value(), ","),
		Subgroup:      optSubgroup.value(),
		SubgroupOrder: "A-Z", // the second level is always A-Z
		Within:        optWithin.value(),
		ArcadeFirst:   optArcadeFirst.isOn(),
	}
}

// sortSystems sorts the systems list with the current settings.
func sortSystems(ids []string) { games.SortSystemNamesWith(ids, systemSortOptions()) }

// gameOrder returns the current game list ordering.
func gameOrder() gamesdb.GameOrder {
	return gamesdb.GameOrder{Natural: optGameOrder.value() == "Natural", IgnoreThe: optIgnoreThe.isOn(), GroupDiscs: optGroupDiscs.isOn()}
}

// systemsSortOptions shows the systems list sorting settings, previewing
// the top of the sorted list.
func systemsSortOptions(stdscr *gc.Window, sysIds []string, cfg *config.Config) {
	runOptionsScreen(stdscr, cfg, optionsScreen{
		title: "Systems List Sorting",
		options: func() []*labelOption {
			opts := []*labelOption{&optGroup}
			switch optGroup.value() {
			case "Category":
				opts = append(opts, &optCategoryOrder, &optSubgroup, &optGroupView)
			case "Manufacturer":
				opts = append(opts, &optGroupOrder, &optSubgroup, &optGroupView)
			}
			// Headings are for the single list; folders don't need them.
			if optGroup.value() != "None" && optGroupView.value() == "List" {
				opts = append(opts, &optHeaders)
			}
			return append(opts, &optWithin, &optArcadeFirst)
		},
		changed: func(o *labelOption) {
			// Then group by skips whatever the first level already uses.
			if o == &optSubgroup && o.value() != "None" && o.value() == optGroup.value() {
				o.next()
			}
			fixSortOptions()
		},
		preview: func() []string {
			names := previewSystems(sysIds)
			sortSystems(names)
			_, items := topEntries(names)
			return items
		},
	})
}

// gameSortOptions shows the game list sorting settings, previewing a
// made-up folder so each setting's effect is easy to see.
func gameSortOptions(stdscr *gc.Window, cfg *config.Config) {
	sample := &gamesdb.Node{Children: map[string]*gamesdb.Node{"Hacks": {}, "Translations": {}}}
	for _, name := range []string{"Game 10", "Game 2", "The Legend of Zelda", "Aladdin", "Metroid",
		"Final Fantasy VII (Disc 1)", "Final Fantasy VII (Disc 2)"} {
		sample.Files = append(sample.Files, gamesdb.FileInfo{Name: name, Ext: "nes"})
	}
	runOptionsScreen(stdscr, cfg, optionsScreen{
		title: "Game List Sorting",
		options: func() []*labelOption {
			return []*labelOption{&optFolders, &optGameOrder, &optIgnoreThe, &optShowExt, &optGroupDiscs}
		},
		preview: func() []string {
			var lines []string
			for _, e := range sample.Entries(gameOrder(), optFolders.value()) {
				if e.File != nil {
					lines = append(lines, gameName(e.File.Name, e.File.Ext))
				} else {
					lines = append(lines, e.Folder+"/")
				}
			}
			return lines
		},
	})
}

// previewTop returns the first n lines, with "..." if there are more.
func previewTop(lines []string, n int) []string {
	if len(lines) <= n {
		return lines
	}
	return append(lines[:n:n], "...")
}

// previewSample is a mix of systems that shows every sorting setting at
// work: categories, manufacturers, and systems whose release order
// differs from A-Z (NES, Gameboy, SNES; Commodore 64, Amiga).
var previewSample = []string{"Arcade", "Atari2600", "AtariLynx", "NES", "Gameboy", "SNES",
	"MasterSystem", "GameGear", "C64", "Amiga"}

// previewSystems picks the preview sample from the systems you have,
// always with the Arcade Cores. With too few of them it falls back to
// your first systems.
func previewSystems(have []string) []string {
	owned := map[string]bool{}
	for _, n := range have {
		owned[n] = true
	}
	var out []string
	for _, id := range previewSample {
		s, err := games.GetSystem(id)
		if err == nil && (owned[s.Name] || id == "Arcade") {
			out = append(out, s.Name)
		}
	}
	if len(out) < 5 {
		for _, n := range have {
			if len(out) >= 10 {
				break
			}
			if !contains(out, n) {
				out = append(out, n)
			}
		}
	}
	return out
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
