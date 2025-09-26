package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	gc "github.com/rthornton128/goncurses"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/curses"
	"github.com/synrais/SAM-GO/pkg/games"
	"github.com/synrais/SAM-GO/pkg/gamesdb"
	"github.com/synrais/SAM-GO/pkg/mister"
)

const appName = "gamesmenu"

// builds a map[systemID] -> map[dir] -> []games
func buildTree(results []gamesdb.SearchResult) map[string]map[string][]gamesdb.SearchResult {
	tree := make(map[string]map[string][]gamesdb.SearchResult)

	for _, result := range results {
		system := result.SystemId
		dir := filepath.Dir(result.Path)

		if _, ok := tree[system]; !ok {
			tree[system] = make(map[string][]gamesdb.SearchResult)
		}
		tree[system][dir] = append(tree[system][dir], result)
	}

	return tree
}

func systemMenu(cfg *config.UserConfig, stdscr *gc.Window, tree map[string]map[string][]gamesdb.SearchResult) error {
	// Build system list
	var systems []string
	for sys := range tree {
		if system, err := games.GetSystem(sys); err == nil {
			systems = append(systems, system.Name)
		} else {
			systems = append(systems, sys)
		}
	}

	for {
		button, selected, err := curses.ListPicker(stdscr, curses.ListPickerOpts{
			Title:         "Select System",
			Buttons:       []string{"PgUp", "PgDn", "Open", "Exit"},
			DefaultButton: 2,
			ActionButton:  2,
			ShowTotal:     true,
			Width:         70,
			Height:        20,
		}, systems)
		if err != nil {
			return err
		}
		if button == 3 { // Exit
			return nil
		}
		if button == 2 { // Open system
			sysId := games.AllSystems()[selected].Id
			err = folderMenu(cfg, stdscr, sysId, tree[sysId])
			if err != nil {
				return err
			}
		}
	}
}

func folderMenu(cfg *config.UserConfig, stdscr *gc.Window, sysId string, dirs map[string][]gamesdb.SearchResult) error {
	var folders []string
	for d := range dirs {
		folders = append(folders, d)
	}

	for {
		button, selected, err := curses.ListPicker(stdscr, curses.ListPickerOpts{
			Title:         fmt.Sprintf("Folders (%s)", sysId),
			Buttons:       []string{"PgUp", "PgDn", "Open", "Back"},
			DefaultButton: 2,
			ActionButton:  2,
			ShowTotal:     true,
			Width:         70,
			Height:        20,
		}, folders)
		if err != nil {
			return err
		}
		if button == 3 { // Back
			return nil
		}
		if button == 2 { // Open folder
			dir := folders[selected]
			err = gamesMenu(cfg, stdscr, sysId, dirs[dir])
			if err != nil {
				return err
			}
		}
	}
}

func gamesMenu(cfg *config.UserConfig, stdscr *gc.Window, sysId string, gamesList []gamesdb.SearchResult) error {
	var names []string
	for _, g := range gamesList {
		names = append(names, filepath.Base(g.Path))
	}

	button, selected, err := curses.ListPicker(stdscr, curses.ListPickerOpts{
		Title:         fmt.Sprintf("Games (%s)", sysId),
		Buttons:       []string{"PgUp", "PgDn", "Launch", "Back"},
		DefaultButton: 2,
		ActionButton:  2,
		ShowTotal:     true,
		Width:         70,
		Height:        20,
	}, names)
	if err != nil {
		return err
	}
	if button == 3 { // Back
		return nil
	}
	if button == 2 { // Launch game
		game := gamesList[selected]
		system, err := games.GetSystem(game.SystemId)
		if err != nil {
			return err
		}
		return mister.LaunchGame(cfg, *system, game.Path)
	}
	return nil
}

func main() {
	printPtr := flag.Bool("print", false, "Print game path instead of launching")
	flag.Parse()
	_ = !*printPtr // currently unused, could wire in if needed

	cfg, err := config.LoadUserConfig(appName, &config.UserConfig{})
	if err != nil && !os.IsNotExist(err) {
		fmt.Println("Error loading config file:", err)
		os.Exit(1)
	}

	stdscr, err := curses.Setup()
	if err != nil {
		log.Fatal(err)
	}
	defer gc.End()

	if !gamesdb.DbExists() {
		fmt.Println("Games DB missing, run search to index first.")
		os.Exit(1)
	}

	results, err := gamesdb.SearchNamesWords(games.AllSystems(), "")
	if err != nil {
		log.Fatal(err)
	}
	if len(results) == 0 {
		fmt.Println("No games found in DB.")
		os.Exit(1)
	}

	tree := buildTree(results)
	err = systemMenu(cfg, stdscr, tree)
	if err != nil {
		log.Fatal(err)
	}
}
