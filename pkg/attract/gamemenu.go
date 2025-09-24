package attract

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/input"
)

// ===== Menus 1–8 (placeholders) =====

func GameMenu1() error { fmt.Println("[DEBUG] GameMenu1 called"); return nil }
func GameMenu2() error { fmt.Println("[DEBUG] GameMenu2 called"); return nil }
func GameMenu3() error { fmt.Println("[DEBUG] GameMenu3 called"); return nil }
func GameMenu4() error { fmt.Println("[DEBUG] GameMenu4 called"); return nil }
func GameMenu5() error { fmt.Println("[DEBUG] GameMenu5 called"); return nil }
func GameMenu6() error { fmt.Println("[DEBUG] GameMenu6 called"); return nil }
func GameMenu7() error { fmt.Println("[DEBUG] GameMenu7 called"); return nil }
func GameMenu8() error { fmt.Println("[DEBUG] GameMenu8 called"); return nil }

// ===== Menu 9 (special: switch to tty2 and run internal menu) =====

func GameMenu9() error {
	fmt.Println("[DEBUG] Entered GameMenu9()")

	// Step 1: reload MiSTer menu core
	cmdPath := "/dev/MiSTer_cmd"
	fmt.Printf("[DEBUG] Writing reload command to %s\n", cmdPath)
	if err := os.WriteFile(cmdPath, []byte("load_core /media/fat/menu.rbf\n"), 0644); err != nil {
		return fmt.Errorf("[DEBUG] failed to reload menu core: %w", err)
	}
	fmt.Println("[DEBUG] Reload command written successfully")

	// Step 2: wait for menu reload
	fmt.Println("[DEBUG] Sleeping 3s to let menu reload…")
	time.Sleep(3 * time.Second)

	// Step 3: press F9 (open terminal)
	fmt.Println("[DEBUG] Creating virtual keyboard…")
	kb, err := input.NewVirtualKeyboard()
	if err != nil {
		return fmt.Errorf("[DEBUG] failed to create virtual keyboard: %w", err)
	}
	defer kb.Close()
	fmt.Println("[DEBUG] Virtual keyboard ready")

	fmt.Println("[DEBUG] Sending Console() → F9")
	if err := kb.Console(); err != nil {
		return fmt.Errorf("[DEBUG] failed to press F9: %w", err)
	}
	fmt.Println("[DEBUG] F9 pressed")

	fmt.Println("[DEBUG] Sleeping 3s for terminal…")
	time.Sleep(3 * time.Second)

	// Step 4: switch to tty2
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	fmt.Println("[DEBUG] Running chvt 2")
	if err := exec.CommandContext(ctx, "chvt", "2").Run(); err != nil {
		return fmt.Errorf("[DEBUG] failed to switch to tty2: %w", err)
	}
	fmt.Println("[DEBUG] Successfully switched to tty2")

	// Step 5: hand off to RunMenu
	fmt.Println("[DEBUG] Launching RunMenu() on tty2")
	RunMenu()
	return nil
}

// ===== Direct in-RAM Menu (tview) =====

func RunMenu() {
	allMaster := FlattenCache("master")
	if len(allMaster) == 0 {
		fmt.Println("[MENU] No games available in master list (RAM empty?)")
		return
	}

	// force TERM so tcell knows what to load
	os.Setenv("TERM", "linux")

	// open tty2 directly
	tty, err := tcell.NewDevTtyFromDev("/dev/tty2")
	if err != nil {
		fmt.Printf("[MENU] Failed to open tty2: %v\n", err)
		return
	}
	defer tty.Close()

	screen, err := tcell.NewTerminfoScreenFromTty(tty)
	if err != nil {
		fmt.Printf("[MENU] Failed to create screen: %v\n", err)
		return
	}
	if err := screen.Init(); err != nil {
		fmt.Printf("[MENU] Failed to init screen: %v\n", err)
		return
	}
	defer screen.Fini()

	app := tview.NewApplication()
	list := tview.NewList().
		ShowSecondaryText(false).
		SetHighlightFullLine(true)

	// Fill with master list
	for _, g := range allMaster {
		base := filepath.Base(g)
		name := strings.TrimSuffix(base, filepath.Ext(base))
		if len(name) > 70 {
			name = name[:67] + "..."
		}
		gamePath := g
		list.AddItem(name, "", 0, func() {
			app.Stop()
			fmt.Printf("[MENU] Launching: %s\n", gamePath)
			Run([]string{gamePath})
		})
	}

	list.SetDoneFunc(func() {
		app.Stop()
	})

	// exit on ESC
	list.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		if ev.Key() == tcell.KeyEscape {
			app.Stop()
			return nil
		}
		return ev
	})

	if err := app.SetScreen(screen).SetRoot(list, true).Run(); err != nil {
		fmt.Printf("[MENU] Failed to start TUI: %v\n", err)
	}
}

// Entry point for `SAM -menu`
func LaunchMenu(cfg *config.UserConfig) error {
	fmt.Println("[DEBUG] LaunchMenu() called")
	RunMenu()
	return nil
}
