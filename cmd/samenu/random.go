package main

import (
	"fmt"
	"path/filepath"
	"strconv"

	gc "github.com/rthornton128/goncurses"

	"github.com/synrais/SAMenu/pkg/attract"
	"github.com/synrais/SAMenu/pkg/config"
	"github.com/synrais/SAMenu/pkg/curses"
	"github.com/synrais/SAMenu/pkg/games"
	"github.com/synrais/SAMenu/pkg/gamesdb"
	"github.com/synrais/SAMenu/pkg/mister"
)

// -------------------------
// Pick Random Game
// -------------------------
//
// [Pick Random Game] sits at the top of the systems list and of every
// folder ([Menu] RandomEntry, Options -> Display & Sorting -> Pick Random
// Game). It launches a random game from everything at that level and
// below, picked the way attract mode picks ([Attract] Selection, OneVersion
// and [Weights]; the in-turn modes use Balanced for a single pick).

const randomLabel = "[Pick Random Game]"

// randomRows is how many rows the entry adds to a list (0 or 1).
func randomRows(cfg *config.Config) int {
	if cfg.Menu.RandomEntry {
		return 1
	}
	return 0
}

// nodeFiles is every game in a folder and its subfolders, skipping the
// virtual [Games A-Z] folder (it only repeats them).
func nodeFiles(n *gamesdb.Node) []MenuFile {
	var files []MenuFile
	var walk func(x *gamesdb.Node)
	walk = func(x *gamesdb.Node) {
		if x == nil {
			return
		}
		files = append(files, x.Files...)
		for _, c := range x.Children {
			if !c.Pinned {
				walk(c)
			}
		}
	}
	walk(n)
	return files
}

// systemsFiles is every game of the named systems.
func systemsFiles(st *menuState, names []string) []MenuFile {
	var files []MenuFile
	for _, name := range names {
		files = append(files, nodeFiles(st.tree.Children[name])...)
	}
	return files
}

// pickRandomGame picks a game from files, says which, and launches it. It
// returns errGameLaunched when the game started.
func pickRandomGame(stdscr *gc.Window, cfg *config.Config, files []MenuFile) error {
	f, ok := attract.NewPicker(cfg, files, true).Next()
	if !ok {
		message(stdscr, "No games here to pick from.")
		return nil
	}
	sys, err := games.GetSystem(f.SystemId)
	if err != nil {
		message(stdscr, "Couldn't launch it: "+err.Error())
		return nil
	}
	name := f.FileName()
	text := fmt.Sprintf("%s  (%s)", name, sys.Name)
	if max := systemListWidth - 4; len(text) > max && max > 3 {
		text = text[:max-3] + "..."
	}
	_ = curses.InfoBox(stdscr, "Random game", text, true, false)
	gc.Nap(1200)
	if err := mister.LaunchGame(cfg, *sys, f.Path); err != nil {
		message(stdscr, fmt.Sprintf("Couldn't launch %s: %v", filepath.Base(f.Path), err))
		return nil
	}
	return errGameLaunched
}

// randomGameScreen is Options -> Display & Sorting -> Pick Random Game.
func randomGameScreen(stdscr *gc.Window, cfg *config.Config) {
	show := onOffOption("Show [Pick Random Game]", cfg.Menu.RandomEntry)
	runOptionsScreen(stdscr, cfg, optionsScreen{
		title:     "Pick Random Game",
		noPreview: true,
		options: func() []*labelOption {
			return []*labelOption{&show}
		},
		save: func() error {
			cfg.Menu.RandomEntry = show.isOn()
			return config.SaveValues(cfg.Path, "Menu", [][2]string{
				{"RandomEntry", strconv.FormatBool(cfg.Menu.RandomEntry)},
			})
		},
	})
}
