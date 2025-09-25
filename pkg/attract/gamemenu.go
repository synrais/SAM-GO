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
