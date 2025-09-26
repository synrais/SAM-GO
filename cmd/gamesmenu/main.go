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
// DB Indexer (same as search)
// -------------------------
func generateIndexWindow(cfg *config.UserConfig, stdscr *gc.Window) error {
	win, err := curses.NewWindow(stdscr, 4, 75, "", -1)
	if err != nil {
		return err
	}
	defer win.Delete()

	_, width := win.MaxYX()

	drawProgressBar := func(current int, total int) {
		pct := int(float64(current) / float64(total) * 100)
		progressWidth := width - 4
		progressPct := int(float64(pct) / float64(100) * float64(progressWidth))
		if progressPct < 1 {
			progressPct = 1
		}
		for i := 0; i < progressPct; i++ {
			win.MoveAddChar(2, 2+i, gc.ACS_BLOCK)
		}
		win.NoutRefresh()
	}

	clearText := func() {
		win.MovePrint(1, 2, strings.Repeat(" ", width-4))
	}

	status := struct {
		Step        int
		Total       int
		SystemName  string
		DisplayText string
		Complete    bool
		Error       error
	}{
		Step:        1,
		Total:       100,
		DisplayText: "Finding games folders...",
	}

	go func() {
		_, err = gamesdb.NewNamesIndex(cfg, games.AllSystems(), func(is gamesdb.IndexStatus) {
			systemName := is.SystemId
			system, err := games.GetSystem(is.SystemId)
			if err == nil {
				systemName = system.Name
			}

			text := fmt.Sprintf("Indexing %s...", systemName)
			if is.Step == 1 {
				text = "Finding games folders..."
			} else if is.Step == is.Total {
				text = "Writing database to disk..."
			}

			status.Step = is.Step
			status.Total = is.Total
			status.SystemName = systemName
			status.DisplayText = text
		})

		status.Error = err
		status.Complete = true
	}()

	spinnerSeq := []string{"|", "/", "-", "\\"}
	spinnerCount := 0

	for {
		if status.Complete || status.Error != nil {
			break
		}

		clearText()

		spinnerCount++
		if spinnerCount == len(spinnerSeq) {
			spinnerCount = 0
		}

		win.MovePrint(1, width-3, spinnerSeq[spinnerCount])

		win.MovePrint(1, 2, status.DisplayText)
		drawProgressBar(status.Step, status.Total)

		win.NoutRefresh()
		_ = gc.Update()
		gc.Nap(100)
	}

	return status.Error
}

func mainOptionsWindow(cfg *config.UserConfig, stdscr *gc.Window) error {
	button, selected, err := curses.ListPicker(stdscr, curses.ListPickerOpts{
		Title:         "Options",
		Buttons:       []string{"Select", "Back"},
		DefaultButton: 0,
		ActionButton:  0,
		ShowTotal:     false,
		Width:         70,
		Height:        18,
	}, []string{"Update games database..."})

	if err != nil {
		return err
	}

	if button == 0 && selected == 0 {
		err := generateIndexWindow(cfg, stdscr)
		if err != nil {
			return err
		}
	}
	return nil
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
			Buttons:       []string{"PgUp", "PgDn", "Open", "Options", "Exit"},
			DefaultButton: 2,
			ActionButton:  2,
			ShowTotal:     true,
			Width:         70,
			Height:        20,
		}, sysIds)
		if err != nil {
			return err
		}
		if button == 3 { // Options
			_ = mainOptionsWindow(cfg, stdscr)
			// rebuild DB + tree
			results, _ := gamesdb.SearchNamesWords(games.AllSystems(), "")
			systems = buildTree(results)
			continue
		}
		if button == 4 { // Exit
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

	// Initial DB build
	if !gamesdb.DbExists() {
		err := generateIndexWindow(cfg, stdscr)
		if err != nil {
			log.Fatal(err)
		}
	}

	// Load DB → Tree
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
		// Debug print
		for sys, node := range tree {
			fmt.Printf("System: %s (%d entries)\n", sys, len(node.Children))
		}
	}
}
