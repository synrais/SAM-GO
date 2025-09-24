package attract

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"
)

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

func (n Node) Description() string { return n.Path }
func (n Node) FilterValue() string { return n.Display }

// ===== Styles =====

var (
	// Basic UI Styles
	borderStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2).Foreground(lipgloss.Color("12"))
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	selectedItem = lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57"))

	// Header Styles
	blueBgStyle = lipgloss.NewStyle().Background(lipgloss.Color("33")).Foreground(lipgloss.Color("255")).Bold(true)
)


// ===== Menu Model =====

type menuModel struct {
	list   list.Model
	stack  []string
	root   string
	cursor int
}

func newMenuModel(root string) menuModel {
	items := buildRootItems(root)
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = selectedItem
	delegate.Styles.SelectedDesc = selectedItem

	l := list.New(toListItems(items), delegate, 50, 20)
	l.Title = "SAM File Browser"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(true)
	l.SetShowPagination(true)

	return menuModel{list: l, stack: []string{}, root: root, cursor: 0}
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
				if sel.Display == ".. Back" {
					return m.goBack(), nil
				}
				m.stack = append(m.stack, sel.Path)
				children := listDir(sel.Path)
				children = append([]Node{{Display: ".. Back", Path: "..", IsDir: true}}, children...)
				m.list.SetItems(toListItems(children))
				m.list.Title = filepath.Base(sel.Path)
			} else {
				fmt.Printf("[MENU] Launching: %s\n", sel.Path)
				Run([]string{sel.Path})
			}
		case "q", "esc":
			return m.goBack(), nil
		case "up":
			m.cursor--
			if m.cursor < 0 {
				m.cursor = len(m.list.Items) - 1
			}
		case "down":
			m.cursor++
			if m.cursor >= len(m.list.Items) {
				m.cursor = 0
			}
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
		return m
	}
	m.stack = m.stack[:len(m.stack)-1]
	if len(m.stack) == 0 {
		items := buildRootItems(m.root)
		m.list.SetItems(toListItems(items))
		m.list.Title = "SAM File Browser"
	} else {
		parent := m.stack[len(m.stack)-1]
		children := listDir(parent)
		children = append([]Node{{Display: ".. Back", Path: "..", IsDir: true}}, children...)
		m.list.SetItems(toListItems(children))
		m.list.Title = filepath.Base(parent)
	}
	return m
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
