package attract

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/synrais/SAM-GO/pkg/input"
)

// ===== Shared Utility =====

// waitKey reads a line (blocking)
func waitKey() string {
	reader := bufio.NewReader(os.Stdin)
	text, _ := reader.ReadString('\n')
	return strings.TrimSpace(text)
}

// ===== Menus 1–8 (prototype style) =====

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

// ===== Helper Functions for Menu 9 =====

func getTTY() (string, error) {
	sys := "/sys/devices/virtual/tty/tty0/active"
	if _, err := os.Stat(sys); err != nil {
		return "", fmt.Errorf("failed to stat tty active file: %w", err)
	}

	tty, err := os.ReadFile(sys)
	if err != nil {
		return "", fmt.Errorf("failed to read tty active file: %w", err)
	}

	return strings.TrimSpace(string(tty)), nil
}

func echoFile(path, s string) error {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("failed to open file for echo: %w", err)
	}
	defer f.Close()

	if _, err = f.WriteString(s); err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}
	return nil
}

func writeTty(id, s string) error {
	tty := "/dev/tty" + id
	return echoFile(tty, s)
}

func cleanConsole(vt string) error {
	if err := writeTty(vt, "\033[?25l"); err != nil {
		return err
	}
	if err := echoFile("/sys/class/graphics/fbcon/cursor_blink", "0"); err != nil {
		return err
	}
	return writeTty(vt, "\033[?17;0;0c")
}

func restoreConsole(vt string) error {
	if err := writeTty(vt, "\033[?25h"); err != nil {
		return err
	}
	return echoFile("/sys/class/graphics/fbcon/cursor_blink", "1")
}

func openConsole() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := exec.CommandContext(ctx, "chvt", "3").Run(); err != nil {
		return fmt.Errorf("failed to run chvt: %w", err)
	}

	kb, err := input.NewVirtualKeyboard()
	if err != nil {
		return fmt.Errorf("failed to create virtual keyboard: %w", err)
	}
	defer kb.Close()

	tries := 0
	for {
		if tries > 10 {
			return errors.New("openConsole: could not switch to tty1")
		}

		if err := kb.Console(); err != nil {
			return fmt.Errorf("failed to press F9: %w", err)
		}

		time.Sleep(50 * time.Millisecond)

		tty, err := getTTY()
		if err != nil {
			return err
		}
		if tty == "tty1" {
			break
		}
		tries++
	}

	return nil
}

// MENU 9: reload MiSTer menu, open console, and run SAM_MENU.sh
func GameMenu9() error {
	// Step 1: reload MiSTer menu core
	cmdPath := "/dev/MiSTer_cmd"
	if err := os.WriteFile(cmdPath, []byte("load_core /media/fat/menu.rbf\n"), 0644); err != nil {
		return fmt.Errorf("failed to reload menu core: %w", err)
	}
	fmt.Println("[MENU9] Reloaded MiSTer menu core")

	// Step 2: wait for menu reload
	time.Sleep(2 * time.Second)

	// Step 3: press F9 (open terminal)
	kb, err := input.NewVirtualKeyboard()
	if err != nil {
		return fmt.Errorf("failed to create virtual keyboard: %w", err)
	}
	defer kb.Close()

	if err := kb.Console(); err != nil {
		return fmt.Errorf("failed to press F9: %w", err)
	}
	fmt.Println("[MENU9] Sent F9 to open terminal")

	// Step 4: wait briefly for console to spawn
	time.Sleep(2 * time.Second)

	// Step 5: switch to tty2
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := exec.CommandContext(ctx, "chvt", "2").Run(); err != nil {
		return fmt.Errorf("failed to switch to tty2: %w", err)
	}
	fmt.Println("[MENU9] Switched to tty2")

	// Step 6: run SAM_MENU.sh on tty2 with agetty
	cmd := exec.CommandContext(
		context.Background(),
		"/sbin/agetty",
		"-a", "root",
		"-l", "/media/fat/Scripts/SAM_MENU.sh",
		"--nohostname",
		"-L",
		"tty2",
		"linux",
	)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to run SAM_MENU.sh via agetty: %w", err)
	}

	fmt.Println("[MENU9] SAM_MENU.sh should now be running on tty2")
	return nil
}
