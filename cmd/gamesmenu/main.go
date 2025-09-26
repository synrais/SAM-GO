package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	gc "github.com/rthornton128/goncurses"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/curses"
	"github.com/synrais/SAM-GO/pkg/games"
	"github.com/synrais/SAM-GO/pkg/gamesdb"
	"github.com/synrais/SAM-GO/pkg/mister"
)

// -------------------------
// Tree structure
// -------------------------
type Node struct {
	Name     string
	IsFolder bool
	Children map[string]*Node
	Game     *gamesdb.SearchResult
}

func buildTree(results []gamesdb.SearchResult) map[string]*Node {
	systems := make(map[string]*Node)

	for _, result := range results {
		sysId := result.SystemId
		sysNode, ok := systems[sysId]
		if !ok {
			sysNode = &Node{
				Name:     sysId,
				IsFolder: true,
				Children: make(map[string]*Node),
			}
			systems[sysId] = sysNode
		}

		rel := result.Path
		var parts []string

		if idx := strings.Index(rel, ".zip"+string(filepath.Separator)); idx != -1 {
			// inside .zip
			inside := rel[idx+len(".zip"+string(filepath.Separator)):]
			parts = strings.Split(inside, string(filepath.Separator))
		} else {
			// after system id
			idx := strings.Index(rel, sysId+string(filepath.Separator))
			if idx != -1 {
				inside := rel[idx+len(sysId+string(filepath.Separator)):]
				parts = strings.Split(inside, string(filepath.Separator))
			} else {
				// fallback = just file
				parts = []string{filepath.Base(rel)}
			}
		}

		current := sysNode
		for i, part := range parts {
			if part == "" {
				continue
			}

			if i == len(parts)-1 {
				// always insert a game node
				current.Children[part] = &Node{
					Name:     part,
					IsFolder: false,
					Game:     &result,
				}
			} else {
				// create or reuse folder node
				child, ok := current.Children[part]
				if !ok {
					child = &Node{
						Name:     part,
						IsFolder: true,
						Children: make(map[string]*Node),
					}
					current.Children[part] = child
				}
				current = child
			}
		}
	}

	return systems
}

// -------------------------
// Browsing
// -------------------------
func browseNode(cfg *config.UserConfig, stdscr *gc.Window, system *games.System, node *Node) error {
	for {
		var items []string
		var order []*Node

		// Sort children: folders first, then games
		var folders, gamesList []*Node
		for _, child := range node.Children {
			if child.IsFolder {
				folders = append(folders, child)
			} else {
				gamesList = append(gamesList, child)
			}
		}
		sort.Slice(folders, func(i, j int) bool { return strings.ToLower(folders[i].Name) < strings.ToLower(folders[j].Name) })
		sort.Slice(gamesList, func(i, j int) bool { return strings.ToLower(gamesList[i].Name) < strings.ToLower(gamesList[j].Name) })

		for _, f := range folders {
			items = append(items, "[DIR] "+f.Name)
			order = append(order, f)
		}
		for _, g := range gamesList {
			items = append(items, g.Name)
			order = append(order, g)
		}

		button, selected, err := curses.ListPicker(stdscr, curses.ListPickerOpts{
			Title:         node.Name,
			Buttons:       []string{"PgUp", "PgDn", "Open", "Back"},
			DefaultButton: 2,
			ActionButton:  2,
			ShowTotal:     true,
			Width:         70,
			Height:        20,
		}, items)
		if err != nil {
			return err
		}
		if button == 3 { // Back
			return nil
		}
		if button == 2 {
			choice := order[selected]
			if choice.IsFolder {
				if err := browseNode(cfg, stdscr, system, choice); err != nil {
					return err
				}
			} else {
				return mister.LaunchGame(cfg, *system, choice.Game.Path)
			}
		}
	}
}

func systemMenu(cfg *config.UserConfig, stdscr *gc.Window, systems map[string]*Node) error {
	var sysIds []string
	for sys := range systems {
		sysIds = append(sysIds, sys)
	}
	sort.Strings(sysIds)

	for {
		button, selected, err := curses.ListPicker(stdscr, curses.ListPickerOpts{
			Title:         "Systems",
			Buttons:       []string{"PgUp", "PgDn", "Open", "Exit"},
			DefaultButton: 2,
			ActionButton:  2,
			ShowTotal:     true,
			Width:         70,
			Height:        20,
		}, sysIds)
		if err != nil {
			return err
		}
		if button == 3 {
			return nil
		}
		if button == 2 {
			sysId := sysIds[selected]
			system, err := games.GetSystem(sysId)
			if err != nil {
				return err
			}
			root := systems[sysId]
			if err := browseNode(cfg, stdscr, system, root); err != nil {
				return err
			}
		}
	}
}

// -------------------------
// Main
// -------------------------
func main() {
	printPtr := flag.Bool("print", false, "Print game path instead of launching")
	flag.Parse()
	launchGame := !*printPtr

	cfg, err := config.LoadUserConfig("gamesmenu", &config.UserConfig{})
	if err != nil && !os.IsNotExist(err) {
		fmt.Println("Error loading config:", err)
		os.Exit(1)
	}

	stdscr, err := curses.Setup()
	if err != nil {
		log.Fatal(err)
	}
	defer gc.End()

	// Load DB contents
	results, err := gamesdb.SearchNamesWords(games.AllSystems(), "")
	if err != nil {
		log.Fatal(err)
	}

	tree := buildTree(results)

	if launchGame {
		err = systemMenu(cfg, stdscr, tree)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		// Debug print mode
		for sys, node := range tree {
			fmt.Printf("System: %s (%d entries)\n", sys, len(node.Children))
		}
	}
}
