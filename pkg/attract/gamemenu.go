package attract

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/synrais/SAM-GO/pkg/input"
)

// ===== Helper Functions =====

// getTTY reads the currently active tty from Linux sysfs
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

// echoFile writes a string directly into a file
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

// writeTty writes a string directly to a tty device
func writeTty(id, s string) error {
	tty := "/dev/tty" + id
	return echoFile(tty, s)
}

// cleanConsole hides the cursor and disables blinking
func cleanConsole(vt string) error {
	if err := writeTty(vt, "\033[?25l"); err != nil {
		return err
	}
	if err := echoFile("/sys/class/graphics/fbcon/cursor_blink", "0"); err != nil {
		return err
	}
	return writeTty(vt, "\033[?17;0;0c")
}

// restoreConsole restores cursor and blinking
func restoreConsole(vt string) error {
	if err := writeTty(vt, "\033[?25h"); err != nil {
		return err
	}
	return echoFile("/sys/class/graphics/fbcon/cursor_blink", "1")
}

// openConsole tries to switch MiSTer into console mode by pressing F9 repeatedly
func openConsole() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// switch to an unused tty first
	if err := exec.CommandContext(ctx, "chvt", "3").Run(); err != nil {
		return fmt.Errorf("failed to run chvt: %w", err)
	}

	// create virtual keyboard
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

		// press F9 (Console)
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

// ===== Upgraded Menu 9 =====

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

	// Step 2: build launcher script
	scriptPath := "/tmp/script"
	launcher := `#!/bin/bash
export LC_ALL=en_US.UTF-8
export HOME=/root
export LESSKEY=/media/fat/linux/lesskey
export ZAPAROO_RUN_SCRIPT=1
cd /media/fat/Scripts
/media/fat/Scripts/SAM_MENU.sh
`
	err = os.WriteFile(scriptPath, []byte(launcher), 0o750)
	if err != nil {
		return fmt.Errorf("failed to write %s: %w", scriptPath, err)
	}
	if err := os.Chown(scriptPath, 0, 0); err != nil {
		fmt.Println("[MENU9] Warning: could not chown /tmp/script:", err)
	}
	fmt.Println("[MENU9] Launcher written to /tmp/script")

	// Step 3: wait briefly for menu reload
	time.Sleep(2 * time.Second)

	// Step 4: switch to tty2 (reserved for scripts)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := exec.CommandContext(ctx, "chvt", "2").Run(); err != nil {
		return fmt.Errorf("failed to switch to tty2: %w", err)
	}

	// Step 5: clean the console
	_ = cleanConsole("2")

	// Step 6: use agetty to attach the launcher to tty2
	cmd := exec.CommandContext(
		context.Background(),
		"/sbin/agetty",
		"-a", "root",
		"-l", scriptPath,
		"--nohostname",
		"-L",
		"tty2",
		"linux",
	)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to run agetty for script: %w", err)
	}

	fmt.Println("[MENU9] SAM_MENU.sh should now be running on tty2.")
	return nil
}
