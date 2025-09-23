//go:build linux

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// fallback runner (from your snippet)
func tryRunApp(app *tview.Application, builder func() (*tview.Application, error)) error {
	if err := app.Run(); err != nil {
		// fallback build
		appTty2, err := builder()
		if err != nil {
			return err
		}

		ttyPath := "/dev/tty2"
		if os.Getenv("ZAPAROO_RUN_SCRIPT") == "2" {
			ttyPath = "/dev/tty4"
		}

		tty, err := tcell.NewDevTtyFromDev(ttyPath)
		if err != nil {
			return fmt.Errorf("failed to create tty from device %s: %w", ttyPath, err)
		}

		screen, err := tcell.NewTerminfoScreenFromTty(tty)
		if err != nil {
			return fmt.Errorf("failed to create screen from tty: %w", err)
		}

		appTty2.SetScreen(screen)
		if err := appTty2.Run(); err != nil {
			return fmt.Errorf("failed to run TUI application: %w", err)
		}
	}
	return nil
}

func buildMenuApp() (*tview.Application, error) {
	root := "/media/fat/Scripts/.MiSTer_SAM/SAM_Gamelists"

	// discover gamelists
	files, err := filepath.Glob(filepath.Join(root, "*_gamelist.txt"))
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no gamelists found in %s", root)
	}

	app := tview.NewApplication()
	list := tview.NewList().
		ShowSecondaryText(false).
		SetBorder(true).
		SetTitle("MiSTer Game Browser")

	// add systems
	for _, f := range files {
		system := strings.TrimSuffix(filepath.Base(f), "_gamelist.txt")
		list.AddItem(system, "", 0, func() {
			showGameList(app, system, f)
		})
	}

	app.SetRoot(list, true)
	return app, nil
}

func showGameList(app *tview.Application, system, gamelistPath string) {
	list := tview.NewList().
		ShowSecondaryText(false).
		SetBorder(true).
		SetTitle(fmt.Sprintf("%s Games", system))

	data, _ := ioutil.ReadFile(gamelistPath)
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name := filepath.Base(line)
		name = strings.TrimSuffix(name, filepath.Ext(name))
		path := line

		list.AddItem(name, "", 0, func() {
			// launch game with SAM
			cmd := fmt.Sprintf("/media/fat/Scripts/.MiSTer_SAM/SAM -run \"%s\"", path)
			app.Stop()
			fmt.Println(cmd)
			// optional: actually call exec.Command here if you want
		})
	}

	list.AddItem("< Back >", "", 0, func() {
		app.Stop()
		main() // rebuild from scratch
	})

	app.SetRoot(list, true).SetFocus(list)
}

func main() {
	app, err := buildMenuApp()
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	if err := tryRunApp(app, buildMenuApp); err != nil {
		fmt.Println("TUI failed:", err)
		os.Exit(1)
	}
}
