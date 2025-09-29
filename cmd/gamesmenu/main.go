package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
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
func loadMenuDb() (gamesdb.GobIndex, error) {
	return gamesdb.LoadGobIndex(config.MenuDb)
}

// -------------------------
// Index regeneration
// -------------------------
func generateIndexWindow(cfg *config.UserConfig, stdscr *gc.Window) (gamesdb.GobIndex, error) {
	_ = os.Remove(config.MenuDb)

	stdscr.Clear()
	stdscr.Refresh()

	// Create a bordered subwindow for progress
	win, err := curses.NewWindow(stdscr, 4, 75, "", -1)
	if err != nil {
		return gamesdb.GobIndex{}, err
	}
	defer win.Delete()

	_, width := win.MaxYX()

	type progress struct {
		system string
		done   int
		total  int
	}

	updates := make(chan progress, len(games.AllSystems()))

	status := struct {
		Complete bool
		Error    error
		Idx      gamesdb.GobIndex
	}{}

	// Worker goroutine: build index and push updates
	go func() {
		idx, err := gamesdb.BuildGobIndex(cfg, games.AllSystems(),
			func(system string, done, total int) {
				updates <- progress{system, done, total}
			})
		if err == nil {
			err = gamesdb.SaveGobIndex(idx, config.MenuDb)
		}
		status.Error = err
		status.Complete = true
		status.Idx = idx
		close(updates)
	}()

	spinnerSeq := []string{"|", "/", "-", "\\"}
	spinnerCount := 0
	var lastProgress *progress

	for {
		if status.Complete {
			break
		}

		select {
		case p, ok := <-updates:
			if ok {
				lastProgress = &p
			}
		default:
		}

		win.MovePrint(1, 2, strings.Repeat(" ", width-6))

		if lastProgress != nil {
			left := fmt.Sprintf("Indexing %s...", lastProgress.system)
			right := fmt.Sprintf("(%3d/%-3d)", lastProgress.done, lastProgress.total)

			win.MovePrint(1, 2, left)
			win.MovePrint(1, width-len(right)-4, right)

			progressWidth := width - 4
			filled := int(float64(lastProgress.done) / float64(lastProgress.total) * float64(progressWidth))
			for i := 0; i < filled; i++ {
				win.MoveAddChar(2, 2+i, gc.ACS_BLOCK)
			}
		} else {
			win.MovePrint(1, 2, "Indexing games...")
		}

		spinnerCount = (spinnerCount + 1) % len(spinnerSeq)
		win.MovePrint(1, width-3, spinnerSeq[spinnerCount])

		win.NoutRefresh()
		_ = gc.Update()
		gc.Nap(100)
	}

	stdscr.Clear()
	stdscr.Refresh()

	if status.Error != nil {
		return gamesdb.GobIndex{}, status.Error
	}

	return status.Idx, nil
}

// -------------------------
// Tree navigation
// -------------------------
type Node struct {
	Name       string
	Files      []gamesdb.GobEntry
	Children   map[string]*Node
	ChildNames []string
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
					curr.ChildNames = append(curr.ChildNames, part)
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
		items = append(items, node.ChildNames...)

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
				if idx < len(node.ChildNames) {
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
		case 2:
			if selected < len(node.ChildNames) {
				folderName := node.ChildNames[selected]
				childIdx, err := browseNode(cfg, stdscr, node.Children[folderName], 0)
				if err != nil {
					return currentIndex, err
				}
				currentIndex = selected
				_ = childIdx
			} else {
				file := node.Files[selected-len(node.ChildNames)]
				sys, _ := games.GetSystem(file.SystemId)
				_ = mister.LaunchGame(cfg, *sys, file.Path)
			}
		case 3:
			return currentIndex, nil
		}
	}
}

// -------------------------
// Options menu
// -------------------------
func optionsMenu(cfg *config.UserConfig, stdscr *gc.Window) (gamesdb.GobIndex, error) {
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
		InitialIndex:  0,
	}, []string{"Rebuild games database..."})
	if err != nil {
		return gamesdb.GobIndex{}, err
	}

	if button == 0 && selected == 0 {
		return generateIndexWindow(cfg, stdscr)
	}
	return gamesdb.GobIndex{}, nil
}

// -------------------------
// Main menu
// -------------------------
func mainMenu(cfg *config.UserConfig, stdscr *gc.Window, idx gamesdb.GobIndex) error {
	tree := buildTree(idx.Entries)

	sysNames := tree.ChildNames
	items := append([]string{}, sysNames...)

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
			DynamicActionLabel: func(idx int) string {
				return "Open"
			},
		}, items)
		if err != nil {
			return err
		}

		startIndex = selected
		switch button {
		case 2:
			sysName := sysNames[selected]
			_, err := browseNode(cfg, stdscr, tree.Children[sysName], 0)
			if err != nil {
				return err
			}
		case 3:
			if err := searchWindow(cfg, stdscr, idx); err != nil {
				return err
			}
			stdscr.Clear()
			stdscr.Refresh()
		case 4:
			newIdx, err := optionsMenu(cfg, stdscr)
			if err != nil {
				return err
			}
			if len(newIdx.Entries) > 0 {
				idx = newIdx
				tree = buildTree(idx.Entries)
				sysNames = tree.ChildNames
				items = append([]string{}, sysNames...)
			}
		case 5:
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
		gc.Cursor(1)
		button, query, err := curses.OnScreenKeyboard(stdscr, "Search", []string{"Search", "Back"}, text, 0)
		gc.Cursor(0)

		if err != nil || button == 1 {
			return nil
		}
		text = query

		results := idx.SearchWords(query)
		if len(results) == 0 {
			_ = curses.InfoBox(stdscr, "", "No results found.", false, true)
			continue
		}

		items := make([]string, len(results))
		for i, r := range results {
			items[i] = r.SearchName
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
				continue
			}
			if button == 3 {
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
func loadingWindow(stdscr *gc.Window, loadFn func() (gamesdb.GobIndex, error)) (gamesdb.GobIndex, error) {
	status := struct {
		Done bool
		Err  error
		Idx  gamesdb.GobIndex
	}{}

	go func() {
		idx, err := loadFn()
		status.Idx = idx
		status.Err = err
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

	if status.Err != nil {
		return gamesdb.GobIndex{}, status.Err
	}
	return status.Idx, nil
}

// -------------------------
// Main
// -------------------------
func main() {
	// Lockfile
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

	// Flags
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

	gc.Cursor(0)
	defer gc.Cursor(1)

	idx, err := loadingWindow(stdscr, loadMenuDb)
	if err != nil {
		idx, err = generateIndexWindow(cfg, stdscr)
		if err != nil {
			log.Fatal(err)
		}
	}

	if launchGame {
		if err := mainMenu(cfg, stdscr, idx); err != nil {
			log.Fatal(err)
		}
	} else {
		for _, f := range idx.Entries {
			displayName := f.Name
			if f.Ext != "" {
				displayName = fmt.Sprintf("%s.%s", f.Name, f.Ext)
			}
			fmt.Println(filepath.Join(f.MenuPath, displayName))
		}
	}
}
