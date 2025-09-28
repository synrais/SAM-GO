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
			if sys, ok := games.GetSystem(is.SystemId); ok {
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

	if status.Error != nil {
		return nil, status.Error
	}

	// reload the Gob now that it’s rebuilt
	return loadMenuDb()
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
		// trigger rebuild
		return generateIndexWindow(cfg, stdscr)
	}
	return nil, nil
}

// -------------------------
// Main Menu (flat by MenuPath)
// -------------------------
func mainMenu(cfg *config.UserConfig, stdscr *gc.Window, files []MenuFile) error {
	for {
		stdscr.Clear()
		stdscr.Refresh()

		// sort all files by MenuPath
		sort.Slice(files, func(i, j int) bool {
			return strings.ToLower(files[i].MenuPath) < strings.ToLower(files[j].MenuPath)
		})

		var items []string
		for _, f := range files {
			items = append(items, f.MenuPath)
		}

		button, selected, err := curses.ListPicker(stdscr, curses.ListPickerOpts{
			Title:         "Games",
			Buttons:       []string{"PgUp", "PgDn", "Launch", "Search", "Options", "Exit"},
			ActionButton:  2,
			DefaultButton: 2,
			ShowTotal:     true,
			Width:         80,
			Height:        22,
		}, items)
		if err != nil {
			return err
		}

		switch button {
		case 2: // Launch
			game := files[selected]
			sys, _ := games.GetSystem(game.SystemId)
			return mister.LaunchGame(cfg, *sys, game.Path)
		case 3: // Search
			if err := searchWindow(cfg, stdscr); err != nil {
				return err
			}
		case 4: // Options
			if newFiles, err := optionsMenu(cfg, stdscr); err != nil {
				return err
			} else if newFiles != nil {
				files = newFiles
			}
		case 5: // Exit
			return nil
		}
	}
}

// -------------------------
// Search Menu (Bolt)
// -------------------------
func searchWindow(cfg *config.UserConfig, stdscr *gc.Window) error {
	stdscr.Clear()
	stdscr.Refresh()

	text := ""
	button, query, err := curses.OnScreenKeyboard(stdscr, "Search", []string{"Search", "Back"}, text, 0)
	if err != nil || button == 1 {
		return nil
	}

	results, err := gamesdb.SearchNamesWords(games.AllSystems(), query)
	if err != nil {
		return err
	}
	if len(results) == 0 {
		_ = curses.InfoBox(stdscr, "", "No results found.", false, true)
		return nil
	}

	var items []string
	for _, r := range results {
		systemName := r.SystemId
		if sys, ok := games.GetSystem(r.SystemId); ok {
			systemName = sys.Name
		}
		items = append(items, fmt.Sprintf("[%s] %s", systemName, r.Name))
	}

	button, selected, err := curses.ListPicker(stdscr, curses.ListPickerOpts{
		Title:         "Search Results",
		Buttons:       []string{"PgUp", "PgDn", "Launch", "Cancel"},
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
		return mister.LaunchGame(cfg, *sys, game.Path)
	}
	return nil
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

	// load Gob
	files, err := loadMenuDb()
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
