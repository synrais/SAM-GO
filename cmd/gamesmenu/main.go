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

func buildTree(files []gamesdb.FileInfo) map[string]*Node {
    systems := make(map[string]*Node)

    for _, f := range files {
        sysId := f.SystemId
        sysNode, ok := systems[sysId]
        if !ok {
            sysNode = &Node{
                Name:     sysId,
                IsFolder: true,
                Children: make(map[string]*Node),
            }
            systems[sysId] = sysNode
        }

        parts := strings.Split(f.Path, string(filepath.Separator))

        // Find SystemId in path
        var relParts []string
        for i, p := range parts {
            if strings.EqualFold(p, sysId) {
                relParts = parts[i:]
                break
            }
        }
        if len(relParts) == 0 {
            relParts = []string{filepath.Base(f.Path)}
        }

        // ✅ Collapse or skip .zip node
        if len(relParts) > 1 && strings.HasSuffix(strings.ToLower(relParts[1]), ".zip") {
            zipName := strings.TrimSuffix(relParts[1], ".zip")
            if strings.EqualFold(zipName, sysId) {
                // Case: SystemId.zip -> drop both sysId and .zip
                relParts = relParts[2:]
            } else {
                // Case: Other.zip -> drop .zip but keep sysId
                relParts = append(relParts[:1], relParts[2:]...)
            }
        } else {
            // Drop the sysId itself, root node already exists
            if len(relParts) > 0 && strings.EqualFold(relParts[0], sysId) {
                relParts = relParts[1:]
            }
        }

        // ✅ Handle `.txt` folders (convert to folder, drop "listings" before it)
        for i := 0; i < len(relParts)-1; i++ {
            part := relParts[i]
            if strings.HasSuffix(strings.ToLower(part), ".txt") {
                txtName := strings.TrimSuffix(part, ".txt")
                relParts[i] = txtName // keep as folder

                // Drop "listings" if it’s right before
                if i > 0 && strings.EqualFold(relParts[i-1], "listings") {
                    relParts = append(relParts[:i-1], relParts[i:]...)
                    i-- // adjust index
                }
            }
        }

        // Walk tree
        current := sysNode
        for i, part := range relParts {
            if part == "" {
                continue
            }
            if i == len(relParts)-1 {
                // leaf = game
                current.Children[part] = &Node{
                    Name:     part,
                    IsFolder: false,
                    Game: &gamesdb.SearchResult{
                        SystemId: f.SystemId,
                        Name:     filepath.Base(f.Path),
                        Path:     f.Path,
                    },
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

// Flatten tree into []FileInfo for gamesdb.UpdateNames
func collectFiles(tree map[string]*Node) []gamesdb.FileInfo {
	var out []gamesdb.FileInfo
	var walk func(sysId string, node *Node)
	walk = func(sysId string, node *Node) {
		if !node.IsFolder && node.Game != nil {
			out = append(out, gamesdb.FileInfo{
				SystemId: sysId,
				Path:     node.Game.Path,
			})
		}
		for _, child := range node.Children {
			walk(sysId, child)
		}
	}
	for sysId, root := range tree {
		for _, child := range root.Children {
			walk(sysId, child)
		}
	}
	return out
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
		progressWidth := width - 4
		progressPct := int(float64(current) / float64(total) * float64(progressWidth))
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
	}{}

	go func() {
		// 🔹 Step 0: Ensure fresh DBs
		menuPath := filepath.Join(filepath.Dir(config.GamesDb), "menu.db")
		_ = os.Remove(menuPath)
		_ = os.Remove(config.GamesDb)

		// 🔹 Step 1: Scan games + write incrementally to Bolt
		files, err := gamesdb.NewNamesIndex(cfg, games.AllSystems(), func(is gamesdb.IndexStatus) {
			systemName := is.SystemId
			if sys, serr := games.GetSystem(is.SystemId); serr == nil {
				systemName = sys.Name
			}

			text := fmt.Sprintf("Indexing %s... (%d files)", systemName, is.Files)
			if is.Step == 1 {
				text = "Finding games folders..."
			}
			status.Step = is.Step
			status.Total = is.Total
			status.SystemName = systemName
			status.DisplayText = text
		})

		if err != nil {
			status.Error = err
			status.Complete = true
			return
		}

		// 🔹 Step 2: Build menu.db
		status.DisplayText = "Building menu.db..."
		tree := buildTree(files)

		if f, ferr := os.Create(menuPath); ferr == nil {
			_ = gob.NewEncoder(f).Encode(tree)
			f.Close()
		}

		// 🔹 Step 3: Explicit Bolt flush
		status.DisplayText = "Finalizing games.db..."
		db, dberr := gamesdb.OpenForWrite()
		if dberr != nil {
			status.Error = dberr
			status.Complete = true
			return
		}
		_ = db.Sync()
		db.Close()

		cachedTree = tree
		status.DisplayText = "Rebuild complete."
		status.Complete = true
	}()

	// Spinner animation
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
		if status.Total > 0 {
			drawProgressBar(status.Step, status.Total)
		}

		win.NoutRefresh()
		_ = gc.Update()
		gc.Nap(100)
	}

	// Clear the indexing window once complete
	win.Erase()
	win.NoutRefresh()
	_ = gc.Update()

	if status.Error != nil {
		return nil, status.Error
	}
	return cachedTree, nil
}

// -------------------------
// Options
// -------------------------
func mainOptionsWindow(cfg *config.UserConfig, stdscr *gc.Window) (map[string]*Node, error) {
    // ✅ Clear the main screen first so system menu doesn't show behind
    stdscr.Erase()
    stdscr.NoutRefresh()
    _ = gc.Update()

    button, selected, err := curses.ListPicker(stdscr, curses.ListPickerOpts{
        Title:         "Options",
        Buttons:       []string{"Select", "Back"},
        DefaultButton: 0,
        ActionButton:  0,
        ShowTotal:     false,
        Width:         70,  // match system menu
        Height:        20,  // match system menu
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
			Buttons:       []string{"PgUp", "PgDn", "", "Back"},
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

	if button == 0 {
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
			Buttons:       []string{"PgUp", "PgDn", "", "Cancel"},
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
		// Build slice of system IDs (map keys) for stable processing
		var sysIds []string
		for sys := range systems {
			sysIds = append(sysIds, sys)
		}

		// Sort by friendly name, fallback to ID
		sort.SliceStable(sysIds, func(i, j int) bool {
			var nameI, nameJ string
			if s, err := games.GetSystem(sysIds[i]); err == nil {
				nameI = s.Name
			} else {
				nameI = sysIds[i]
			}
			if s, err := games.GetSystem(sysIds[j]); err == nil {
				nameJ = s.Name
			} else {
				nameJ = sysIds[j]
			}
			return strings.ToLower(nameI) < strings.ToLower(nameJ)
		})

		// Build display list matching sysIds order
		var sysDisplay []string
		for _, sysId := range sysIds {
			if s, err := games.GetSystem(sysId); err == nil {
				sysDisplay = append(sysDisplay, s.Name)
			} else {
				sysDisplay = append(sysDisplay, sysId)
			}
		}

		// Draw menu
		button, selected, err := curses.ListPicker(stdscr, curses.ListPickerOpts{
			Title:         "Systems",
			Buttons:       []string{"PgUp", "PgDn", "", "Search", "Options", "Exit"},
			DefaultButton: 2,
			ActionButton:  2,
			ShowTotal:     true,
			Width:         70,
			Height:        20,
			DynamicActionLabel: func(idx int) string {
				return "Open"
			},
		}, sysDisplay)
		if err != nil {
			return err
		}

		// Handle buttons
		if button == 3 { // Search
			_ = searchWindow(cfg, stdscr, "", true, true)
			continue
		}
		if button == 4 { // Options
			if tree, err := mainOptionsWindow(cfg, stdscr); err == nil && tree != nil {
				systems = tree
			}
			continue
		}
		if button == 5 { // Exit
			return nil
		}
		if button == 2 { // Open
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
var cachedTree map[string]*Node

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

	menuPath := filepath.Join(filepath.Dir(config.GamesDb), "menu.db")
	var tree map[string]*Node

	menuOk := false
	gamesOk := false

	if f, ferr := os.Open(menuPath); ferr == nil {
		defer f.Close()
		decErr := gob.NewDecoder(f).Decode(&tree)
		if decErr == nil {
			menuOk = true
		} else {
			tree = nil
		}
	}

	if _, gerr := os.Stat(config.GamesDb); gerr == nil {
		gamesOk = true
	}

	if !menuOk || !gamesOk {
		tree, err = generateIndexWindow(cfg, stdscr)
		if err != nil {
			log.Fatal(err)
		}
	}

	cachedTree = tree

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
