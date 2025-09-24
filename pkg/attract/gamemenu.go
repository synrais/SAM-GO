package attract

import (
	"bufio"
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

	fmt.Println("[DEBUG] Sleeping 2s after F9…")
	time.Sleep(2 * time.Second)

	// Step 4: switch to tty2
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	fmt.Println("[DEBUG] Running chvt 2")
	if err := exec.CommandContext(ctx, "chvt", "2").Run(); err != nil {
		return fmt.Errorf("[DEBUG] failed to switch to tty2: %w", err)
	}
	fmt.Println("[DEBUG] Successfully switched to tty2")

	// Step 5: redirect stdio to tty2 and run internal menu
	fmt.Println("[DEBUG] Opening /dev/tty2…")
	tty, err := os.OpenFile("/dev/tty2", os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("[DEBUG] failed to open /dev/tty2: %w", err)
	}

	os.Stdout = tty
	os.Stderr = tty
	os.Stdin = tty

	fmt.Println("[DEBUG] Handing control to RunMenu() on tty2")
	RunMenu()
	return nil
}

// ===== Simple text-based menu =====

type Game struct {
	Display string
	Path    string
}

func RunMenu() {
	fmt.Println("[DEBUG] Entered RunMenu()")

	root := "/media/fat/Scripts/.MiSTer_SAM/SAM_Gamelists"

	for {
		// --- System menu ---
		systems := listSystems(root)
		if len(systems) == 0 {
			fmt.Printf("No gamelists found in %s\n", root)
			return
		}

		fmt.Println("==== Systems ====")
		for i, sys := range systems {
			fmt.Printf("%2d) %s\n", i+1, sys)
		}
		fmt.Print("Choose a system (0 to quit): ")

		var choice int
		fmt.Scanln(&choice)
		if choice == 0 {
			return
		}
		if choice < 1 || choice > len(systems) {
			fmt.Println("Invalid choice")
			continue
		}
		system := systems[choice-1]

		// --- Game menu ---
		games := listGames(root, system)
		if len(games) == 0 {
			fmt.Printf("No games found for %s\n", system)
			continue
		}

		for {
			fmt.Printf("==== %s Games ====\n", system)
			for i, g := range games {
				fmt.Printf("%4d) %s\n", i+1, g.Display)
			}
			fmt.Print("Choose a game (0 to go back): ")

			var gChoice int
			fmt.Scanln(&gChoice)
			if gChoice == 0 {
				break
			}
			if gChoice < 1 || gChoice > len(games) {
				fmt.Println("Invalid choice")
				continue
			}

			fullpath := games[gChoice-1].Path
			fmt.Printf("[MENU] Launching: %s\n", fullpath)
			Run([]string{fullpath})
		}
	}
}

func listSystems(root string) []string {
	var systems []string
	entries, _ := filepath.Glob(filepath.Join(root, "*_gamelist.txt"))
	for _, e := range entries {
		base := filepath.Base(e)
		systems = append(systems, strings.TrimSuffix(base, "_gamelist.txt"))
	}
	return systems
}

func listGames(root, system string) []Game {
	file := filepath.Join(root, system+"_gamelist.txt")
	f, err := os.Open(file)
	if err != nil {
		return nil
	}
	defer f.Close()

	var games []Game
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		name := filepath.Base(line)
		name = strings.TrimSuffix(name, filepath.Ext(name))
		if len(name) > 70 {
			name = name[:67] + "..."
		}
		games = append(games, Game{Display: name, Path: line})
	}
	return games
}

// Entry point for `SAM -menu`
func LaunchMenu(cfg *config.UserConfig) error {
	fmt.Println("[DEBUG] LaunchMenu() called")
	RunMenu()
	return nil
}
