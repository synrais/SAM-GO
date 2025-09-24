package attract

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/synrais/SAM-GO/pkg/input"
)

// ===== Menus 1–8 (placeholders) =====

func GameMenu1() error { fmt.Println("Menu 1 placeholder"); return nil }
func GameMenu2() error { fmt.Println("Menu 2 placeholder"); return nil }
func GameMenu3() error { fmt.Println("Menu 3 placeholder"); return nil }
func GameMenu4() error { fmt.Println("Menu 4 placeholder"); return nil }
func GameMenu5() error { fmt.Println("Menu 5 placeholder"); return nil }
func GameMenu6() error { fmt.Println("Menu 6 placeholder"); return nil }
func GameMenu7() error { fmt.Println("Menu 7 placeholder"); return nil }
func GameMenu8() error { fmt.Println("Menu 8 placeholder"); return nil }

// ===== Menu 9 =====

func GameMenu9() error {
	// Step 1: reload MiSTer menu core
	cmdPath := "/dev/MiSTer_cmd"
	if err := os.WriteFile(cmdPath, []byte("load_core /media/fat/menu.rbf\n"), 0644); err != nil {
		return fmt.Errorf("failed to reload menu core: %w", err)
	}
	fmt.Println("[MENU9] Reloaded MiSTer menu core")

	// Step 2: wait for menu reload (grace period)
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

	// Step 4: switch to tty2
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := exec.CommandContext(ctx, "chvt", "2").Run(); err != nil {
		return fmt.Errorf("failed to switch to tty2: %w", err)
	}
	fmt.Println("[MENU9] Switched to tty2")

	// Step 5: run SAM_MENU.sh on tty2 with agetty
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
