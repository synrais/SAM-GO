package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"

	gc "github.com/rthornton128/goncurses"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/curses"
	"github.com/synrais/SAM-GO/pkg/games"
	"github.com/synrais/SAM-GO/pkg/gamesdb"
	"github.com/synrais/SAM-GO/pkg/mister"
)

const appName = "gamesmenu"

type menuNode struct {
	Name     string
	Path     string
	SystemId string
	IsFolder bool
	Children []*menuNode
}

func buildMenuTree(results []gamesdb.SearchResult) *menuNode {
	root := &menuNode{Name: "ROOT", IsFolder: true}
	folders := map[string]*menuNode{"": root}

	for _, r := range results {
		rel := r.Path
		parts := filepath.SplitList(rel)
		if len(parts) == 0 {
			parts = []string{r.Name}
		}

		// Build folder chain
		parentKey := ""
		parent := root
		for _, p := range parts[:len(parts)-1] {
			key := filepath.Join(parentKey, p)
			if _, ok := folders[key]; !ok {
				node := &menuNode{Name: p, IsFolder: true}
				parent.Children = append(parent.Children, node)
				folders[key] = node
			}
			parent = folders[key]
			parentKey = key
		}

		// Add game leaf
		gameNode := &menuNode{
			Name:     r.Name,
			Path:     r.Path,
			SystemId: r.SystemId,
			IsFolder: false,
		}
		parent.Children = append(parent.Children, gameNode)
	}

	return root
}

func listSystems(cfg *config.UserConfig, stdscr *gc.Window) error {
	results, err := gamesdb.ListAll()
	if err != nil {
		return err
	}

	tree := buildMenuTree(results)

	// Build system list
	var systems []string
	sysMap := make(map[string][]gamesdb.SearchResult)
	for _, r := range results {
		system, _ := games.GetSystem(r.SystemId)
		name := system.Name
		sysMap[name] = append(sysMap[name], r)
	}
	for s := range sysMap {
		systems = append(systems, s)
	}
	sort.Strings(systems)

	for {
		// Pick system
		btn, idx, err := curses.ListPicker(stdscr, curses.ListPickerOpts{
			Title:         "Select System",
			Buttons:       []string{"Open", "Exit"},
			DefaultButton: 0,
			ActionButton:  0,
			ShowTotal:     true,
			Width:         70,
			Height:        18,
		}, systems)
		if err != nil {
			return err
		}
		if btn != 0 {
			return nil
		}

		systemName := systems[idx]
		items := sysMap[systemName]

		// Pick game
		var names []string
		for _, r := range items {
			names = append(names, r.Name)
		}
		sort.Strings(names)

		btn, idx, err = curses.ListPicker(stdscr, curses.ListPickerOpts{
			Title:         fmt.Sprintf("[%s] Pick Game", systemName),
			Buttons:       []string{"Launch", "Back"},
			DefaultButton: 0,
			ActionButton:  0,
			ShowTotal:     true,
			Width:         70,
			Height:        18,
		}, names)
		if err != nil {
			return err
		}
		if btn != 0 {
			continue
		}

		// Launch
		var game gamesdb.SearchResult
		for _, r := range items {
			if r.Name == names[idx] {
				game = r
				break
			}
		}
		system, err := games.GetSystem(game.SystemId)
		if err != nil {
			log.Fatal(err)
		}
		if err := mister.LaunchGame(cfg, *system, game.Path); err != nil {
			log.Fatal(err)
		}
	}
}

func main() {
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
		fmt.Println("Game database not found. Run search to index first.")
		os.Exit(1)
	}

	if err := listSystems(cfg, stdscr); err != nil {
		log.Fatal(err)
	}
}
