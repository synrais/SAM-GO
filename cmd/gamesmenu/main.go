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
// Aliases & helpers
// -------------------------

type MenuFile = gamesdb.FileInfo

func clearScreen(stdscr *gc.Window) {
	stdscr.Clear()
	stdscr.Refresh()
}

func launchGame(cfg *config.UserConfig, stdscr *gc.Window, f MenuFile) {
	sys, _ := games.GetSystem(f.SystemId)
	_ = mister.LaunchGame(cfg, *sys, f.Path)
	clearScreen(stdscr)
}

// Run any function in a goroutine while showing a spinner
func runWithSpinner(stdscr *gc.Window, label string, fn func() error) error {
	done := false
	var runErr error
	go func() {
		runErr = fn()
		done = true
	}()
	spinnerSeq := []string{"|", "/", "-", "\\"}
	spinnerCount := 0
	for {
		if done {
			break
		}
		_ = curses.InfoBox(stdscr, "", fmt.Sprintf("%s %s", label, spinnerSeq[spinnerCount]), false, false)
		spinnerCount = (spinnerCount + 1) % len(spinnerSeq)
		_ = gc.Update()
		gc.Nap(100)
	}
	clearScreen(stdscr)
	return runErr
}

// -------------------------
// Loading & Indexing
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

func generateIndexWindow(cfg *config.UserConfig, stdscr *gc.Window) ([]MenuFile, error) {
    _ = gamesdb.ResetDatabase()

    var count int
    var currentStatus gamesdb.IndexStatus
    var done bool
    var err error

    go func() {
        count, err = gamesdb.NewNamesIndex(cfg, games.AllSystems(), func(is gamesdb.IndexStatus) {
            currentStatus = is
        })
        done = true
    }()

    spinnerSeq := []string{"|", "/", "-", "\\"}
    spinnerCount := 0
    for {
        if done {
            break
        }

        sysName := currentStatus.SystemId
        if sys, serr := games.GetSystem(currentStatus.SystemId); serr == nil {
            sysName = sys.Name
        }

        text := fmt.Sprintf("Indexing %s... (%d files)", sysName, currentStatus.Files)
        label := fmt.Sprintf("%s %s", text, spinnerSeq[spinnerCount])
        _ = curses.InfoBox(stdscr, "", label, false, false)

        spinnerCount = (spinnerCount + 1) % len(spinnerSeq)
        _ = gc.Update()
        gc.Nap(100)
    }

    if err != nil {
        return nil, err
    }

    _ = curses.InfoBox(stdscr, "", fmt.Sprintf("Indexed %d games successfully.", count), false, true)

    files, err := loadMenuDb()
    if err != nil {
        return nil, err
    }
    clearScreen(stdscr)
    return files, nil
}

// -------------------------
// Options Menu
// -------------------------

func optionsMenu(cfg *config.UserConfig, stdscr *gc.Window) ([]MenuFile, error) {
	clearScreen(stdscr)
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
	clearScreen(stdscr)

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
		clearScreen(stdscr)
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

		button, selected, err := curses.ListPicker(stdscr, curses.ListPickerOpts{
			Title:         title,
			Buttons:       []string{"PgUp", "PgDn", "", "Back"},
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
		case 2:
			clearScreen(stdscr)
			if selected < len(folders) {
				childName := folders[selected]
				childIdx, err := browseNode(cfg, stdscr, node.Children[childName], 0)
				if err != nil {
					return currentIndex, err
				}
				currentIndex = childIdx
			} else {
				file := node.Files[selected-len(folders)]
				launchGame(cfg, stdscr, file)
			}
		case 3:
			clearScreen(stdscr)
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

	startIndex := 0
	for {
		clearScreen(stdscr)
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
		}, sysIds)
		if err != nil {
			return err
		}

		startIndex = selected
		clearScreen(stdscr)

		switch button {
		case 2:
			sysId := sysIds[selected]
			_, err := browseNode(cfg, stdscr, tree.Children[sysId], 0)
			if err != nil {
				return err
			}
			clearScreen(stdscr)
		case 3:
			if err := searchWindow(cfg, stdscr); err != nil {
				return err
			}
			clearScreen(stdscr)
		case 4:
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
			clearScreen(stdscr)
		case 5:
			clearScreen(stdscr)
			return nil
		}
	}
}

// -------------------------
// Search Window
// -------------------------

func searchWindow(cfg *config.UserConfig, stdscr *gc.Window) error {
	clearScreen(stdscr)
	text := ""
	startIndex := 0
	for {
		gc.Cursor(1)
		button, query, err := curses.OnScreenKeyboard(stdscr, "Search", []string{"Search", "Back"}, text, 0)
		gc.Cursor(0)
		if err != nil || button == 1 {
			clearScreen(stdscr)
			return nil
		}
		text = query

		var results []gamesdb.SearchResult
		err = runWithSpinner(stdscr, "Searching...", func() error {
			var err error
			results, err = gamesdb.SearchNamesWords(games.AllSystems(), query)
			return err
		})
		if err != nil {
			return err
		}
		clearScreen(stdscr)

		if len(results) == 0 {
			_ = curses.InfoBox(stdscr, "", "No results found.", false, true)
			clearScreen(stdscr)
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
			clearScreen(stdscr)
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
				launchGame(cfg, stdscr, gamesdb.FileInfo{
					SystemId: game.SystemId,
					Name:     game.Name,
					Ext:      game.Ext,
					Path:     game.Path,
				})
			} else if button == 3 {
				clearScreen(stdscr)
				break
			}
		}
	}
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
	launchMode := !*printPtr

	cfg, err := config.LoadUserConfig("gamesmenu", &config.UserConfig{})
	if err != nil && !os.IsNotExist(err) {
		log.Fatal(err)
	}

	stdscr, err := curses.Setup()
	if err != nil {
		log.Fatal(err)
	}
	defer gc.End()
	gc.Cursor(0)

	files, err := loadMenuDb()
	if err != nil {
		files, err = generateIndexWindow(cfg, stdscr)
		if err != nil {
			log.Fatal(err)
		}
	}

	if launchMode {
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
