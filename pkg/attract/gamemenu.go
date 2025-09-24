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

// ===== File/Dir Struct =====

type Node struct {
	Display string
	Path    string
	IsDir   bool
}

func (n Node) Title() string       { return n.Display }
func (n Node) Description() string { if n.IsDir { return "[DIR]" } else { return n.Path } }
func (n Node) FilterValue() string { return n.Display }

// ===== Menu Model =====

type menuModel struct {
	list  list.Model
	stack []string
	root  string
}

func newMenuModel(root string) menuModel {
	items := buildRootItems(root)
	l := list.New(toListItems(items), list.NewDefaultDelegate(), 50, 20)
	l.Title = "SAM File Browser"
	return menuModel{list: l, stack: []string{}, root: root}
}

func (m menuModel) Init() tea.Cmd { return nil }

func (m menuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			sel, ok := m.list.SelectedItem().(Node)
			if !ok {
				return m, nil
			}
			if sel.IsDir {
				m.stack = append(m.stack, sel.Path)
				children := listDir(sel.Path)
				m.list.SetItems(toListItems(children))
				m.list.Title = sel.Display
			} else {
				fmt.Printf("[MENU] Launching: %s\n", sel.Path)
				Run([]string{sel.Path})
			}
		case "q", "esc":
			if len(m.stack) == 0 {
				return m, tea.Quit
			}
			m.stack = m.stack[:len(m.stack)-1]
			if len(m.stack) == 0 {
				items := buildRootItems(m.root)
				m.list.SetItems(toListItems(items))
				m.list.Title = "SAM File Browser"
			} else {
				parent := m.stack[len(m.stack)-1]
				children := listDir(parent)
				m.list.SetItems(toListItems(children))
				m.list.Title = filepath.Base(parent)
			}
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m menuModel) View() string {
	return m.list.View()
}

// ===== Helpers =====

func buildRootItems(root string) []Node {
	var items []Node
	entries, _ := filepath.Glob(filepath.Join(root, "*_gamelist.txt"))
	for _, e := range entries {
		lines := readLines(e)
		for _, line := range lines {
			if line == "" {
				continue
			}
			dir := filepath.Dir(line)
			if !contains(items, dir) {
				items = append(items, Node{
					Display: filepath.Base(dir),
					Path:    dir,
					IsDir:   true,
				})
			}
		}
	}
	return items
}

func listDir(path string) []Node {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil
	}
	var nodes []Node
	for _, e := range entries {
		nodes = append(nodes, Node{
			Display: e.Name(),
			Path:    filepath.Join(path, e.Name()),
			IsDir:   e.IsDir(),
		})
	}
	return nodes
}

func toListItems(nodes []Node) []list.Item {
	var out []list.Item
	for _, n := range nodes {
		out = append(out, n)
	}
	return out
}

func contains(nodes []Node, path string) bool {
	for _, n := range nodes {
		if n.Path == path {
			return true
		}
	}
	return false
}

func readLines(file string) []string {
	f, err := os.Open(file)
	if err != nil {
		return nil
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, strings.TrimSpace(sc.Text()))
	}
	return lines
}

// ===== Entry Points =====

func RunMenu() {
	root := "/media/fat/Scripts/.MiSTer_SAM/SAM_Gamelists"
	p := tea.NewProgram(newMenuModel(root), tea.WithAltScreen())
	if err := p.Start(); err != nil {
		fmt.Println("Error running menu:", err)
		os.Exit(1)
	}
}

func LaunchMenu(cfg *config.UserConfig) error {
	fmt.Println("[DEBUG] LaunchMenu() called")
	RunMenu()
	return nil
}
