package main

import (
	"encoding/gob"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"

	gc "github.com/rthornton128/goncurses"

	"github.com/synrais/SAM-GO/pkg/attract"
	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/curses"
	"github.com/synrais/SAM-GO/pkg/games"
	"github.com/synrais/SAM-GO/pkg/gamesdb"
	"github.com/synrais/SAM-GO/pkg/mister"
	"github.com/synrais/SAM-GO/pkg/music"
)

// -------------------------
// Aliases
// -------------------------

type MenuFile = gamesdb.FileInfo

// errGameLaunched unwinds the menu so the program exits once a game starts.
var errGameLaunched = errors.New("game launched")

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

func generateIndexWindow(cfg *config.Config, stdscr *gc.Window) ([]MenuFile, error) {
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
		SystemName  string
		Files       int
		Complete    bool
		Error       error
	}{}

	go func() {
		_, err = gamesdb.NewNamesIndex(cfg, games.AllSystems(), func(is gamesdb.IndexStatus) {
			sysName := is.SystemId
			if sys, err := games.GetSystem(is.SystemId); err == nil {
				sysName = sys.Name
			}
			status.Step, status.Total, status.SystemName, status.Files = is.Step, is.Total, sysName, is.Files
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

		countText := fmt.Sprintf("%6d files", status.Files)
		countCol := width - len(countText) - 6
		win.MovePrint(1, countCol, countText)

		maxSysWidth := countCol - 10
		sysText := fmt.Sprintf("Indexing %s...", status.SystemName)
		if len(sysText) > maxSysWidth {
			sysText = sysText[:maxSysWidth]
		}
		win.MovePrint(1, 2, sysText)

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

func optionsMenu(cfg *config.Config, stdscr *gc.Window, files []MenuFile, sysIds []string) ([]MenuFile, error) {
	// Each group stays open after its screens, with the same entry
	// highlighted, until Back. Rebuilding the database and starting
	// attract mode leave the options straight away.
	groups := []string{"Game Database", "Attract Mode", "Display & Sorting", "Controls", "Music Player"}
	group := 0
	for {
		sel, ok := optionsList(stdscr, "Options", groups, group)
		if !ok {
			return nil, nil
		}
		group = sel

		var newFiles []MenuFile
		var err error
		leave := false
		switch sel {
		case 0:
			newFiles, leave, err = optionsGroup(stdscr, "Game Database", []string{
				"Rebuild games database...",
				"Database systems...",
			}, func(i int) ([]MenuFile, bool, error) {
				if i == 0 || databaseSystemsScreen(stdscr, cfg) {
					f, err := generateIndexWindow(cfg, stdscr)
					return f, true, err
				}
				return nil, false, nil
			})
		case 1:
			newFiles, leave, err = optionsGroup(stdscr, "Attract Mode", []string{
				"Start attract mode",
				"Attract mode settings...",
				"Detector & list settings...",
				"On startup...",
			}, func(i int) ([]MenuFile, bool, error) {
				switch i {
				case 0:
					if err := startAttractInBackground(); err != nil {
						_ = curses.InfoBox(stdscr, "Error",
							fmt.Sprintf("Failed to start attract mode: %v", err), false, true)
						return nil, false, nil
					}
					return nil, true, errGameLaunched // the menu exits, attract mode carries on
				case 1:
					attractSettingsScreen(stdscr, cfg, sysIds)
				case 2:
					detectorSettingsScreen(stdscr, cfg)
				case 3:
					startupScreen(stdscr, cfg)
				}
				return nil, false, nil
			})
		case 2:
			newFiles, leave, err = optionsGroup(stdscr, "Display & Sorting", []string{
				"Menu list options...",
				"Systems list sorting...",
				"Game list sorting...",
				"Virtual A-Z Folders...",
			}, func(i int) ([]MenuFile, bool, error) {
				switch i {
				case 0:
					menuListOptions(stdscr, sysIds, cfg)
				case 1:
					systemsSortOptions(stdscr, sysIds, cfg)
				case 2:
					gameSortOptions(stdscr, cfg)
				case 3:
					azFoldersScreen(stdscr, cfg, sysIds)
				}
				return nil, false, nil
			})
		case 3:
			controlsScreen(stdscr, cfg)
		case 4:
			musicScreen(stdscr, cfg)
		}
		if err != nil || leave {
			return newFiles, err
		}
	}
}

// optionsGroup shows one group of options until Back, or until an entry
// says to leave the options altogether.
func optionsGroup(stdscr *gc.Window, title string, items []string,
	run func(i int) ([]MenuFile, bool, error)) ([]MenuFile, bool, error) {
	selected := 0
	for {
		sel, ok := optionsList(stdscr, title, items, selected)
		if !ok {
			return nil, false, nil
		}
		selected = sel
		files, leave, err := run(sel)
		if err != nil || leave {
			return files, leave, err
		}
	}
}

// optionsList shows a list of options and returns the one chosen, or false
// for Back.
func optionsList(stdscr *gc.Window, title string, items []string, selected int) (int, bool) {
	stdscr.Clear()
	stdscr.Refresh()
	button, sel, err := curses.ListPicker(stdscr, curses.ListPickerOpts{
		Shortcuts:     menuShortcuts(),
		Title:         title,
		Buttons:       []string{"Select", "Back"},
		DefaultButton: 0,
		ActionButton:  0,
		Width:         60,
		Height:        len(items) + 4,
		InitialIndex:  selected,
	}, items)
	stdscr.Clear()
	stdscr.Refresh()
	if err != nil || button != 0 || sel < 0 || sel >= len(items) {
		return 0, false
	}
	return sel, true
}

// -------------------------
// Tree Navigation
// -------------------------

// browseNode shows one folder of the games tree. It reports whether the user
// left it with Back, so the parent can keep Back highlighted and the user can
// hammer Back to climb out of deep folders.
type browseEntry = gamesdb.Entry

func browseNode(cfg *config.Config, stdscr *gc.Window, node *gamesdb.Node, depth int) (bool, error) {
	const actionButton, backButton = 2, 3
	currentIndex := 0
	defaultButton := actionButton
	first := true
	for {
		stdscr.Clear()
		stdscr.Refresh()

		entries := node.Entries(gameOrder(), optFolders.value())

		items := make([]string, len(entries))
		for i, e := range entries {
			if e.File == nil {
				items[i] = e.Folder
			} else {
				items[i] = gameName(e.File.Name, e.File.Ext)
			}
		}

		title := node.Name
		if title == "" {
			title = "Games"
		}

		// Walking back to a remembered position: start on its line here,
		// and open it straight away if the position goes deeper.
		autoOpen := false
		if first {
			first = false
			if idx, ok, found, open := restoreAt(depth, func(i int) string { return browseKey(entries[i]) }, len(entries)); ok {
				currentIndex = idx
				autoOpen = found && open && entries[idx].File == nil
				if !autoOpen {
					doneRestoring(depth)
				}
			}
		}

		buttons := []string{"PgUp", "PgDn", "", "Back"}
		button, selected, err := actionButton, currentIndex, error(nil)
		if !autoOpen {
			button, selected, err = curses.ListPicker(stdscr, curses.ListPickerOpts{
				Shortcuts:     menuShortcuts(),
				Title:         title,
				Buttons:       buttons,
				ActionButton:  actionButton,
				DefaultButton: defaultButton,
				SnapToAction:  true,
				ShowTotal:     true,
				Width:         systemListWidth,
				Height:        listHeight,
				InitialIndex:  currentIndex,
				DynamicActionLabel: func(idx int) string {
					if idx >= 0 && idx < len(entries) && entries[idx].File == nil {
						return "Open"
					}
					return "Launch"
				},
			}, items)
		}
		if err != nil {
			return false, err
		}

		// Keep our place in this list (Back reports no selection).
		if selected >= 0 {
			currentIndex = selected
			if selected < len(entries) {
				navHere(depth, browseKey(entries[selected]), selected)
			}
		}
		defaultButton = actionButton

		switch button {
		case actionButton:
			stdscr.Clear()
			stdscr.Refresh()
			if selected < 0 || selected >= len(entries) {
				continue
			}
			if e := entries[selected]; e.File == nil {
				wentBack, err := browseNode(cfg, stdscr, folderNode(node, e), depth+1)
				if err != nil {
					return false, err
				}
				if wentBack {
					defaultButton = backButton
				}
			} else {
				file := entries[selected].File
				sys, err := games.GetSystem(file.SystemId)
				if err == nil && mister.LaunchGame(cfg, *sys, file.Path) == nil {
					savePosition()
					return false, errGameLaunched
				}
				stdscr.Clear()
				stdscr.Refresh()
			}
		case backButton:
			stdscr.Clear()
			stdscr.Refresh()
			return true, nil
		}
	}
}

// -------------------------
// Main Menu
// -------------------------

func mainMenu(cfg *config.Config, stdscr *gc.Window, files []MenuFile) error {
	setupPosition(cfg)
	st := &menuState{}
	st.load(files)
	if startInSearch {
		if err := searchWindow(cfg, stdscr); err != nil {
			return err
		}
	}
	return systemsScreen(cfg, stdscr, st, "Systems", nil, true)
}

// -------------------------
// Search Window
// -------------------------

func searchWindow(cfg *config.Config, stdscr *gc.Window) error {
	stdscr.Clear()
	stdscr.Refresh()

	text := ""
	startIndex := 0
	for {
		// Controller keys: Cross types (and presses Search), Circle backs
		// out, Square types a space, Triangle deletes, L/R move the cursor
		// (see KeyboardOpts).
		gc.Cursor(1)
		button, query, err := curses.OnScreenKeyboardWith(stdscr, "Search", []string{"Search", "Back"}, text, 0,
			curses.KeyboardOpts{PadKeys: true})
		gc.Cursor(0)

		// Only the Search button searches: Back and Esc (Circle) back out.
		if err != nil || button != 0 {
			stdscr.Clear()
			stdscr.Refresh()
			return nil
		}
		text = query

		// Nothing to search for: stay on the keyboard (searching for nothing
		// would list every game).
		if strings.TrimSpace(query) == "" {
			_ = curses.InfoBox(stdscr, "", "Type something to search for first.", false, false)
			gc.Nap(1200)
			stdscr.Clear()
			stdscr.Refresh()
			continue
		}
		startIndex = 0
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

		// Search results always read "[System] Title.ext": systems A-Z, then
		// titles A-Z, whatever the menu's sorting and extension settings.
		results := status.Result
		gamesdb.SortResultsBySystem(results, func(id string) string {
			if sys, err := games.GetSystem(id); err == nil {
				return sys.Name
			}
			return id
		})
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
				display += "." + r.Ext
			}
			items = append(items, fmt.Sprintf("[%s] %s", systemName, display))
		}

		for {
			stdscr.Clear()
			stdscr.Refresh()
			button, selected, err := curses.ListPicker(stdscr, curses.ListPickerOpts{
				Shortcuts:     menuShortcuts(),
				Title:         "Search Results",
				Buttons:       []string{"PgUp", "PgDn", "Launch", "Back"},
				ActionButton:  2,
				DefaultButton: 2,
				ShowTotal:     true,
				Width:         systemListWidth,
				Height:        listHeight,
				InitialIndex:  startIndex,
			}, items)
			if err != nil {
				return err
			}
			startIndex = selected
			if button == 2 {
				game := results[selected]
				sys, err := games.GetSystem(game.SystemId)
				if err == nil && mister.LaunchGame(cfg, *sys, game.Path) == nil {
					savePosition()
					return errGameLaunched
				}
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
// Attract (command line)
// -------------------------

// runAttract starts attract mode without the menu screens (gamesmenu.sh
// -attract), printing its progress to the terminal. The games database is
// built first if it doesn't exist yet.
func runAttract(cfg *config.Config) {
	files, err := loadMenuDb()
	if err != nil {
		fmt.Println("[Menu] No games database found, building...")
		prevSystem, prevTotal, done := "", 0, 0
		if _, err := gamesdb.NewNamesIndex(cfg, games.AllSystems(), func(s gamesdb.IndexStatus) {
			if prevSystem != "" {
				done++
				fmt.Printf("[DB] %d/%d %s: %d games (total %d)\n", done, s.Total-1, prevSystem, s.Files-prevTotal, s.Files)
			}
			prevSystem, prevTotal = s.SystemId, s.Files
		}); err != nil {
			log.Fatal(err)
		}
		if files, err = loadMenuDb(); err != nil {
			log.Fatal(err)
		}
	}

	if err := attract.StartAttractMode(cfg, files); err != nil {
		log.Fatal(err)
	}
}

// -------------------------
// Main Entry
// -------------------------

func main() {
	listPtr := flag.Bool("list", false, "Print every game in the database, one per line")
	printPtr := flag.Bool("print", false, "Same as -list")
	statusPtr := flag.Bool("status", false, "Show whether attract mode is running, and what it's playing")
	stopPtr := flag.Bool("stop", false, "Stop attract mode and go back to the MiSTer menu")
	nextPtr := flag.Bool("next", false, "Attract mode: next game")
	backPtr := flag.Bool("back", false, "Attract mode: previous game")
	playPtr := flag.Bool("play", false, "Attract mode: stop the show and keep playing this game")
	stayPtr := flag.Bool("stay", false, "Attract mode: stay on this game (again to resume)")
	blacklistPtr := flag.Bool("blacklist", false, "Attract mode: never play this game again, next game")
	rebuildPtr := flag.Bool("rebuild", false, "Rebuild the games database (no screens)")
	launchPtr := flag.String("launch", "", "Launch a game file, e.g. -launch /media/fat/games/NES/Tetris.nes")
	randomPtr := flag.Bool("random", false, "Launch a random game, from the systems or groups given (e.g. -random Nintendo)")
	bgPtr := flag.Bool("bg", false, "With -attract: run in the background (log in /tmp/gamesmenu_attract.log)")
	menuPtr := flag.Bool("menu", false, "Open the games menu on the TV (closes the running game)")
	searchPtr := flag.Bool("search", false, "Open the games menu's Search on the TV (closes the running game)")
	openMenuPtr := flag.String("openmenu", "", "Internal: open the games menu on the TV (menu or search)")
	findPtr := flag.Bool("find", false, "Attract mode: play the games matching the words given first, e.g. -find mario 3")
	musicPtr := flag.String("music", "", "Music player: start, stop, next or status")
	musicdPtr := flag.Bool("musicd", false, "Internal: the music player process")
	bootPtr := flag.String("boot", "", "Internal: run at MiSTer startup (menu or attract)")
	attractPtr := flag.Bool("attract", false, "Start attract mode straight away (no menu screens)")
	watchPtr := flag.Bool("watch", false, "Show the static detector's live status (leaves a running menu or attract mode alone)")
	inputsPtr := flag.Bool("inputs", false, "Print every key, mouse and controller press, to test the input detectors (leaves a running menu or attract mode alone)")
	pressPtr := flag.Bool("press", false, "Press buttons on SAM's virtual pad or keyboard, e.g. -press start (leaves a running menu or attract mode alone)")
	autoInputPtr := flag.String("autoinput", "", "Internal: run a BIOS skip sequence in the background")
	flag.Parse()
	launchGame := true
	if *listPtr || *printPtr {
		listGames()
		return
	}
	if *statusPtr {
		fmt.Println(attract.Status())
		return
	}
	for cmd, on := range map[string]bool{"stop": *stopPtr, "next": *nextPtr, "back": *backPtr,
		"play": *playPtr, "stay": *stayPtr, "blacklist": *blacklistPtr} {
		if on {
			remoteCommand(cmd)
			return
		}
	}
	if *musicdPtr {
		music.Run(mustConfig())
		return
	}
	if *musicPtr != "" {
		musicCommand(*musicPtr)
		return
	}
	if *bootPtr != "" {
		bootStart(*bootPtr)
		return
	}
	if *findPtr {
		query := strings.TrimSpace(strings.Join(flag.Args(), " "))
		if query == "" {
			fmt.Println("Usage: gamesmenu.sh -find <words>, e.g. -find mario 3")
			os.Exit(1)
		}
		remoteCommand("find:" + query)
		return
	}
	if *openMenuPtr != "" {
		if err := mister.OpenGamesMenu(*openMenuPtr == "search"); err != nil {
			fmt.Println("Couldn't open the games menu:", err)
			if f, e := os.OpenFile(mister.OpenMenuLog, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); e == nil {
				fmt.Fprintln(f, "FAILED:", err)
				f.Close()
			}
		}
		return
	}
	// On the TV these just open the menu (-search straight into Search);
	// from anywhere else they open it on the TV.
	if (*menuPtr || *searchPtr) && !mister.OnConsole() {
		openOnTV(*searchPtr)
		return
	}
	startInSearch = *searchPtr

	if *rebuildPtr {
		rebuildDatabase()
		return
	}
	if *launchPtr != "" {
		launchFile(*launchPtr)
		return
	}
	if *randomPtr {
		launchRandom(flag.Args())
		return
	}
	if *attractPtr && *bgPtr {
		if err := startAttractInBackground(flag.Args()...); err != nil {
			fmt.Println("Couldn't start attract mode:", err)
			os.Exit(1)
		}
		fmt.Println("Attract mode started in the background. Log: " + attractLog)
		return
	}

	// Watching only reads a status file, so it never takes the lock (which
	// would stop a running menu or attract mode).
	if *watchPtr {
		watchDetector()
		return
	}
	if *inputsPtr {
		testInputs()
		return
	}
	if *autoInputPtr != "" {
		mister.RunAutoInput(*autoInputPtr)
		return
	}
	if *pressPtr {
		testPress(flag.Args())
		return
	}

	lockFile := "/tmp/gamesmenu.lock"
	f, err := os.OpenFile(lockFile, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		log.Fatalf("failed to open lock file: %v", err)
	}
	defer f.Close()
	menuLock = f

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

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	applyMenuConfig(cfg.Menu)
	applyAZConfig(cfg.Menu)
	menuControls = cfg.MenuControls
	curses.SwapConfirmBack = cfg.MenuLayout == "Japanese"

	if *attractPtr {
		// Attract mode can run for hours on a MiSTer with ~500 MB shared
		// with Linux and MiSTer's own program: keep Go's memory in check.
		debug.SetMemoryLimit(128 * 1024 * 1024)

		// Systems or groups after -attract replace [Attract] Include for
		// this run, e.g. -attract Nintendo,Console.
		if args := flag.Args(); len(args) > 0 {
			cfg.Attract.Include = nil
			for _, a := range args {
				cfg.Attract.Include = append(cfg.Attract.Include, strings.Split(a, ",")...)
			}
		}
		runAttract(cfg)
		return
	}

	applyMenuConfig(cfg.Menu) // Text size is needed before the screen is set up
	applyTextSize()

	stdscr, err := curses.Setup()
	if err != nil {
		log.Fatal(err)
	}
	defer gc.End()
	if err := fitToScreen(stdscr); err != nil {
		gc.End()
		fmt.Println(err)
		return
	}

	gc.Cursor(0)

	files, err := loadingWindow(stdscr, loadMenuDb)
	if err != nil {
		files, err = generateIndexWindow(cfg, stdscr)
		if err != nil {
			log.Fatal(err)
		}
	}

	if launchGame {
		err := mainMenu(cfg, stdscr, files)
		if err != nil && !errors.Is(err, errGameLaunched) {
			log.Fatal(err)
		}
		if err == nil { // Exit
			gc.End()
			if mister.OnConsole() {
				// On the MiSTer's own screen, go straight back to the MiSTer
				// menu. Otherwise MiSTer shows "Press any key to continue"
				// after the script ends. Reloading the menu also resets the
				// text size.
				if mister.LaunchMenu() == nil {
					return
				}
			}
			restoreTextSize()
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

// menuLock is this process's lock file (see main).
var menuLock *os.File

// attractLog is where attract mode started from the menu writes its output.
const attractLog = "/tmp/gamesmenu_attract.log"

// startAttractInBackground runs "gamesmenu -attract" as its own process, in
// its own session, so it keeps running after the menu exits. The menu's
// lock is released first, so the new process takes it over instead of
// stopping the menu.
func startAttractInBackground(systems ...string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	logFile, err := os.OpenFile(attractLog, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer logFile.Close()

	if menuLock != nil {
		_ = menuLock.Truncate(0)
		_ = syscall.Flock(int(menuLock.Fd()), syscall.LOCK_UN)
	}
	cmd := exec.Command(exe, append([]string{"-attract"}, systems...)...)
	cmd.Stdout, cmd.Stderr = logFile, logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd.Start()
}

// buildTree builds the games tree, with each system folder named from its
// current system name (so a renamed system never shows up as "Other").
func buildTree(files []MenuFile) *gamesdb.Node {
	tree := gamesdb.BuildTree(files)
	tree.RenameSystems(func(id string) string {
		if sys, err := games.GetSystem(id); err == nil {
			return sys.Name
		}
		return ""
	})
	addAZFolders(tree)
	return tree
}

// folderNode is the folder an entry opens: a real subfolder, or a disc set.
func folderNode(parent *gamesdb.Node, e gamesdb.Entry) *gamesdb.Node {
	if e.Node != nil {
		return e.Node
	}
	return parent.Children[e.Folder]
}
