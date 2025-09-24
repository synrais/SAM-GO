package attract

import (
	"bufio"
	"fmt"
	"os"
	"strings"
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

// MENU 9: launch SAM_MENU.sh script
func GameMenu9() error {
	script := "/media/fat/Scripts/SAM_MENU.sh"

	fmt.Println("=== TEST MENU 9 ===")
	fmt.Println("Launching:", script)

	cmd := exec.Command(script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run %s: %w", script, err)
	}

	return nil
}
