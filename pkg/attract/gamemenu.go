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

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
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

	cmdPath := "/dev/MiSTer_cmd"
	fmt.Printf("[DEBUG] Writing reload command to %s\n", cmdPath)
	if err := os.WriteFile(cmdPath, []byte("load_core /media/fat/menu.rbf\n"), 0644); err != nil {
		return fmt.Errorf("[DEBUG] failed to reload menu core: %w", err)
	}
	fmt.Println("[DEBUG] Reload command written successfully")

	fmt.Println("[DEBUG] Sleeping 3s to let menu reload…")
	time.Sleep(3 * time.Second)

	fmt.Println("[DEBUG] Creating virtual keyboard…")
	kb, err := input.NewVirtualKeyboard()
	if err != nil {
		return fmt.Errorf("[DEBUG] failed to create virtual keyboard: %w", err)
	}
	defer kb.Close()

	fmt.Println("[DEBUG] Sending Console() → F9")
	if err := kb.Console(); err != nil {
		return fmt.Errorf("[DEBUG] failed to press F9: %w", err)
	}
	fmt.Println("[DEBUG] F9 pressed")

	fmt.Println("[DEBUG] Sleeping 2s after F9…")
	time.Sleep(2 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	fmt.Println("[DEBUG] Running chvt 2")
	if err := exec.CommandContext(ctx, "chvt", "2").Run(); err != nil {
		return fmt.Errorf("[DEBUG] failed to switch to tty2: %w", err)
	}
	fmt.Println("[DEBUG] Successfully switched to tty2")

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

// ===== Game struct =====

type Game struct {
	Display string
	Path    string
}

func (g Game) Title() string       { return g.Display }
func (g Game) Description() string { return g.Path }
func (g Game) FilterValue() string { return g.Display }

// ===== Bubbletea Menu Model =====

type menuModel struct {
	list     list.Model
	root     string
	system   string
	isSystem bool
}

func newMenuModel(root string, isSystem bool, system string) menuModel {
	var items []list.Item
	if isSystem {
		for _, s := range listSystems(root) {
			items = append(items, Game{Display: s, Path: s})
		}
	} else {
		for _, g := range listGames(root, system) {
			items = append(items, g)
		}
	}
	l := list.New(items, list.NewDefaultDelegate(), 40, 15)
	if isSystem {
		l.Title = "==== Systems ===="
	} else {
		l.Title = fmt.Sprintf("==== %s Games ====", system)
	}
	return menuModel{list: l, root: root, system: system, isSystem: isSystem}
}

func (m menuModel) Init() tea.Cmd { return nil }

func (m menuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			selected, ok := m.list.SelectedItem().(Game)
			if !ok {
				return m, nil
			}
			if m.isSystem {
				return newMenuModel(m.root, false, selected.Display), nil
			}
			fmt.Printf("[MENU] Launching: %s\n", selected.Path)
			Run([]string{selected.Path})
			return m, nil
		case "q", "esc":
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m menuModel) View() string {
	return m.list.View()
}

// ===== Pretty Menu Entry Point =====

func RunMenu() {
	root := "/media/fat/Scripts/.MiSTer_SAM/SAM_Gamelists"
	p := tea.NewProgram(newMenuModel(root, true, ""), tea.WithAltScreen())
	if err := p.Start(); err != nil {
		fmt.Println("Error running menu:", err)
		os.Exit(1)
	}
}

// ===== Helpers =====

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

// ===== Entry Point for `SAM -menu` =====

func LaunchMenu(cfg *config.UserConfig) error {
	fmt.Println("[DEBUG] LaunchMenu() called")
	RunMenu()
	return nil
}
