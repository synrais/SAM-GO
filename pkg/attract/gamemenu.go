package attract

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/input"
)

//
// ===== Menus 1–8 (real handlers can be added) =====
//

func GameMenu1() error { fmt.Println("[DEBUG] GameMenu1 called"); return nil }
func GameMenu2() error { fmt.Println("[DEBUG] GameMenu2 called"); return nil }
func GameMenu3() error { fmt.Println("[DEBUG] GameMenu3 called"); return nil }
func GameMenu4() error { fmt.Println("[DEBUG] GameMenu4 called"); return nil }
func GameMenu5() error { fmt.Println("[DEBUG] GameMenu5 called"); return nil }
func GameMenu6() error { fmt.Println("[DEBUG] GameMenu6 called"); return nil }
func GameMenu7() error { fmt.Println("[DEBUG] GameMenu7 called"); return nil }
func GameMenu8() error { fmt.Println("[DEBUG] GameMenu8 called"); return nil }

//
// ===== Menu 9 (special: switch to tty2 and run internal menu) =====
//

func GameMenu9() error {
	fmt.Println("[DEBUG] Entered GameMenu9()")

	// Step 1: reload MiSTer menu core
	cmdPath := "/dev/MiSTer_cmd"
	if err := os.WriteFile(cmdPath, []byte("load_core /media/fat/menu.rbf\n"), 0644); err != nil {
		return fmt.Errorf("[DEBUG] failed to reload menu core: %w", err)
	}
	time.Sleep(3 * time.Second)

	// Step 2: press F9 (open terminal)
	kb, err := input.NewVirtualKeyboard()
	if err != nil {
		return fmt.Errorf("[DEBUG] failed to create virtual keyboard: %w", err)
	}
	defer kb.Close()
	if err := kb.Console(); err != nil {
		return fmt.Errorf("[DEBUG] failed to press F9: %w", err)
	}
	time.Sleep(2 * time.Second)

	// Step 3: switch to tty2
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := exec.CommandContext(ctx, "chvt", "2").Run(); err != nil {
		return fmt.Errorf("[DEBUG] failed to switch to tty2: %w", err)
	}

	// Step 4: redirect stdio to tty2 and run internal menu
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

//
// ===== File/Dir Struct =====
//

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

//
// ===== Styles =====
//

var (
	selectedItem = lipgloss.NewStyle().
			Foreground(lipgloss.Color("230")).
			Background(lipgloss.Color("62")).
			Bold(true).
			PaddingLeft(1)

	normalItem = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			PaddingLeft(1)

	titleBarStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("230")).
			Background(lipgloss.Color("57")).
			Padding(1, 2)

	breadcrumbStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("180")).
			Background(lipgloss.Color("57")).
			Padding(0, 2, 1, 2)

	frameStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("105")).
			Padding(1, 2)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("228")).
			Background(lipgloss.Color("60")).
			Padding(0, 2)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("249")).
			Background(lipgloss.Color("55")).
			Padding(1, 2)
)

type keyMap struct {
	up, down, enter, back, quit, filter key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
		enter: key.NewBinding(
			key.WithKeys("enter", "right", "l"),
			key.WithHelp("enter", "open / launch"),
		),
		back: key.NewBinding(
			key.WithKeys("esc", "left", "h", "backspace"),
			key.WithHelp("esc", "go back"),
		),
		quit: key.NewBinding(
			key.WithKeys("ctrl+c", "q"),
			key.WithHelp("ctrl+c", "quit"),
		),
		filter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter"),
		),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.up, k.down, k.enter, k.back, k.quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.up, k.down, k.enter},
		{k.back, k.filter, k.quit},
	}
}

//
// ===== Menu Model =====
//

type menuModel struct {
	list    list.Model
	stack   []string
	systems map[string][]Node
	keys    keyMap
	help    help.Model
	width   int
	height  int
	ready   bool
	status  string
}

