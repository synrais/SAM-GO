package attract

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// getTTY reads the active tty from sysfs
func getTTY() (string, error) {
	sys := "/sys/devices/virtual/tty/tty0/active"
	tty, err := os.ReadFile(sys)
	if err != nil {
		return "", fmt.Errorf("getTTY: %w", err)
	}
	return strings.TrimSpace(string(tty)), nil
}

// pressF9 emulates F9 keypress (using evemu or input-event injection)
func pressF9() error {
	// On MiSTer this is usually handled by zaparoo’s KeyboardPress.
	// Simplest approach: use "chvt 1" after forcing tty3, since F9 maps to it.
	cmd := exec.Command("chvt", "1")
	return cmd.Run()
}

// openConsole forces MiSTer to give us tty1 via F9 keypress
func openConsole() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// switch to tty3 (unused)
	if err := exec.CommandContext(ctx, "chvt", "3").Run(); err != nil {
		return fmt.Errorf("chvt 3 failed: %w", err)
	}

	// spam F9 (or just chvt 1) until tty1 becomes active
	for i := 0; i < 10; i++ {
		if err := pressF9(); err != nil {
			return err
		}
		time.Sleep(100 * time.Millisecond)

		tty, err := getTTY()
		if err == nil && tty == "tty1" {
			return nil
		}
	}
	return errors.New("could not switch to tty1 via F9")
}

// GameMenu9 launches SAM_MENU.sh through MiSTer’s tty2 mechanism
func GameMenu9() error {
	script := "/media/fat/Scripts/SAM_MENU.sh"

	// First force console switch like F9 would
	if err := openConsole(); err != nil {
		return err
	}

	// Prepare /tmp/script launcher
	launcher := fmt.Sprintf(`#!/bin/bash
export LC_ALL=en_US.UTF-8
export HOME=/root
export LESSKEY=/media/fat/linux/lesskey
export ZAPAROO_RUN_SCRIPT=1
cd $(dirname "%s")
%s
`, script, script)

	tmpScript := "/tmp/script"
	if err := os.WriteFile(tmpScript, []byte(launcher), 0o750); err != nil {
		return fmt.Errorf("failed to write launcher: %w", err)
	}

	// Switch to tty2 (MiSTer convention for scripts)
	if err := exec.Command("chvt", "2").Run(); err != nil {
		return fmt.Errorf("failed chvt 2: %w", err)
	}

	// Run agetty to execute script as login shell
	cmd := exec.Command("/sbin/agetty",
		"-a", "root",
		"-l", tmpScript,
		"--nohostname", "-L",
		"tty2", "linux",
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Println("[MENU9] Launching SAM_MENU.sh via tty2 + agetty…")
	return cmd.Run()
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
