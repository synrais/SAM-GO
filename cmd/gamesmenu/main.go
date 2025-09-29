package main

import (
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

const appName = "gamesmenu"

// -------------------------
// Database loading
// -------------------------
func loadMenuDb() ([]gamesdb.GobEntry, gamesdb.GobIndex, error) {
	idx, err := gamesdb.LoadGobIndex(config.MenuDb)
	if err != nil {
		return nil, nil, err
	}
	// flatten map into slice
	var files []gamesdb.GobEntry
	for _, entries := range idx {
		files = append(files, entries...)
	}
	return files, idx, nil
}

// -------------------------
// Index regeneration
// -------------------------
func generateIndexWindow(cfg *config.UserConfig, stdscr *gc.Window) ([]gamesdb.GobEntry, gamesdb.GobIndex, error) {
	_ = os.Remove(config.MenuDb)

	stdscr.Clear()
	stdscr.Refresh()

	// Create a bordered subwindow for progress
	win, err := curses.NewWindow(stdscr, 4, 75, "", -1)
	if err != nil {
		return nil, nil, err
	}
	defer win.Delete()

	_, width := win.MaxYX()

	// Progress update structure
	type progress struct {
		system string
		done   int
		total  int
	}
	updates := make(chan progress, 1)

	status := struct {
		Complete bool
		Error    error
		Idx      gamesdb.GobIndex
	}{}

	// Worker goroutine: build index and push updates
	go func() {
		idx, err := gamesdb.BuildGobIndex(cfg, games.AllSystems(),
			func(system string, done, total int) {
				select {
				case updates <- progress{system, done, total}:
				default:
				}
			})
		if err == nil {
			err = gamesdb.SaveGobIndex(idx, config.MenuDb)
		}
		status.Idx = idx
		status.Error = err
		status.Complete = true
		close(updates)
	}()

	spinnerSeq := []string{"|", "/", "-", "\\"}
	spinnerCount := 0
	var lastProgress *progress

	for {
		if status.Complete {
			break
		}

		// Non-blocking receive
		select {
		case p, ok := <-updates:
			if ok {
				lastProgress = &p
			}
		default:
		}

		// Clear only the text row (line 1 inside border)
		win.MovePrint(1, 2, strings.Repeat(" ", width-4))

		if lastProgress != nil {
			// system name left, totals right
			left := fmt.Sprintf("Indexing %s...", lastProgress.system)
			right := fmt.Sprintf("(%d/%d)", lastProgress.done, lastProgress.total)

			win.MovePrint(1, 2, left)
			win.MovePrint(1, width-len(right)-2, right)

			// Progress bar (line 2 inside border)
			progressWidth := width - 4
			filled := int(float64(lastProgress.done) / float64(lastProgress.total) * float64(progressWidth))
			for i := 0; i < filled; i++ {
				win.MoveAddChar(2, 2+i, gc.ACS_BLOCK)
			}
		} else {
			win.MovePrint(1, 2, "Indexing games...")
		}

		// Spinner in top-right corner
		spinnerCount = (spinnerCount + 1) % len(spinnerSeq)
		win.MovePrint(1, width-3, spinnerSeq[spinnerCount])

		win.NoutRefresh()
		_ = gc.Update()
		gc.Nap(100)
	}

	stdscr.Clear()
	stdscr.Refresh()

	if status.Error != nil {
		return nil, nil, status.Error
	}

	// Flatten map into slice
	var files []gamesdb.GobEntry
	for _, entries := range status.Idx {
		files = append(files, entries...)
	}

	// keep your outline, but still load via loader
	return loadingWindow(stdscr, loadMenuDb)
}

// -------------------------
// Tree navigation
// -------------------------
type Node struct {
	Name     string
	Files    []gamesdb.GobEntry
	Children map[string]*Node
}

func buildTree(files []gamesdb.GobEntry) *Node {
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
		case 2: // Open/Launch
			if selected < len(folders) {
				folderName := folders[selected]
				childIdx, err := browseNode(cfg, stdscr, node.Children[folderName], 0)
				if err != nil {
					return currentIndex, err
				}
				currentIndex = selected
				_ = childIdx
			} else {
				file := node.Files[selected-len(folders)]
				sys, _ := games.GetSystem(file.SystemId)
				_ = mister.LaunchGame(cfg, *sys, file.Path)
			}
		case 3: // Back
			return currentIndex, nil
		}
	}
}

