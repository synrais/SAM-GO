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

const appName = "gamesmenu"

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
			inside := rel[idx+len(".zip"+string(filepath.Separator)):]
			parts = strings.Split(inside, string(filepath.Separator))
		} else {
			idx := strings.Index(rel, sysId+string(filepath.Separator))
			if idx != -1 {
				inside := rel[idx+len(sysId+string(filepath.Separator)):]
				parts = strings.Split(inside, string(filepath.Separator))
			} else {
				parts = []string{filepath.Base(rel)}
			}
		}

		current := sysNode
		for i, part := range parts {
			if part == "" {
				continue
			}

			if i == len(parts)-1 {
				current.Children[part] = &Node{
					Name:     part,
					IsFolder: false,
					Game:     &result,
				}
			} else {
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
		if button == 3 {
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
	for {
		var sysIds []string
		for sys := range systems {
			sysIds = append(sysIds, sys)
		}
		sort.Strings(sysIds)

		button, selected, err := curses.ListPicker(stdscr, curses.ListPickerOpts{
			Title:         "Systems",
			Buttons:       []string{"Options", "PgUp", "PgDn", "Open", "Exit"},
			DefaultButton: 3,
			ActionButton:  3,
			ShowTotal:     true,
			Width:         70,
			Height:        20,
		}, sysIds)
		if err != nil {
			return err
		}
		switch button {
		case 4: // Exit
			return nil
		case 0: // Options -> rebuild DB
			if err := generateIndexWindow(cfg, stdscr); err != nil {
				return err
			}
			stdscr.Erase()
			stdscr.NoutRefresh()
			_ = gc.Update()

			results, err := gamesdb.SearchNamesWords(games.AllSystems(), "")
			if err != nil {
				return err
			}
			systems = buildTree(results)
		case 3: // Open
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
// DB progress (copied from search)
// -------------------------
func generateIndexWindow(cfg *config.UserConfig, stdscr *gc.Window) error {
	win, err := curses.NewWindow(stdscr, 4, 75, "", -1)
	if err != nil {
		return err
	}
	defer win.Delete()

	_, width := win.MaxYX()
	drawProgressBar := func(current, total int) {
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
	clearText := func() { win.MovePrint(1, 2, strings.Repeat(" ", width-4)) }

	status := struct {
		Step, Total int
		SystemName  string
		DisplayText string
		Complete    bool
		Error       error
	}{Step: 1, Total: 100, DisplayText: "Finding games folders..."}

	go func() {
		_, err = gamesdb.NewNamesIndex(cfg, games.AllSystems(), func(is gamesdb.IndexStatus) {
			systemName := is.SystemId
			if sys, err := games.GetSystem(is.SystemId); err == nil {
				systemName = sys.Name
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
	spin := 0
	for {
		if status.Complete || status.Error != nil {
			break
		}
		clearText()
		spin = (spin + 1) % len(spinnerSeq)
		win.MovePrint(1, width-3, spinnerSeq[spin])
		win.MovePrint(1, 2, status.DisplayText)
		drawProgressBar(status.Step, status.Total)
		win.NoutRefresh()
		_ = gc.Update()
		gc.Nap(100)
	}
	return status.Error
}

// -------------------------
// Main
// -------------------------
func main() {
	printPtr := flag.Bool("print", false, "Print game path instead of launching")
	flag.Parse()
	launchGame := !*printPtr

	cfg, err := config.LoadUserConfig(appName, &config.UserConfig{})
	if err != nil {
		fmt.Println("[WARN] Could not load config, using defaults:", err)
		cfg = &config.UserConfig{}
	}

	stdscr, err := curses.Setup()
	if err != nil {
		log.Fatal(err)
	}
	defer gc.End()

	if !gamesdb.DbExists() {
		if err := generateIndexWindow(cfg, stdscr); err != nil {
			log.Fatal(err)
		}
		stdscr.Erase()
		stdscr.NoutRefresh()
		_ = gc.Update()
	}

	results, err := gamesdb.SearchNamesWords(games.AllSystems(), "")
	if err != nil {
		log.Fatal(err)
	}
	tree := buildTree(results)

	if launchGame {
		if err := systemMenu(cfg, stdscr, tree); err != nil {
			log.Fatal(err)
		}
	} else {
		for sys, node := range tree {
			fmt.Printf("System: %s (%d entries)\n", sys, len(node.Children))
		}
	}
}
