package attract

import (
	"bufio"
	"fmt"
	"time"
	"os"
	"strings"

	"github.com/synrais/SAM-GO/pkg/input"
)

// waitKey reads a line (blocking)
func waitKey() string {
	reader := bufio.NewReader(os.Stdin)
	text, _ := reader.ReadString('\n')
	return strings.TrimSpace(text)
}

// MENU 1: one-shot message
func GameMenu1() error {
	fmt.Println("=== TEST MENU 1 ===")
	fmt.Println("Press Enter to exit.")
	waitKey()
	return nil
}

// MENU 2: static list
func GameMenu2() error {
	fmt.Println("=== TEST MENU 2 ===")
	fmt.Println("1) Option A\n2) Option B\nq) Quit")
	choice := waitKey()
	fmt.Println("You picked:", choice)
	return nil
}

// MENU 3: system/game browser (hardcoded)
func GameMenu3() error {
	systems := []string{"NES", "SNES", "Genesis"}
	games := map[string][]string{
		"NES":     {"Mario", "Zelda"},
		"SNES":    {"SMW", "DKC"},
		"Genesis": {"Sonic", "Streets of Rage"},
	}
	fmt.Println("Pick a system:", systems)
	sys := waitKey()
	if g, ok := games[sys]; ok {
		fmt.Println("Games:", g)
	} else {
		fmt.Println("Unknown system:", sys)
	}
	waitKey()
	return nil
}

// MENU 4: fake settings form
func GameMenu4() error {
	fmt.Println("=== SETTINGS ===")
	fmt.Print("Enter Name: ")
	name := waitKey()
	fmt.Print("Enable feature (y/n): ")
	enable := waitKey()
	fmt.Println("Saved", name, enable)
	return nil
}

// MENU 5: scrolling log
func GameMenu5() error {
	fmt.Println("=== LOG ===")
	for i := 1; i <= 20; i++ {
		fmt.Printf("Line %d: Demo\n", i)
	}
	waitKey()
	return nil
}

// MENU 6: table
func GameMenu6() error {
	fmt.Println("ID | Name   | Status")
	fmt.Println("1  | Mario  | OK")
	fmt.Println("2  | Zelda  | Missing")
	fmt.Println("3  | Sonic  | OK")
	waitKey()
	return nil
}

// MENU 7: pages
func GameMenu7() error {
	for {
		fmt.Println("=== PAGE MAIN === (n=next, q=quit)")
		k := waitKey()
		if k == "n" {
			fmt.Println("=== PAGE NEXT === (b=back, q=quit)")
			k2 := waitKey()
			if k2 == "b" {
				continue
			} else if k2 == "q" {
				break
			}
		} else if k == "q" {
			break
		}
	}
	return nil
}

// MENU 8: read gamelists from filesystem
func GameMenu8() error {
	root := "/media/fat/Scripts/.MiSTer_SAM/SAM_Gamelists"
	entries, err := os.ReadDir(root)
	if err != nil {
		return fmt.Errorf("error reading %s: %w", root, err)
	}

	fmt.Println("Systems:")
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), "_gamelist.txt") {
			sys := strings.TrimSuffix(e.Name(), "_gamelist.txt")
			fmt.Println("-", sys)
		}
	}
	waitKey()
	return nil
}

// MENU 9: reset to MiSTer menu, open console with F9, and run SAM_MENU.sh
func GameMenu9() error {
	// Step 1: reload menu core
	cmdPath := "/dev/MiSTer_cmd"
	f, err := os.OpenFile(cmdPath, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", cmdPath, err)
	}
	if _, err := f.WriteString("load_core /media/fat/menu.rbf\n"); err != nil {
		f.Close()
		return fmt.Errorf("failed to write to %s: %w", cmdPath, err)
	}
	f.Close()
	fmt.Println("[MENU9] Reloaded MiSTer menu core")

	// Step 2: wait a bit for menu to load
	time.Sleep(2 * time.Second)

	// Step 3: write launcher to /tmp/script
	launcher := `#!/bin/bash
export LC_ALL=en_US.UTF-8
export HOME=/root
export LESSKEY=/media/fat/linux/lesskey
cd /media/fat/Scripts
/media/fat/Scripts/SAM_MENU.sh
`
	if err := os.WriteFile("/tmp/script", []byte(launcher), 0750); err != nil {
		return fmt.Errorf("failed to write /tmp/script: %w", err)
	}
	fmt.Println("[MENU9] Launcher /tmp/script created")

	// Step 4: press F9 to switch to console (tty2) and let MiSTer auto-run /tmp/script
	kb, err := input.NewVirtualKeyboard()
	if err != nil {
		return fmt.Errorf("failed to create virtual keyboard: %w", err)
	}
	defer kb.Close()

	fmt.Println("[MENU9] Sending F9 to open console and run SAM_MENU.sh...")
	if err := kb.Console(); err != nil {
		return fmt.Errorf("failed to press F9: %w", err)
	}

	fmt.Println("[MENU9] Console should now be visible and SAM_MENU.sh running.")
	return nil
}
