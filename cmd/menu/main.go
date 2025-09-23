package main

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/gdamore/tcell/v2"
)

func resetToMenuCore() error {
	cmd := exec.Command("sh", "-c", "echo load_core /media/fat/menu.rbf > /dev/MiSTer_cmd")
	return cmd.Run()
}

func main() {
	// Step 1: Force menu core
	if err := resetToMenuCore(); err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Failed to reset to menu core: %v\n", err)
		os.Exit(1)
	}

	// Small delay so menu.rbf actually loads
	time.Sleep(500 * time.Millisecond)

	// Step 2: Init TUI
	screen, err := tcell.NewScreen()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create screen: %v\n", err)
		os.Exit(1)
	}
	if err := screen.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to init screen: %v\n", err)
		os.Exit(1)
	}
	defer screen.Fini()

	screen.Clear()
	drawText(screen, 1, 1, tcell.StyleDefault, "SAM-GO Test Menu (menu core loaded)")
	screen.Show()

	// Wait for ESC to exit
	for {
		ev := screen.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventKey:
			if ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyCtrlC {
				return
			}
		}
	}
}

func drawText(s tcell.Screen, x, y int, style tcell.Style, text string) {
	for i, r := range text {
		s.SetContent(x+i, y, r, nil, style)
	}
}
