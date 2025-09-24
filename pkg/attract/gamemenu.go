package attract

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// ===== Game struct =====
type Game struct {
	Display string
	Path    string
}

func (g Game) Title() string       { return g.Display }
func (g Game) Description() string { return g.Path }
func (g Game) FilterValue() string { return g.Display }

// ===== Bubbletea model =====
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
		l.Title = "Choose a System"
	} else {
		l.Title = "Choose a Game"
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
				// Jump into game list
				return newMenuModel(m.root, false, selected.Display), nil
			}
			// Launch game
			fmt.Printf("[MENU] Launching: %s\n", selected.Path)
			Run([]string{selected.Path})
			return m, nil
		case "q", "esc":
			return nil, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m menuModel) View() string {
	return m.list.View()
}

// ===== Pretty menu entry point =====
func RunMenu() {
	root := "/media/fat/Scripts/.MiSTer_SAM/SAM_Gamelists"
	p := tea.NewProgram(newMenuModel(root, true, ""), tea.WithAltScreen())
	if err := p.Start(); err != nil {
		fmt.Println("Error running menu:", err)
		os.Exit(1)
	}
}

// ===== Helpers (unchanged) =====
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
