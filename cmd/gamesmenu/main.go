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
                // leaf node = actual game file
                res := result
                if _, ok := current.Children[part]; !ok {
                    current.Children[part] = &Node{
                        Name:     part,
                        IsFolder: false,
                        Game:     &res,
                    }
                }
            } else {
                // folder node
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

	// wipe old DB files before rebuilding
	_ = os.Remove(config.GamesDb)
	menuPath := filepath.Join(filepath.Dir(config.GamesDb), "menu.db")
	_ = os.Remove(menuPath)

	// reset in-memory tree so no stale copy lingers
	cachedTree = nil

	// simple stage updater
	updateStage := func(msg string) {
		clearText()
		win.MovePrint(1, 2, msg)
		win.NoutRefresh()
		_ = gc.Update()
	}

	// -------------------------
	// Phase 1: Build games.db from scratch
	// -------------------------
	_, err = gamesdb.NewNamesIndex(cfg, games.AllSystems(), func(is gamesdb.IndexStatus) {
		systemName := is.SystemId
		if system, serr := games.GetSystem(is.SystemId); serr == nil {
			systemName = system.Name
		}

		var text string
		switch {
		case is.Step == 1:
			text = "Finding games folders..."
		case is.Step == is.Total:
			text = "Finalizing games database..."
		default:
			text = fmt.Sprintf("Indexing %s...", systemName)
		}

		updateStage(text)
		drawProgressBar(is.Step, is.Total)
	})
	if err != nil {
		return nil, err
	}

	// -------------------------
	// Phase 2: Load fresh results
	// -------------------------
	updateStage("Loading game list...")
	gc.Nap(200)

	results, rerr := gamesdb.SearchNamesWords(games.AllSystems(), "")
	if rerr != nil {
		return nil, rerr
	}

	// -------------------------
	// Phase 3: Build new menu tree
	// -------------------------
	updateStage("Building menu tree...")
	gc.Nap(200)

	tree := buildTree(results)

	// -------------------------
	// Phase 4: Write menu.db
	// -------------------------
	updateStage("Writing menu database...")
	gc.Nap(200)

	if f, ferr := os.Create(menuPath); ferr == nil {
		defer f.Close()
		_ = gob.NewEncoder(f).Encode(tree)
	}

	// replace in-memory copy
	cachedTree = tree

	// warm up games.db
	_, _ = gamesdb.SearchNamesWords(games.AllSystems(), "")

	// fill progress bar to 100% before leaving
	drawProgressBar(1, 1)
	win.NoutRefresh()
	_ = gc.Update()

	// cleanup + return
	stdscr.Erase()
	stdscr.NoutRefresh()
	_ = gc.Update()

	return tree, nil
}

// -------------------------
// Options
// -------------------------
func mainOptionsWindow(cfg *config.UserConfig, stdscr *gc.Window) (map[string]*Node, error) {
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
        return nil, err
    }

    if button == 0 && selected == 0 {
        tree, err := generateIndexWindow(cfg, stdscr)
        if err != nil {
            return nil, err
        }
        cachedTree = tree
        return tree, nil
    }

    return nil, nil
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
        sort.Slice(folders, func(i, j int) bool {
            return strings.ToLower(folders[i].Name) < strings.ToLower(folders[j].Name)
        })
        sort.Slice(gamesList, func(i, j int) bool {
            return strings.ToLower(gamesList[i].Name) < strings.ToLower(gamesList[j].Name)
        })

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
            Buttons:       []string{"PgUp", "PgDn", "", "Back"}, // placeholder at index 2
            DefaultButton: 2,
            ActionButton:  2,
            ShowTotal:     true,
            Width:         70,
            Height:        20,
            DynamicActionLabel: func(idx int) string {
                if idx < 0 || idx >= len(order) {
                    return "Open"
                }
                if order[idx].IsFolder {
                    return "Open"
                }
                return "Launch"
            },
        }, items)
        if err != nil {
            return err
        }
        if button == 3 { // “Back”
            return nil
        }
        if button == 2 { // Action button pressed
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
        // after returning from a recursive call or launch, loop again to refresh
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

			// show real filename including extension
			filename := filepath.Base(result.Path)
			display := fmt.Sprintf("[%s] %s", systemName, filename)

			if !utils.Contains(names, display) {
				names = append(names, display)
				items = append(items, result)
			}
		}

		stdscr.Erase()
		stdscr.NoutRefresh()
		_ = gc.Update()

		var titleLabel string
		var actionLabel string
		if launchGame {
			titleLabel = "Launch Game"
			actionLabel = "Launch"
		} else {
			titleLabel = "Pick Game"
			actionLabel = "Select"
		}

		button, selected, err := curses.ListPicker(stdscr, curses.ListPickerOpts{
			Title:         titleLabel,
			Buttons:       []string{"PgUp", "PgDn", "", "Cancel"}, // placeholder at 2
			DefaultButton: 2,
			ActionButton:  2,
			ShowTotal:     true,
			Width:         70,
			Height:        18,
			DynamicActionLabel: func(idx int) string {
				return actionLabel
			},
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
    for {
        // rebuild sysIds fresh each time
        var sysIds []string
        for sys := range systems {
            sysIds = append(sysIds, sys)
        }

        sort.Slice(sysIds, func(i, j int) bool {
            if strings.EqualFold(sysIds[i], "ao486") {
                return true
            }
            if strings.EqualFold(sysIds[j], "ao486") {
                return false
            }
            return strings.ToLower(sysIds[i]) < strings.ToLower(sysIds[j])
        })

        button, selected, err := curses.ListPicker(stdscr, curses.ListPickerOpts{
            Title:         "Systems",
            Buttons:       []string{"PgUp","PgDn","", "Search","Options","Exit"}, // placeholder at 2
            DefaultButton: 2,
            ActionButton:  2,
            ShowTotal:     true,
            Width:         70,
            Height:        20,
            DynamicActionLabel: func(idx int) string {
                return "Open"
            },
        }, sysIds)
        if err != nil {
            return err
        }

        if button == 3 {
            _ = searchWindow(cfg, stdscr, "", true, true)
            continue
        }
        if button == 4 {
            // after rebuild, refresh systems reference
            if tree, err := mainOptionsWindow(cfg, stdscr); err == nil && tree != nil {
                systems = tree
            }
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

// -----------------------------
// Main
// -----------------------------
var cachedTree map[string]*Node // stays in RAM until program exit

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

    // -----------------------------
    // Load menu tree (or rebuild if missing/corrupt)
    // -----------------------------
    menuPath := filepath.Join(filepath.Dir(config.GamesDb), "menu.db")
    var tree map[string]*Node

    menuOk := false
    gamesOk := false

    // check menu.db
    if f, ferr := os.Open(menuPath); ferr == nil {
        defer f.Close()
        decErr := gob.NewDecoder(f).Decode(&tree)
        if decErr == nil {
            menuOk = true
        } else {
            tree = nil
        }
    }

    // check games.db
    if _, gerr := os.Stat(config.GamesDb); gerr == nil {
        gamesOk = true
    }

    // if either DB is missing/corrupt → rebuild both
    if !menuOk || !gamesOk {
        tree, err = generateIndexWindow(cfg, stdscr)
        if err != nil {
            log.Fatal(err)
        }
    }

    cachedTree = tree
    _, _ = gamesdb.SearchNamesWords(games.AllSystems(), "") // warm up

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


