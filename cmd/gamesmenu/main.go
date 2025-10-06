package main

import (
	"encoding/gob"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"

	gc "github.com/rthornton128/goncurses"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/curses"
	"github.com/synrais/SAM-GO/pkg/games"
	"github.com/synrais/SAM-GO/pkg/gamesdb"
	"github.com/synrais/SAM-GO/pkg/mister"
)

// -------------------------
// Aliases
// -------------------------

type MenuFile = gamesdb.FileInfo

// -------------------------
// Load Gob Index
// -------------------------

func loadMenuDb() ([]MenuFile, error) {
	f, err := os.Open(config.MenuDb)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var files []MenuFile
	if err := gob.NewDecoder(f).Decode(&files); err != nil {
		return nil, err
	}
	return files, nil
}

// -------------------------
// Index Generation
// -------------------------

func generateIndexWindow(cfg *config.UserConfig, stdscr *gc.Window) ([]MenuFile, error) {
	_ = os.Remove(config.MenuDb)

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
	clearText := func() { win.MovePrint(1, 2, strings.Repeat(" ", width-4)) }

	status := struct {
		Step, Total int
		DisplayText string
		Complete    bool
		Error       error
	}{}

	go func() {
		_, err = gamesdb.NewNamesIndex(cfg, games.AllSystems(), func(is gamesdb.IndexStatus) {
			sysName := is.SystemId
			if sys, err := games.GetSystem(is.SystemId); err == nil {
				sysName = sys.Name
			}
			text := fmt.Sprintf("Indexing %s... (%d files)", sysName, is.Files)
			status.Step, status.Total, status.DisplayText = is.Step, is.Total, text
		})
		status.Error, status.Complete = err, true
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
	stdscr.Clear()
	stdscr.Refresh()

	if button == 0 && selected == 0 {
		return generateIndexWindow(cfg, stdscr)
	}
	return nil, nil
}

// -------------------------
// Tree Navigation
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

func browseNode(cfg *config.UserConfig, stdscr *gc.Window, node *Node, startIndex int) (int, error) {
	currentIndex := startIndex
	for {
		stdscr.Clear()
		stdscr.Refresh()

		var items []string
		var folders []string
		for name := range node.Children {
			folders = append(folders, name)
		}
		sort.Strings(folders)
		items = append(items, folders...)
		for _, f := range node.Files {
			displayName := f.Name
			if f.Ext != "" {
				displayName = fmt.Sprintf("%s.%s", f.Name, f.Ext)
			}
			items = append(items, displayName)
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
			InitialIndex:  currentIndex,
			DynamicActionLabel: func(idx int) string {
				if idx < len(folders) {
					return "Open"
				}
				return "Launch"
			},
		}, items)
		if err != nil {
			return currentIndex, err
		}

		currentIndex = selected
		switch button {
		case 2: // Open / Launch
			stdscr.Clear()
			stdscr.Refresh()
			if selected < len(folders) {
				childName := folders[selected]
				childIdx, err := browseNode(cfg, stdscr, node.Children[childName], 0)
				if err != nil {
					return currentIndex, err
				}
				currentIndex = childIdx
			} else {
				file := node.Files[selected-len(folders)]
				sys, _ := games.GetSystem(file.SystemId)
				_ = mister.LaunchGame(cfg, *sys, file.Path)
				stdscr.Clear()
				stdscr.Refresh()
			}
		case 3:
			stdscr.Clear()
			stdscr.Refresh()
			return currentIndex, nil
		}
	}
}

// -------------------------
// Main Menu
// -------------------------

func mainMenu(cfg *config.UserConfig, stdscr *gc.Window, files []MenuFile) error {
	tree := buildTree(files)
	var sysIds []string
	for id := range tree.Children {
		sysIds = append(sysIds, id)
	}
	sort.Strings(sysIds)

	makeList := func() []string {
		items := make([]string, len(sysIds))
		copy(items, sysIds)
		return items
	}

	startIndex := 0
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
			InitialIndex:  startIndex,
			DynamicActionLabel: func(_ int) string { return "Open" },
		}, makeList())
		if err != nil {
			return err
		}

		startIndex = selected
		stdscr.Clear()
		stdscr.Refresh()

		switch button {
		case 2: // Open
			sysId := sysIds[selected]
			_, err := browseNode(cfg, stdscr, tree.Children[sysId], 0)
			if err != nil {
				return err
			}
			stdscr.Clear()
			stdscr.Refresh()

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
				files = newFiles
				tree = buildTree(files)
				sysIds = sysIds[:0]
				for id := range tree.Children {
					sysIds = append(sysIds, id)
				}
				sort.Strings(sysIds)
			}
			stdscr.Clear()
			stdscr.Refresh()

		case 5:
			stdscr.Clear()
			stdscr.Refresh()
			return nil
		}
	}
}

