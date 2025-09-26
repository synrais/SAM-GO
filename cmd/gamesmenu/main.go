package main

import (
	"encoding/gob"
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
	"github.com/synrais/SAM-GO/pkg/utils"
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

		// -------------------------
		// Case 1: Inside a .zip
		// -------------------------
		if idx := strings.Index(rel, ".zip"+string(filepath.Separator)); idx != -1 {
			inside := rel[idx+len(".zip"+string(filepath.Separator)):]
			parts = strings.Split(inside, string(filepath.Separator))

		} else {
			// -------------------------
			// Case 2: Regular file → anchor to system.Folder
			// -------------------------
			system, err := games.GetSystem(sysId)
			if err == nil {
				for _, sysFolder := range system.Folder {
					marker := sysFolder + string(filepath.Separator)
					if idx := strings.Index(strings.ToLower(rel), strings.ToLower(marker)); idx != -1 {
						inside := rel[idx+len(marker):]
						parts = strings.Split(inside, string(filepath.Separator))
						break
					}
				}
			}

			// -------------------------
			// Case 3: Fallback → just the filename
			// -------------------------
			if len(parts) == 0 {
				parts = []string{filepath.Base(rel)}
			}
		}

		// -------------------------
		// Build tree nodes
		// -------------------------
		current := sysNode
		for i, part := range parts {
			if part == "" {
				continue
			}

			if i == len(parts)-1 {
				res := result
				current.Children[part] = &Node{
					Name:     part,
					IsFolder: false,
					Game:     &res,
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
// Shared DB Indexer
// -------------------------
func generateIndexWindow(cfg *config.UserConfig, stdscr *gc.Window) (map[string]*Node, error) {
	stdscr.Erase()
	stdscr.NoutRefresh()
	_ = gc.Update()

	win, err := curses.NewWindow(stdscr, 4, 75, "", -1)
	if err != nil {
		return nil, err
	}
	defer win.Delete()

	_, width := win.MaxYX()

	// Progress bar helper
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

	// Clear line helper
	clearText := func() {
		win.MovePrint(1, 2, strings.Repeat(" ", width-4))
	}

	// Status struct for spinner
	status := struct {
		Step        int
		Total       int
		SystemName  string
		DisplayText string
		Complete    bool
		Error       error
		Tree        map[string]*Node
	}{Step: 1, Total: 100, DisplayText: "Finding games folders..."}

	// Background worker: force rebuild games.db + menu.db
	go func() {
		// 🔥 Clear old cached tree immediately (so no stale copy lingers)
		cachedTree = nil

		// -------------------------
		// Phase 1: Build games.db from scratch
		// -------------------------
		_, err = gamesdb.NewNamesIndex(cfg, games.AllSystems(), func(is gamesdb.IndexStatus) {
			systemName := is.SystemId
			if system, serr := games.GetSystem(is.SystemId); serr == nil {
				systemName = system.Name
			}

			text := fmt.Sprintf("Indexing %s...", systemName)
			if is.Step == 1 {
				text = "Finding games folders..."
			} else if is.Step == is.Total {
				text = "Writing games database..."
			}

			status.Step = is.Step
			status.Total = is.Total
			status.SystemName = systemName
			status.DisplayText = text
		})

		if err == nil {
			// -------------------------
			// Phase 2: Load fresh results from games.db
			// -------------------------
			status.DisplayText = "Loading game list..."
			status.Step = status.Total
			gc.Nap(200) // let UI refresh

			results, rerr := gamesdb.SearchNamesWords(games.AllSystems(), "")
			if rerr == nil {
				// -------------------------
				// Phase 3: Build new menu tree
				// -------------------------
				status.DisplayText = "Building menu tree..."
				gc.Nap(200)

				tree := buildTree(results)
				status.Tree = tree

				// -------------------------
				// Phase 4: Write menu.db
				// -------------------------
				status.DisplayText = "Writing menu database..."
				gc.Nap(200)

				menuPath := filepath.Join(filepath.Dir(config.GamesDb), "menu.db")
				if f, ferr := os.Create(menuPath); ferr == nil {
					defer f.Close()
					_ = gob.NewEncoder(f).Encode(tree)
				}

				// 🔥 Replace with fresh in-memory copy
				cachedTree = tree

				// Warm up games.db too
				_, _ = gamesdb.SearchNamesWords(games.AllSystems(), "")
			} else {
				status.Error = rerr
			}
		} else {
			status.Error = err
		}

		status.Complete = true
	}()

	// -------------------------
	// Spinner loop
	// -------------------------
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

	// -------------------------
	// Cleanup + return
	// -------------------------
	stdscr.Erase()
	stdscr.NoutRefresh()
	_ = gc.Update()

	return status.Tree, status.Error
}

// -------------------------
// Options
// -------------------------
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
		_, err := generateIndexWindow(cfg, stdscr)
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

		actionLabel := "Open"
		if len(order) > 0 && !order[0].IsFolder {
			actionLabel = "Launch"
		}

		button, selected, err := curses.ListPicker(stdscr, curses.ListPickerOpts{
			Title:         node.Name,
			Buttons:       []string{"PgUp", "PgDn", actionLabel, "Back"},
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
				_ = mister.LaunchGame(cfg, *system, choice.Game.Path)
				gc.End()
				os.Exit(0)
			}
		}
	}
}

// -------------------------
// Search Menu
// -------------------------
func searchWindow(cfg *config.UserConfig, stdscr *gc.Window, query string, launchGame bool, fromMenu bool) error {
	stdscr.Erase()
	stdscr.NoutRefresh()
	_ = gc.Update()

	searchTitle := "Search"
	searchButtons := []string{"Search"}
	if fromMenu {
		searchButtons = append(searchButtons, "Menu")
	} else {
		searchButtons = append(searchButtons, "Exit")
	}

	button, text, err := curses.OnScreenKeyboard(stdscr, searchTitle, searchButtons, query, 0)
	if err != nil {
		return err
	}

	if button == 0 { // Search
		if len(text) == 0 {
			return searchWindow(cfg, stdscr, "", launchGame, fromMenu)
		}

		if err := curses.InfoBox(stdscr, "", "Searching...", false, false); err != nil {
			return err
		}

		results, err := gamesdb.SearchNamesWords(games.AllSystems(), text)
		if err != nil {
			return err
		}

		if len(results) == 0 {
			if err := curses.InfoBox(stdscr, "", "No results found.", false, true); err != nil {
				return err
			}
			return searchWindow(cfg, stdscr, text, launchGame, fromMenu)
		}

		var names []string
		var items []gamesdb.SearchResult
		for _, result := range results {
			systemName := result.SystemId
			system, err := games.GetSystem(result.SystemId)
			if err == nil {
				systemName = system.Name
			}
			display := fmt.Sprintf("[%s] %s", systemName, result.Name)
			if !utils.Contains(names, display) {
				names = append(names, display)
				items = append(items, result)
			}
		}

		stdscr.Erase()
		stdscr.NoutRefresh()
		_ = gc.Update()

		var titleLabel, launchLabel string
		if launchGame {
			titleLabel = "Launch Game"
			launchLabel = "Launch"
		} else {
			titleLabel = "Pick Game"
			launchLabel = "Select"
		}

		button, selected, err := curses.ListPicker(stdscr, curses.ListPickerOpts{
			Title:         titleLabel,
			Buttons:       []string{"PgUp", "PgDn", launchLabel, "Cancel"},
			DefaultButton: 2,
			ActionButton:  2,
			ShowTotal:     true,
			Width:         70,
			Height:        18,
		}, names)
		if err != nil {
			return err
		}

		if button == 2 {
			game := items[selected]
			if launchGame {
				system, err := games.GetSystem(game.SystemId)
				if err != nil {
					return err
				}
				err = mister.LaunchGame(cfg, *system, game.Path)
				if err != nil {
					return err
				}
				gc.End()
				os.Exit(0)
			} else {
				gc.End()
				fmt.Fprintln(os.Stderr, game.Path)
				os.Exit(0)
			}
		}
		return searchWindow(cfg, stdscr, text, launchGame, fromMenu)
	} else {
		return nil
	}
}

// -------------------------
// System Menu
// -------------------------
func systemMenu(cfg *config.UserConfig, stdscr *gc.Window, systems map[string]*Node) error {
    var sysIds []string
    for sys := range systems {
        sysIds = append(sysIds, sys)
    }

    // Custom sort: ao486 at top, everything else alphabetical
    sort.Slice(sysIds, func(i, j int) bool {
        if strings.EqualFold(sysIds[i], "ao486") {
            return true
        }
        if strings.EqualFold(sysIds[j], "ao486") {
            return false
        }
        return strings.ToLower(sysIds[i]) < strings.ToLower(sysIds[j])
    })

    for {
        button, selected, err := curses.ListPicker(stdscr, curses.ListPickerOpts{
            Title:         "Systems",
            Buttons:       []string{
                "PgUp",
                "PgDn",
                "Open",
                "Search",
                "Options",
                "Exit",
            },
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
            _ = searchWindow(cfg, stdscr, "", true, true)
            continue
        }
        if button == 4 {
            _ = mainOptionsWindow(cfg, stdscr)
            continue
        }
        if button == 5 {
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
var cachedTree map[string]*Node // stays in RAM until program exit

func main() {
	printPtr := flag.Bool("print", false, "Print game path instead of launching")
	rebuildPtr := flag.Bool("rebuild", false, "Force rebuild of databases")
	flag.Parse()
	launchGame := !*printPtr
	forceRebuild := *rebuildPtr

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

	// -----------------------------
	// Use in-memory tree if we have one
	// -----------------------------
	if cachedTree == nil || forceRebuild {
		menuPath := filepath.Join(filepath.Dir(config.GamesDb), "menu.db")
		var tree map[string]*Node

		if !forceRebuild {
			// Try to load menu.db from disk
			f, ferr := os.Open(menuPath)
			if ferr == nil {
				defer f.Close()
				decErr := gob.NewDecoder(f).Decode(&tree)
				if decErr != nil {
					fmt.Println("Warning: could not decode menu.db, rebuilding...")
					tree = nil
				}
			}
		}

		// If tree missing or corrupt, rebuild both DBs
		if tree == nil {
			tree, err = generateIndexWindow(cfg, stdscr)
			if err != nil {
				log.Fatal(err)
			}
		}

		// Keep in RAM for reuse
		cachedTree = tree

		// Warm up games.db here
		_, _ = gamesdb.SearchNamesWords(games.AllSystems(), "")
	}

	// -----------------------------
	// Ready to go
	// -----------------------------
	if launchGame {
		err = systemMenu(cfg, stdscr, cachedTree)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		for sys, node := range cachedTree {
			fmt.Printf("System: %s (%d entries)\n", sys, len(node.Children))
		}
	}
}

