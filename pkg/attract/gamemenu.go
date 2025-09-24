package attract

import (
	"bufio"
	"fmt"
	"time"
	"os"
	"os/exec"  
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

// MENU 9: Reset to menu, spam F9, then run SAM_MENU.sh
func GameMenu9() error {
    // Reset to MiSTer menu core
    if err := os.WriteFile("/dev/MiSTer_cmd", []byte("load_core /media/fat/menu.rbf\n"), 0644); err != nil {
        return fmt.Errorf("failed to reset to menu core: %w", err)
    }
    fmt.Println("[MENU9] Reset to MiSTer menu core.")

    // Virtual keyboard
    kb, err := input.NewVirtualKeyboard()
    if err != nil {
        return fmt.Errorf("failed to create virtual keyboard: %w", err)
    }
    defer kb.Close()

    // Spam F9 a few times to ensure console opens
    fmt.Println("[MENU9] Spamming F9 to drop into console...")
    for i := 0; i < 3; i++ {
        kb.Console()
        time.Sleep(200 * time.Millisecond)
    }

    // Run the SAM menu script
    scriptPath := "/media/fat/Scripts/SAM_MENU.sh"
    if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
        return fmt.Errorf("script not found: %s", scriptPath)
    }

    cmd := exec.Command("bash", scriptPath)
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.Stdin = os.Stdin

    if err := cmd.Run(); err != nil {
        return fmt.Errorf("failed to run script: %w", err)
    }

    fmt.Println("[MENU9] SAM_MENU.sh finished.")
    return nil
}