// -------------------------
// Search Window (uses cached gamesdb search)
// -------------------------

func searchWindow(cfg *config.UserConfig, stdscr *gc.Window) error {
	stdscr.Clear()
	stdscr.Refresh()

	text := ""
	startIndex := 0
	for {
		button, query, err := curses.OnScreenKeyboard(stdscr, "Search", []string{"Search", "Back"}, text, 0)
		if err != nil || button == 1 {
			stdscr.Clear()
			stdscr.Refresh()
			return nil
		}
		text = query
		_ = curses.InfoBox(stdscr, "", "Searching...", false, false)

		status := struct {
			Done   bool
			Error  error
			Result []gamesdb.SearchResult
		}{}

		go func() {
			results, err := gamesdb.SearchNamesWords(games.AllSystems(), query)
			status.Result, status.Error, status.Done = results, err, true
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

		stdscr.Clear()
		stdscr.Refresh()

		if status.Error != nil {
			return status.Error
		}

		results := status.Result
		if len(results) == 0 {
			_ = curses.InfoBox(stdscr, "", "No results found.", false, true)
			stdscr.Clear()
			stdscr.Refresh()
			continue
		}

		var items []string
		for _, r := range results {
			systemName := r.SystemId
			if sys, err := games.GetSystem(r.SystemId); err == nil {
				systemName = sys.Name
			}
			display := r.Name
			if r.Ext != "" {
				display = fmt.Sprintf("%s.%s", r.Name, r.Ext)
			}
			items = append(items, fmt.Sprintf("[%s] %s", systemName, display))
		}

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
				InitialIndex:  startIndex,
			}, items)
			if err != nil {
				return err
			}
			startIndex = selected
			if button == 2 {
				game := results[selected]
				sys, _ := games.GetSystem(game.SystemId)
				_ = mister.LaunchGame(cfg, *sys, game.Path)
				stdscr.Clear()
				stdscr.Refresh()
			} else if button == 3 {
				stdscr.Clear()
				stdscr.Refresh()
				break
			}
		}
	}
}

// -------------------------
// Loading Spinner
// -------------------------

func loadingWindow(stdscr *gc.Window, loadFn func() ([]MenuFile, error)) ([]MenuFile, error) {
	status := struct {
		Done   bool
		Error  error
		Result []MenuFile
	}{}

	go func() {
		files, err := loadFn()
		status.Result, status.Error, status.Done = files, err, true
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

	stdscr.Clear()
	stdscr.Refresh()
	if status.Error != nil {
		return nil, status.Error
	}
	return status.Result, nil
}

// -------------------------
// Main Entry
// -------------------------

func main() {
	lockFile := "/tmp/gamesmenu.lock"
	f, err := os.OpenFile(lockFile, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		log.Fatalf("failed to open lock file: %v", err)
	}
	defer f.Close()

	tryLock := func() error { return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) }
	if err := tryLock(); err != nil {
		buf := make([]byte, 32)
		n, _ := f.ReadAt(buf, 0)
		if n > 0 {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(buf[:n]))); err == nil {
				_ = syscall.Kill(pid, syscall.SIGKILL)
				gc.Nap(500)
			}
		}
		if err := tryLock(); err != nil {
			log.Fatal("failed to acquire lock even after killing old process")
		}
	}

	_ = f.Truncate(0)
	_, _ = f.Seek(0, 0)
	_, _ = f.WriteString(fmt.Sprintf("%d", os.Getpid()))
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

	printPtr := flag.Bool("print", false, "Print game path instead of launching")
	flag.Parse()
	launchGame := !*printPtr

	cfg, err := config.LoadUserConfig("gamesmenu", &config.UserConfig{})
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
			displayName := f.Name
			if f.Ext != "" {
				displayName = fmt.Sprintf("%s.%s", f.Name, f.Ext)
			}
			fmt.Println(filepath.Join(f.MenuPath, displayName))
		}
	}
}
