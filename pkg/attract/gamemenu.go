package attract

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

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

// ===== Menu 9 (special, spawns tty2) =====
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
	fmt.Println("[DEBUG] Sleeping 2s to let menu reload…")
	time.Sleep(2 * time.Second)

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

	// Step 4: switch to tty2
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	fmt.Println("[DEBUG] Running chvt 2")
	if err := exec.CommandContext(ctx, "chvt", "2").Run(); err != nil {
		return fmt.Errorf("[DEBUG] failed to switch to tty2: %w", err)
	}
	fmt.Println("[DEBUG] Successfully switched to tty2")

	// Step 5: open tty2 and exec SAM -menu on it
	fmt.Println("[DEBUG] Opening /dev/tty2…")
	tty, err := os.OpenFile("/dev/tty2", os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("[DEBUG] failed to open /dev/tty2: %w", err)
	}
	defer tty.Close()

	cmd := exec.Command("/media/fat/Scripts/.MiSTer_SAM/SAM", "-menu")
	cmd.Stdin = tty
	cmd.Stdout = tty
	cmd.Stderr = tty
	// no SysProcAttr fiddling — Linux will attach tty2 automatically

	fmt.Println("[DEBUG] Starting SAM -menu on tty2…")
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("[DEBUG] failed to run SAM -menu on tty2: %w", err)
	}

	fmt.Printf("[DEBUG] SAM menu started on tty2 (PID %d)\n", cmd.Process.Pid)
	return nil
}

// ===== Direct in-RAM Menu =====

func RunMenu() {
	fmt.Println("[DEBUG] Entered RunMenu()")

	systems := CacheKeys("master")
	if len(systems) == 0 {
		fmt.Println("[MENU] No gamelists available in memory")
		return
	}

	for {
		fmt.Println("==== Systems ====")
		for i, sys := range systems {
			fmt.Printf("%2d) %s\n", i+1, sys)
		}

		var sysChoice int
		fmt.Print("Choose a system (0 to quit): ")
		fmt.Scanln(&sysChoice)
		if sysChoice == 0 {
			return
		}
		if sysChoice < 1 || sysChoice > len(systems) {
			fmt.Println("[MENU] Invalid system choice")
			continue
		}
		chosenSys := systems[sysChoice-1]

		games := GetCache("master", chosenSys)
		if len(games) == 0 {
			fmt.Printf("[MENU] No games found for %s\n", chosenSys)
			continue
		}

		for {
			fmt.Printf("==== %s Games ====\n", chosenSys)
			for i, g := range games {
				base := filepath.Base(g)
				name := strings.TrimSuffix(base, filepath.Ext(base))
				if len(name) > 70 {
					name = name[:67] + "..."
				}
				fmt.Printf("%4d) %s\n", i+1, name)
			}

			var gameChoice int
			fmt.Print("Choose a game (0 to go back): ")
			fmt.Scanln(&gameChoice)
			if gameChoice == 0 {
				break
			}
			if gameChoice < 1 || gameChoice > len(games) {
				fmt.Println("[MENU] Invalid game choice")
				continue
			}

			chosenGame := games[gameChoice-1]
			fmt.Printf("[MENU] Launching: %s\n", chosenGame)
			Run([]string{chosenGame}) // direct call into attract.Run()
		}
	}
}

// Entry point for `SAM -menu`
func LaunchMenu(cfg *config.UserConfig) error {
	fmt.Println("[DEBUG] LaunchMenu() called")
	RunMenu()
	return nil
}