// -------------------------
// Main menu
// -------------------------
func mainMenu(cfg *config.UserConfig, stdscr *gc.Window, files []gamesdb.GobEntry, idx gamesdb.GobIndex) error {
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

	startIndex := 0
	for {
		stdscr.Clear()
		stdscr.Refresh()

		button, selected, err := curses.ListPicker(stdscr, curses.ListPickerOpts{
			Title:         "Systems",
			Buttons:       []string{"PgUp", "PgDn", "", "Search", "Rebuild", "Exit"},
			ActionButton:  2,
			DefaultButton: 2,
			ShowTotal:     true,
			Width:         70,
			Height:        20,
			InitialIndex:  startIndex,
			DynamicActionLabel: func(idx int) string {
				return "Open"
			},
		}, items)
		if err != nil {
			return err
		}

		startIndex = selected
		switch button {
		case 2: // Open system
			sysId := sysIds[selected]
			_, err := browseNode(cfg, stdscr, tree.Children[sysId], 0)
			if err != nil {
				return err
			}
		case 3: // Search
			if err := searchWindow(cfg, stdscr, idx); err != nil {
				return err
			}
			stdscr.Clear()
			stdscr.Refresh()
		case 4: // Rebuild DB
			newFiles, newIdx, err := generateIndexWindow(cfg, stdscr)
			if err != nil {
				return err
			}
			files = newFiles
			idx = newIdx
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
		case 5: // Exit
			return nil
		}
	}
}

// -------------------------
// Search window
// -------------------------
func searchWindow(cfg *config.UserConfig, stdscr *gc.Window, idx gamesdb.GobIndex) error {
	stdscr.Clear()
	stdscr.Refresh()

	text := ""
	startIndex := 0

	for {
		button, query, err := curses.OnScreenKeyboard(stdscr, "Search", []string{"Search", "Back"}, text, 0)
		if err != nil || button == 1 {
			return nil
		}
		text = query

		results := idx.SearchWords(query)
		if len(results) == 0 {
			_ = curses.InfoBox(stdscr, "", "No results found.", false, true)
			continue
		}

		// Build display items directly from SearchName
		var items []string
		for _, r := range results {
			items = append(items, r.SearchName)
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

			if button == 2 { // Launch
				game := results[selected]
				sys, _ := games.GetSystem(game.SystemId)
				_ = mister.LaunchGame(cfg, *sys, game.Path)
				continue
			}
			if button == 3 { // Back
				stdscr.Clear()
				stdscr.Refresh()
				break
			}
		}
	}
}

// -------------------------
// Loader
// -------------------------
func loadingWindow(stdscr *gc.Window, loadFn func() ([]gamesdb.GobEntry, gamesdb.GobIndex, error)) ([]gamesdb.GobEntry, gamesdb.GobIndex, error) {
	status := struct {
		Done   bool
		Error  error
		Files  []gamesdb.GobEntry
		Idx    gamesdb.GobIndex
	}{}

	go func() {
		files, idx, err := loadFn()
		status.Files = files
		status.Idx = idx
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
		return nil, nil, status.Error
	}
	return status.Files, status.Idx, nil
}

// -------------------------
// Main
// -------------------------
func main() {
	// --- Lockfile to prevent multiple instances ---
	lockFile := "/tmp/gamesmenu.lock"
	f, err := os.OpenFile(lockFile, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		log.Fatalf("failed to open lock file: %v", err)
	}
	defer f.Close()

	tryLock := func() error {
		return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	}

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

	// --- Normal startup ---
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

	files, idx, err := loadingWindow(stdscr, loadMenuDb)
	if err != nil {
		files, idx, err = generateIndexWindow(cfg, stdscr)
		if err != nil {
			log.Fatal(err)
		}
	}

	if launchGame {
		if err := mainMenu(cfg, stdscr, files, idx); err != nil {
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
