package main

import (
	"encoding/gob"
	"flag"
	"fmt"
	"log"
	"os"
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
// Local struct for menu.db
// -------------------------
type MenuFile struct {
	SystemId     string
	SystemName   string
	SystemFolder string
	Name         string
	NameExt      string
	Path         string
	FolderName   string
	MenuPath     string
}

// -------------------------
// Load Gob from menu.db
// -------------------------
func loadMenuDb() ([]MenuFile, error) {
	f, err := os.Open(config.MenuDb)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var files []MenuFile
	dec := gob.NewDecoder(f)
	if err := dec.Decode(&files); err != nil {
		return nil, err
	}
	return files, nil
}

// -------------------------
// Shared DB Indexer
// -------------------------
func generateIndexWindow(cfg *config.UserConfig, stdscr *gc.Window) ([]MenuFile, error) {
	// 🔹 delete old DBs before rebuilding
	_ = os.Remove(config.MenuDb)
	_ = os.Remove(config.GamesDb)

	stdscr.Clear()
	stdscr.Refresh()

	win, err := curses.NewWindow(stdscr, 4, 75, "", -1)
	if err != nil {
		return nil, err
	}
	defer win.Delete()

	_, width := win.MaxYX()

	drawProgressBar := func(current, total int) {
		if total == 0 {
			return
		}
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
		DisplayText string
		Complete    bool
		Error       error
	}{}

	go func() {
		_, err = gamesdb.NewNamesIndex(cfg, games.AllSystems(), func(is gamesdb.IndexStatus) {
			systemName := is.SystemId
			if sys, err := games.GetSystem(is.SystemId); err == nil {
				systemName = sys.Name
			}
			text := fmt.Sprintf("Indexing %s... (%d files)", systemName, is.Files)
			if is.Step == 1 {
				text = "Finding games folders..."
			}
			status.Step = is.Step
			status.Total = is.Total
			status.DisplayText = text
		})
		status.Error = err
		status.Complete = true
	}()

	spinnerSeq := []string{"|", "/", "-", "\\"}
	spinnerCount := 0

	for {
		if status.Complete {
			break
		}
		clearText()
		spinnerCount = (spinnerCount + 1) % len(spinnerSeq)
		win.MovePrint(1, width-3, spinnerSeq[spinnerCount])
		win.MovePrint(1, 2, status.DisplayText)
		drawProgressBar(status.Step, status.Total)
		win.NoutRefresh()
		_ = gc.Update()
		gc.Nap(100)
	}

	stdscr.Clear()
	stdscr.Refresh()

	if status.Error != nil {
		return nil, status.Error
	}

	return loadingWindow(stdscr, loadMenuDb)
}

// -------------------------
// Options Menu
// -------------------------
func optionsMenu(cfg *config.UserConfig, stdscr *gc.Window) ([]MenuFile, error) {
	stdscr.Clear()
	stdscr.Refresh()

	button, selected, err := curses.ListPicker(stdscr, curses.ListPickerOpts{
		Title:         "Options",
		Buttons:       []string{"Select", "Back"},
		DefaultButton: 0,
		ActionButton:  0,
		ShowTotal:     false,
		Width:         60,
		Height:        10,
	}, []string{"Rebuild games database..."})
	if err != nil {
		return nil, err
	}

	if button == 0 && selected == 0 {
		return generateIndexWindow(cfg, stdscr)
	}
	return nil, nil
}

// -------------------------
// Tree structure
// -------------------------
type Node struct {
	Name     string
	Files    []MenuFile
	Children map[string]*Node
}

func buildTree(files []MenuFile) *Node {
	root := &Node{Name: "", Children: make(map[string]*Node)}
	for _, f := range files {
		parts := strings.Split(f.MenuPath, string(os.PathSeparator))
		curr := root
		for i, part := range parts {
			if i == len(parts)-1 {
				curr.Files = append(curr.Files, f)
			} else {
				if curr.Children[part] == nil {
					curr.Children[part] = &Node{Name: part, Children: make(map[string]*Node)}
				}
				curr = curr.Children[part]
			}
		}
	}
	return root
}

// -------------------------
// Browse inside folders
// -------------------------
func browseNode(cfg *config.UserConfig, stdscr *gc.Window, node *Node) error {
	for {
		stdscr.Clear()
		stdscr.Refresh()

		var items []string
		var folders []string
		for name := range node.Children {
			folders = append(folders, name+"/")
		}
		sort.Strings(folders)
		items = append(items, folders...)

		for _, f := range node.Files {
			items = append(items, f.NameExt)
		}

		title := node.Name
		if title == "" {
			title = "Games"
		}

		buttons := []string{"PgUp", "PgDn", "", "Back"}

		button, selected, err := curses.ListPicker(stdscr, curses.ListPickerOpts{
			Title:         title,
			Buttons:       buttons,
			ActionButton:  2,
			DefaultButton: 2,
			ShowTotal:     true,
			Width:         70,
			Height:        20,
			DynamicActionLabel: func(idx int) string {
				if idx < 0 || idx >= len(items) {
					return "Open"
				}
				if idx < len(folders) {
					return "Open"
				}
				return "Launch"
			},
		}, items)
		if err != nil {
			return err
		}

		switch button {
		case 2: // Open/Launch
			if selected < len(folders) {
				folderName := folders[selected][:len(folders[selected])-1]
				if err := browseNode(cfg, stdscr, node.Children[folderName]); err != nil {
					return err
				}
			} else {
				file := node.Files[selected-len(folders)]
				sys, _ := games.GetSystem(file.SystemId)
				if err := mister.LaunchGame(cfg, *sys, file.Path); err != nil {
					return err
				}
				// stay in same folder after launch
			}
		case 3: // Back
			return nil
		}
	}
}

// -------------------------
// Main Menu (systems)
// -------------------------
func mainMenu(cfg *config.UserConfig, stdscr *gc.Window, files []MenuFile) error {
	tree := buildTree(files)

	var items []string
	var sysIds []string
	for sysId := range tree.Children {
		sysIds = append(sysIds, sysId)
	}
	sort.Strings(sysIds)
	for _, sysId := range sysIds {
		items = append(items, sysId)
	}

	for {
		stdscr.Clear()
		stdscr.Refresh()

		button, selected, err := curses.ListPicker(stdscr, curses.ListPickerOpts{
			Title:         "Systems",
			Buttons:       []string{"PgUp", "PgDn", "", "Search", "Options", "Exit"},
			ActionButton:  2,
			DefaultButton: 2,
			ShowTotal:     true,
			Width:         70,
			Height:        20,
			DynamicActionLabel: func(idx int) string {
				return "Open"
			},
		}, items)
		if err != nil {
			return err
		}

		switch button {
		case 2: // Open system
			sysId := sysIds[selected]
			if err := browseNode(cfg, stdscr, tree.Children[sysId]); err != nil {
				return err
			}

		case 3: // Search
			if err := searchWindow(cfg, stdscr); err != nil {
				return err
			}
			stdscr.Clear()
			stdscr.Refresh()

		case 4: // Options
			if newFiles, err := optionsMenu(cfg, stdscr); err != nil {
				return err
			} else if newFiles != nil {
				files := newFiles

				// Rebuild tree and menu lists
				tree = buildTree(files)
				sysIds = sysIds[:0]
				items = items[:0]
				for sysId := range tree.Children {
					sysIds = append(sysIds, sysId)
				}
				sort.Strings(sysIds)
				for _, sysId := range sysIds {
					items = append(items, sysId)
				}

				stdscr.Clear()
				stdscr.Refresh()
			}

		case 5: // Exit
			return nil
		}
	}
}

// -------------------------
// Search Menu
// -------------------------
func searchWindow(cfg *config.UserConfig, stdscr *gc.Window) error {
	stdscr.Clear()
	stdscr.Refresh()

	text := ""
	for {
		// Keep keyboard open, prefill with last text
		button, query, err := curses.OnScreenKeyboard(stdscr, "Search", []string{"Search", "Back"}, text, 0)
		if err != nil || button == 1 {
			return nil
		}
		text = query

		// Show searching spinner
		_ = curses.InfoBox(stdscr, "", "Searching...", false, false)

		status := struct {
			Done   bool
			Error  error
			Result []gamesdb.SearchResult
		}{}

		go func() {
			results, err := gamesdb.SearchNamesWords(games.AllSystems(), query)
			status.Result = results
			status.Error = err
			status.Done = true
		}()

		spinnerSeq := []string{"|", "/", "-", "\\"}
		spinnerCount := 0

		for {
			if status.Done {
				break
			}
			label := fmt.Sprintf("Searching... %s", spinnerSeq[spinnerCount])
			_ = curses.InfoBox(stdscr, "", label, false, false)
			spinnerCount = (spinnerCount + 1) % len(spinnerSeq)
			_ = gc.Update()
			gc.Nap(100)
		}

		if status.Error != nil {
			return status.Error
		}

		results := status.Result
		if len(results) == 0 {
			// Just layer "No results" on top of keyboard until keypress
			_ = curses.InfoBox(stdscr, "", "No results found.", false, true)
			continue
		}

		// Build results list
		var items []string
		for _, r := range results {
			systemName := r.SystemId
			if sys, err := games.GetSystem(r.SystemId); err == nil {
				systemName = sys.Name
			}
			items = append(items, fmt.Sprintf("[%s] %s", systemName, r.Name))
		}

		// loop inside results picker so launching doesn’t exit search
		for {
			stdscr.Clear()
			stdscr.Refresh()

			button, selected, err := curses.ListPicker(stdscr, curses.ListPickerOpts{
				Title:         "Search Results",
				Buttons:       []string{"PgUp", "PgDn", "Launch", "Back"},
				ActionButton:  2,
				DefaultButton: 2,
				ShowTotal:     true,
				Width:         70,
				Height:        20,
			}, items)
			if err != nil {
				return err
			}
			if button == 2 {
				game := results[selected]
				sys, _ := games.GetSystem(game.SystemId)
				if err := mister.LaunchGame(cfg, *sys, game.Path); err != nil {
					return err
				}
				// stay in results after launch
				continue
			}
			if button == 3 { // Back → return to keyboard without resetting query
				stdscr.Clear()
				stdscr.Refresh()
				break
			}
		}
	}
}

// -------------------------
// Startup Loading Spinner
// -------------------------
func loadingWindow(stdscr *gc.Window, loadFn func() ([]MenuFile, error)) ([]MenuFile, error) {
	status := struct {
		Done   bool
		Error  error
		Result []MenuFile
	}{}

	go func() {
		files, err := loadFn()
		status.Result = files
		status.Error = err
		status.Done = true
	}()

	spinnerSeq := []string{"|", "/", "-", "\\"}
	spinnerCount := 0

	for {
		if status.Done {
			break
		}
		label := fmt.Sprintf("Loading... %s", spinnerSeq[spinnerCount])
		_ = curses.InfoBox(stdscr, "", label, false, false)
		spinnerCount = (spinnerCount + 1) % len(spinnerSeq)
		_ = gc.Update()
		gc.Nap(100)
	}

	if status.Error != nil {
		return nil, status.Error
	}
	return status.Result, nil
}


// -------------------------
// Main
// -------------------------
func main() {
	printPtr := flag.Bool("print", false, "Print game path instead of launching")
	flag.Parse()
	launchGame := !*printPtr

	cfg, err := config.LoadUserConfig(appName, &config.UserConfig{})
	if err != nil && !os.IsNotExist(err) {
		log.Fatal(err)
	}

	stdscr, err := curses.Setup()
	if err != nil {
		log.Fatal(err)
	}
	defer gc.End()

	files, err := loadingWindow(stdscr, loadMenuDb)
	if err != nil {
		files, err = generateIndexWindow(cfg, stdscr)
		if err != nil {
			log.Fatal(err)
		}
	}

	if launchGame {
		if err := mainMenu(cfg, stdscr, files); err != nil {
			log.Fatal(err)
		}
	} else {
		for _, f := range files {
			fmt.Println(f.MenuPath)
		}
	}
}
