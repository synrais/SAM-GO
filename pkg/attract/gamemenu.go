package attract

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/input"
)

// ===== Menus 1–8 (NOT PLACEHOLDERS!!) =====

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

	time.Sleep(3 * time.Second)

	// Step 3: press F9 (open terminal)
	kb, err := input.NewVirtualKeyboard()
	if err != nil {
		return fmt.Errorf("[DEBUG] failed to create virtual keyboard: %w", err)
	}
	defer kb.Close()

	if err := kb.Console(); err != nil {
		return fmt.Errorf("[DEBUG] failed to press F9: %w", err)
	}
	time.Sleep(2 * time.Second)

	// Step 4: switch to tty2
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := exec.CommandContext(ctx, "chvt", "2").Run(); err != nil {
		return fmt.Errorf("[DEBUG] failed to switch to tty2: %w", err)
	}

	// Step 5: redirect stdio to tty2 and run internal menu
	tty, err := os.OpenFile("/dev/tty2", os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("[DEBUG] failed to open /dev/tty2: %w", err)
	}
	os.Stdout = tty
	os.Stderr = tty
	os.Stdin = tty

	RunMenu()
	return nil
}

// ===== File/Dir Struct =====

type Node struct {
	Display string
	Path    string
	IsDir   bool
}

func (n Node) Title() string {
	if n.IsDir {
		return "📂 " + n.Display
	}
	return "🎮 " + n.Display
}

func (n Node) Description() string { return "" } // clean, no paths
func (n Node) FilterValue() string { return n.Display }

// ===== Styles =====

var (
	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			Padding(1, 2).
			BorderForeground(lipgloss.Color("33"))

	selectedItem = lipgloss.NewStyle().
			Foreground(lipgloss.Color("0")).
			Background(lipgloss.Color("39")).
			Bold(true)

	normalItem = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))
)

// ===== Menu Model =====

type menuModel struct {
	list    list.Model
	stack   []string
	systems map[string][]Node
	cursor  int
}

func newMenuModel() menuModel {
	systems := buildSystemsFromCache()

	// root list = all systems
	var systemNodes []Node
	for sys := range systems {
		systemNodes = append(systemNodes, Node{
			Display: sys,
			Path:    sys,
			IsDir:   true,
		})
	}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = selectedItem
	delegate.Styles.NormalTitle = normalItem
	delegate.Styles.SelectedDesc = selectedItem
	delegate.Styles.NormalDesc = normalItem

	l := list.New(toListItems(systemNodes), delegate, 0, 0) // auto-resize
	l.Title = "📜 SAM Masterlist Browser"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)

	return menuModel{list: l, stack: []string{}, systems: systems, cursor: 0}
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
				if games, ok := m.systems[sel.Path]; ok {
					children := append([]Node{{Display: ".. Back", Path: "..", IsDir: true}}, games...)
					m.list.SetItems(toListItems(children))
					m.list.Title = sel.Display
					m.stack = append(m.stack, sel.Path)
				}
			} else {
				fmt.Printf("[MENU] Launching: %s\n", sel.Path)
				Run([]string{sel.Path})
			}
		case "q", "esc":
			return m.goBack(), nil
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m menuModel) View() string {
	return borderStyle.Render(m.list.View())
}

func (m menuModel) goBack() menuModel {
	if len(m.stack) == 0 {
		var systemNodes []Node
		for sys := range m.systems {
			systemNodes = append(systemNodes, Node{
				Display: sys,
				Path:    sys,
				IsDir:   true,
			})
		}
		m.list.SetItems(toListItems(systemNodes))
		m.list.Title = "📜 SAM Masterlist Browser"
		return m
	}
	m.stack = m.stack[:len(m.stack)-1]
	system := m.stack[len(m.stack)-1]
	children := append([]Node{{Display: ".. Back", Path: "..", IsDir: true}}, m.systems[system]...)
	m.list.SetItems(toListItems(children))
	m.list.Title = system
	return m
}

// ===== Helpers =====

func buildSystemsFromCache() map[string][]Node {
	systems := make(map[string][]Node)
	lines := FlattenCache("master")
	currentSystem := ""

	for _, line := range lines {
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "# SYSTEM:") {
			system := strings.TrimSpace(line[len("# SYSTEM:"):])
			currentSystem = system
			if _, ok := systems[currentSystem]; !ok {
				systems[currentSystem] = []Node{}
			}
			continue
		}
		if currentSystem != "" {
			game := strings.TrimSuffix(filepath.Base(line), filepath.Ext(line))
			systems[currentSystem] = append(systems[currentSystem], Node{
				Display: game,
				Path:    line,
				IsDir:   false,
			})
		}
	}
	return systems
}

func toListItems(nodes []Node) []list.Item {
	out := make([]list.Item, len(nodes))
	for i, n := range nodes {
		out[i] = n
	}
	return out
}

// ===== Entry Points =====

func RunMenu() {
	p := tea.NewProgram(newMenuModel(), tea.WithAltScreen())
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
