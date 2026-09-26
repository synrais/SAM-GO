package main

import (
	"fmt"
	"strings"

	gc "github.com/rthornton128/goncurses"

	"github.com/synrais/SAMenu/pkg/config"
	"github.com/synrais/SAMenu/pkg/curses"
	"github.com/synrais/SAMenu/pkg/games"
	"github.com/synrais/SAMenu/pkg/gamesdb"
)

// -------------------------
// Systems Screen
// -------------------------
//
// The top screen lists the systems, either as one list (with optional
// group headings) or, with "Show groups as: Folders", as one folder per
// first-level group. The Arcade Cores stay on the top screen when pinned.

var optGroupView = labelOption{"Show groups as", []string{"List", "Folders"}, 0}

// entry is one line of the systems screen.
type entry struct {
	system string   // a system's display name
	group  string   // a group folder
	header bool     // a heading line
	names  []string // a folder's systems, in order
	direct bool     // a group folder that opens straight into its one system
	random bool     // [Pick Random Game]
}

func (e entry) key() string {
	if e.random {
		return "r:"
	}
	if e.group != "" {
		return "g:" + e.group
	}
	return "s:" + e.system
}

// groupTitle is how a group is named on screen ("Consoles").
func groupTitle(group string) string {
	if optGroup.value() == "Category" {
		if t, ok := categoryTitles[group]; ok {
			return t
		}
	}
	return group
}

// topEntries builds the systems screen from the sorted system names.
func topEntries(names []string) ([]entry, []string) {
	width := systemListWidth - 4
	if optGroupView.value() != "Folders" || optGroup.value() == "None" {
		items, headers, index := systemListItems(names, width)
		entries := make([]entry, len(items))
		for i := range items {
			if headers[i] {
				entries[i] = entry{header: true}
			} else {
				entries[i] = entry{system: names[index[i]]}
			}
		}
		return entries, items
	}

	var entries []entry
	var systems []string // pinned systems, shown as themselves
	folders := map[string]int{}
	for _, n := range names {
		if optArcadeFirst.isOn() && isArcade(n) {
			entries = append(entries, entry{system: n})
			systems = append(systems, n)
			continue
		}
		g := games.SystemGroup(n, optGroup.value())
		if i, ok := folders[g]; ok {
			entries[i].names = append(entries[i].names, n)
			continue
		}
		folders[g] = len(entries)
		entries = append(entries, entry{group: g, names: []string{n}})
	}

	// A group folder holding only the Arcade Cores opens straight into the
	// arcade games (no extra click), and counts its MRAs instead of "(1)".
	for i, e := range entries {
		if e.group != "" && len(e.names) == 1 && isArcade(e.names[0]) {
			entries[i].direct = true
			entries[i].system = e.names[0]
		}
	}

	labels := systemLabels(systems, width)
	items := make([]string, len(entries))
	si := 0
	for i, e := range entries {
		if e.group == "" {
			items[i] = labels[si]
			si++
			continue
		}
		count := len(e.names)
		if e.direct {
			count = gameCount(e.system)
		}
		text := fmt.Sprintf("%s  (%d)", groupTitle(e.group), count)
		items[i] = strings.Repeat(" ", screenOffset(len(text), width)) + text
	}
	return entries, items
}

func isArcade(name string) bool {
	s, err := games.GetSystem("Arcade")
	return err == nil && s.Name == name
}

