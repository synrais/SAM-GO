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

// appName for config load
const appName = "gamesmenu"

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

func showMenu(cfg *config.UserConfig, stdscr *gc.Window) error {
	stdscr.Erase()
	stdscr.NoutRefresh()
	_ = gc.Update()

	// pull ALL results from DB
	results, err := gamesdb.SearchNamesWords(games.AllSystems(), "")
	if err != nil {
		return err
	}
	if len(results) == 0 {
		return fmt.Errorf("no games found in DB, try updating database first")
	}

	tree := buildTree(results)

	var menuItems []string
	var flatList []gamesdb.SearchResult

	for sys, dirs := range tree {
		for dir, games := range dirs {
			menuItems = append(menuItems, fmt.Sprintf("[%s] %s/", sys, dir))
			for _, g := range games {
				menuItems = append(menuItems, fmt.Sprintf("   %s", g.Name))
				flatList = append(flatList, g)
			}
		}
	}

	button, selected, err := curses.ListPicker(stdscr, curses.ListPickerOpts{
		Title:         "Games Menu",
		Buttons:       []string{"PgUp", "PgDn", "Launch", "Cancel"},
		DefaultButton: 2,
		ActionButton:  2,
		ShowTotal:     true,
		Width:         70,
		Height:        20,
	}, menuItems)
	if err != nil {
		return err
	}

	if button == 2 { // Launch
		if selected < len(flatList) {
			game := flatList[selected]
			system, err := games.GetSystem(game.SystemId)
			if err != nil {
				return err
			}
			return mister.LaunchGame(cfg, *system, game.Path)
		}
	}

	return nil
}

func main() {
	printPtr := flag.Bool("print", false, "Print game path to stderr instead of launching")
	flag.Parse()
	launchGame := !*printPtr

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

	err = showMenu(cfg, stdscr)
	if err != nil {
		log.Fatal(err)
	}

	if !launchGame {
		fmt.Println("Exited without launching.")
	}
}
