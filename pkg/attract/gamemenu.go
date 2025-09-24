package attract

import (
	"fmt"
	"log"
	"os"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Shared launcher: reset root and run
func runApp(p tview.Primitive) {
	app := tview.NewApplication()
	if err := app.SetRoot(p, true).Run(); err != nil {
		log.Fatal(err)
	}
}

// ---------------
// MENU 1: Simple modal (one-shot popup)
// ---------------
func GameMenu1() {
	modal := tview.NewModal().
		SetText("This is Test Menu 1").
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(i int, label string) { os.Exit(0) })
	runApp(modal)
}

// ---------------
// MENU 2: List of fixed items
// ---------------
func GameMenu2() {
	list := tview.NewList().
		AddItem("Option A", "Demo entry", 'a', nil).
		AddItem("Option B", "Another entry", 'b', nil).
		AddItem("Quit", "Return", 'q', func() { os.Exit(0) })
	runApp(list)
}

// ---------------
// MENU 3: Two-column system/game browser (static data)
// ---------------
func GameMenu3() {
	systems := []string{"NES", "SNES", "Genesis"}
	games := map[string][]string{
		"NES":     {"Mario Bros", "Zelda"},
		"SNES":    {"Super Mario World", "Donkey Kong Country"},
		"Genesis": {"Sonic", "Streets of Rage"},
	}

	sysList := tview.NewList()
	gameList := tview.NewList()

	for _, sys := range systems {
		s := sys // local copy for closure
		sysList.AddItem(s, "", 0, func() {
			gameList.Clear()
			for _, g := range games[s] {
				gameList.AddItem(g, "", 0, func() {
					fmt.Printf("Launching %s on %s\n", g, s)
				})
			}
		})
	}

	flex := tview.NewFlex().
		AddItem(sysList, 0, 1, true).
		AddItem(gameList, 0, 2, false)

	runApp(flex)
}

// ---------------
// MENU 4: Input form (like a settings dialog)
// ---------------
func GameMenu4() {
	form := tview.NewForm().
		AddInputField("Name", "", 20, nil, nil).
		AddPasswordField("Password", "", 20, '*', nil).
		AddCheckbox("Enable feature", false, nil).
		AddButton("Save", func() { fmt.Println("Saved!") }).
		AddButton("Cancel", func() { os.Exit(0) })
	form.SetBorder(true).SetTitle("Settings").SetTitleAlign(tview.AlignCenter)
	runApp(form)
}

// ---------------
// MENU 5: TextView log output (scrollable console)
// ---------------
func GameMenu5() {
	text := tview.NewTextView().SetDynamicColors(true).SetScrollable(true)
	fmt.Fprintln(text, "[yellow]System Log[white]")
	for i := 1; i <= 20; i++ {
		fmt.Fprintf(text, "Line %d: Demo output\n", i)
	}
	runApp(text)
}

// ---------------
// MENU 6: Table (rows/cols)
// ---------------
func GameMenu6() {
	table := tview.NewTable().SetBorders(true)
	headers := []string{"ID", "Name", "Status"}
	for c, h := range headers {
		table.SetCell(0, c, tview.NewTableCell(h).
			SetTextColor(tcell.ColorYellow).
			SetAlign(tview.AlignCenter).
			SetSelectable(false))
	}
	data := [][]string{
		{"1", "Mario", "OK"},
		{"2", "Zelda", "Missing"},
		{"3", "Sonic", "OK"},
	}
	for r, row := range data {
		for c, val := range row {
			table.SetCell(r+1, c, tview.NewTableCell(val))
		}
	}
	runApp(table)
}

// ---------------
// MENU 7: Pages (switchable views)
// ---------------
func GameMenu7() {
	pages := tview.NewPages()
	pages.AddPage("main", tview.NewTextView().SetText("Main Page (press n)"), true, true)
	pages.AddPage("next", tview.NewTextView().SetText("Next Page (press b)"), true, false)

	app := tview.NewApplication()
	app.SetRoot(pages, true)

	pages.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		switch ev.Rune() {
		case 'n':
			pages.SwitchToPage("next")
		case 'b':
			pages.SwitchToPage("main")
		case 'q':
			app.Stop()
		}
		return ev
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// ---------------
// MENU 8: Like your shell script – systems & games from files
// ---------------
func GameMenu8() {
	root := "/media/fat/Scripts/.MiSTer_SAM/SAM_Gamelists"
	sysList := tview.NewList()
	gameList := tview.NewList()

	entries, err := os.ReadDir(root)
	if err != nil {
		fmt.Printf("Error reading %s: %v\n", root, err)
		return
	}

	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() && len(name) > 12 && name[len(name)-13:] == "_gamelist.txt" {
			system := name[:len(name)-13]
			sysList.AddItem(system, "", 0, func() {
				gameList.Clear()
				path := root + "/" + system + "_gamelist.txt"
				data, _ := os.ReadFile(path)
				lines := strings.Split(string(data), "\n")
				for _, l := range lines {
					if l != "" {
						gameList.AddItem(l, "", 0, func() {
							fmt.Printf("Would run %s on %s\n", l, system)
						})
					}
				}
			})
		}
	}

	flex := tview.NewFlex().
		AddItem(sysList, 0, 1, true).
		AddItem(gameList, 0, 2, false)
	runApp(flex)
}

// ---------------
// MENU 9: Fullscreen message + quit on key
// ---------------
func GameMenu9() {
	text := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetText("TEST MENU 9\n\nPress any key to quit.")
	app := tview.NewApplication()
	text.SetDoneFunc(func(key tcell.Key) { app.Stop() })
	if err := app.SetRoot(text, true).Run(); err != nil {
		log.Fatal(err)
	}
}