// systemsScreen shows a list of systems (or group folders), until Exit on
// the top screen or Back in a folder. It reports errGameLaunched and other
// errors, and whether the games database was rebuilt (so the caller can
// start over with the new systems).
func systemsScreen(cfg *config.Config, stdscr *gc.Window, st *menuState, title string, names []string, top bool) error {
	current := ""
	depth := 0
	if !top {
		depth = 1
	}
	first := true
	for {
		if top {
			names = st.sysIds
		}
		entries, items := topEntries(names)
		if !top {
			// Inside a folder: just its systems, no headings.
			items = systemLabels(names, systemListWidth-4)
			entries = make([]entry, len(names))
			for i, n := range names {
				entries[i] = entry{system: n}
			}
		}
		// [Pick Random Game] on top: from all the systems listed here.
		if cfg.Menu.RandomEntry {
			entries = append([]entry{{random: true}}, entries...)
			items = append([]string{randomLabel}, items...)
		}
		// Walking back to a remembered position (see position.go).
		autoOpen := -1
		if first {
			first = false
			keyAt := func(i int) string {
				if entries[i].header {
					return ""
				}
				return entries[i].key()
			}
			if idx, ok, found, open := restoreAt(depth, keyAt, len(entries)); ok {
				for idx < len(entries)-1 && entries[idx].header {
					idx++
				}
				if idx < len(entries) && !entries[idx].header {
					current = entries[idx].key()
				}
				if found && open {
					autoOpen = idx
				} else {
					doneRestoring(depth)
				}
			}
		}
		headers := map[int]bool{}
		initial := 0
		for i, e := range entries {
			if e.header {
				headers[i] = true
			} else if (current == "" && !e.random) || e.key() == current {
				if current == "" {
					current = e.key()
				}
				initial = i
			}
		}

		buttons := []string{"PgUp", "PgDn", "", "Search", "Options", "Exit"}
		if !top {
			buttons = []string{"PgUp", "PgDn", "", "Back"}
		}
		clearScreen(stdscr)
		button, selected, err := 2, autoOpen, error(nil)
		if autoOpen < 0 {
			button, selected, err = curses.ListPicker(stdscr, curses.ListPickerOpts{
				Shortcuts:     menuShortcuts(),
				Title:         title,
				Buttons:       buttons,
				ActionButton:  2,
				DefaultButton: 2,
				ShowTotal:     true,
				Width:         systemListWidth,
				Height:        listHeight,
				InitialIndex:  initial,
				Headers:       headers,
				DynamicActionLabel: func(i int) string {
					if i >= 0 && i < len(entries) && entries[i].random {
						return "Pick"
					}
					return "Open"
				},
			}, items)
		}
		if err != nil {
			return err
		}
		if selected >= 0 && selected < len(entries) && !entries[selected].header {
			current = entries[selected].key()
			navHere(depth, current, selected)
		}
		clearScreen(stdscr)

		if !top {
			if button == 3 {
				return nil // Back
			}
		}
		switch button {
		case 2:
			if selected < 0 || selected >= len(entries) {
				continue
			}
			e := entries[selected]
			if e.random {
				if err := pickRandomGame(stdscr, cfg, systemsFiles(st, names)); err != nil {
					savePosition()
					return err
				}
				continue
			}
			if e.group != "" && !e.direct {
				if err := systemsScreen(cfg, stdscr, st, groupTitle(e.group), e.names, false); err != nil {
					return err
				}
				continue
			}
			if _, err := browseNode(cfg, stdscr, st.tree.Children[e.system], depth+1); err != nil {
				return err
			}
		case 3:
			if top {
				if err := searchWindow(cfg, stdscr); err != nil {
					return err
				}
			}
		case 4:
			if top {
				newFiles, err := optionsMenu(cfg, stdscr, st.files, st.sysIds)
				if err != nil {
					return err
				}
				if newFiles != nil {
					st.load(newFiles)
				} else if treeDirty {
					st.load(st.files) // A-Z folders changed
				}
				treeDirty = false
				sortSystems(st.sysIds) // settings may have changed
			}
		case 5:
			if top {
				savePosition()
				return nil // Exit
			}
		}
	}
}

// gameCount counts a system's games (set when the games database loads),
// for folders that show games instead of systems: the Arcade folder.
var gameCount = func(system string) int { return 0 }

// uniqueGames counts the games in a folder and all its subfolders, counting
// the same file name (e.g. an MRA on two drives) once.
func uniqueGames(node *gamesdb.Node) int {
	if node == nil {
		return 0
	}
	seen := map[string]bool{}
	var walk func(n *gamesdb.Node)
	walk = func(n *gamesdb.Node) {
		for _, f := range n.Files {
			seen[strings.ToLower(f.Name+"."+f.Ext)] = true
		}
		for _, child := range n.Children {
			walk(child)
		}
	}
	walk(node)
	return len(seen)
}

// menuState is the games database as the menu shows it.
type menuState struct {
	files  []MenuFile
	tree   *gamesdb.Node
	sysIds []string // system display names, sorted
}

func (st *menuState) load(files []MenuFile) {
	st.files = files
	st.tree = buildTree(menuHide.Without(files)) // minus [Menu] HideTags
	currentTree = st.tree
	tree := st.tree
	gameCount = func(system string) int { return uniqueGames(tree.Children[system]) }
	st.sysIds = st.sysIds[:0]
	for name := range st.tree.Children {
		st.sysIds = append(st.sysIds, name)
	}
	sortSystems(st.sysIds)
}