func newMenuModel() menuModel {
	systems := buildSystemsFromCache()

	// root list = all systems
	systemNodes := buildRootNodes(systems)

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = selectedItem
	delegate.Styles.NormalTitle = normalItem
	delegate.Styles.SelectedDesc = selectedItem
	delegate.Styles.NormalDesc = normalItem

	// width/height = 0, will be resized on WindowSizeMsg
	l := list.New(toListItems(systemNodes), delegate, 0, 0)
	l.Title = "📜 SAM Masterlist Browser"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)
	l.SetShowPagination(false)

	styles := list.DefaultStyles()
	styles.Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Padding(0, 1)
	styles.PaginationStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("213")).
		PaddingLeft(1)
	styles.HelpStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("189"))
	styles.NoItems = lipgloss.NewStyle().
		Foreground(lipgloss.Color("213")).
		Italic(true)
	styles.FilterPrompt = lipgloss.NewStyle().
		Foreground(lipgloss.Color("229")).
		Bold(true)
	styles.FilterCursor = lipgloss.NewStyle().
		Foreground(lipgloss.Color("229")).
		Bold(true)
	l.Styles = styles

	keymap := newKeyMap()
	helpModel := help.New()
	helpModel.ShortSeparator = " • "

	return menuModel{list: l, stack: []string{}, systems: systems, keys: keymap, help: helpModel, status: "Select a system to browse its games."}
}

func (m menuModel) Init() tea.Cmd { return nil }

func (m menuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.list.SetSize(max(msg.Width-6, 20), max(msg.Height-10, 5))
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			sel, ok := m.list.SelectedItem().(Node)
			if !ok {
				return m, nil
			}
			if sel.Path == ".." {
				return m.goBack(), nil
			}
			if sel.IsDir {
				if games, ok := m.systems[sel.Path]; ok {
					children := append([]Node{{Display: ".. Back", Path: "..", IsDir: true}}, games...)
					m.list.SetItems(toListItems(children))
					m.list.Title = sel.Display
					m.stack = append(m.stack, sel.Path)
					m.status = fmt.Sprintf("%d games available", len(children)-1)
				}
			} else {
				fmt.Printf("[MENU] Launching: %s\n", sel.Path)
				Run([]string{sel.Path})
			}
		case "q", "ctrl+c":
			if len(m.stack) == 0 {
				return m, tea.Quit
			}
			return m.goBack(), nil
		case "esc", "backspace", "left", "h":
			if len(m.stack) == 0 {
				return m, nil
			}
			return m.goBack(), nil
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m menuModel) View() string {
	if !m.ready {
		return "Loading browser…"
	}

	header := titleBarStyle.Render("SAM ✦ Game Browser")
	breadcrumb := breadcrumbStyle.Render(m.breadcrumb())
	body := frameStyle.
		Width(max(m.width-4, 20)).
		Height(max(m.height-8, 10)).
		Render(m.list.View())
	footer := lipgloss.JoinVertical(lipgloss.Left,
		statusStyle.Render(m.status),
		helpStyle.Render(m.help.View(m.keys)),
	)

	return lipgloss.JoinVertical(lipgloss.Left, header, breadcrumb, body, footer)
}

func (m menuModel) goBack() menuModel {
	if len(m.stack) == 0 {
		systemNodes := buildRootNodes(m.systems)
		m.list.SetItems(toListItems(systemNodes))
		m.list.Title = "Systems"
		m.status = "Select a system to browse its games."
		return m
	}

	parentStack := m.stack[:len(m.stack)-1]
	if len(parentStack) == 0 {
		m.stack = parentStack
		systemNodes := buildRootNodes(m.systems)
		m.list.SetItems(toListItems(systemNodes))
		m.list.Title = "Systems"
		m.status = "Select a system to browse its games."
		return m
	}

	m.stack = parentStack
	system := m.stack[len(m.stack)-1]
	children := append([]Node{{Display: ".. Back", Path: "..", IsDir: true}}, m.systems[system]...)
	m.list.SetItems(toListItems(children))
	m.list.Title = system
	m.status = fmt.Sprintf("%d games available", len(children)-1)
	return m
}

func (m menuModel) breadcrumb() string {
	if len(m.stack) == 0 {
		return "Systems"
	}
	return strings.Join(m.stack, " › ")
}

//
// ===== Helpers =====
//

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

	for system, games := range systems {
		sort.Slice(games, func(i, j int) bool {
			return strings.ToLower(games[i].Display) < strings.ToLower(games[j].Display)
		})
		systems[system] = games
	}

	return systems
}

func buildRootNodes(systems map[string][]Node) []Node {
	var names []string
	for sys := range systems {
		names = append(names, sys)
	}
	sort.Slice(names, func(i, j int) bool {
		return strings.ToLower(names[i]) < strings.ToLower(names[j])
	})

	nodes := make([]Node, 0, len(names))
	for _, name := range names {
		nodes = append(nodes, Node{Display: name, Path: name, IsDir: true})
	}
	return nodes
}

func toListItems(nodes []Node) []list.Item {
	out := make([]list.Item, len(nodes))
	for i, n := range nodes {
		out[i] = n
	}
	return out
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

//
// ===== Entry Points =====
//

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
