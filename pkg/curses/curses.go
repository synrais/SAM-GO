package curses

import (
	gc "github.com/rthornton128/goncurses"
	"os"
	"strings"
)

type Coords struct {
	Y int
	X int
}

type SetupWindowError struct {
	Ctx error
}

func (e *SetupWindowError) Error() string {
	return e.Ctx.Error()
}

func Setup() (*gc.Window, error) {
	// Esc on its own (B on a controller) should register straight away,
	// not after ncurses' default one second wait for an escape sequence.
	if os.Getenv("ESCDELAY") == "" {
		_ = os.Setenv("ESCDELAY", "25")
	}
	stdscr, err := gc.Init()
	if err != nil {
		return nil, err
	}

	gc.Echo(false)
	gc.CBreak(true)
	gc.Cursor(0)

	gc.StartColor()
	gc.InitPair(1, gc.C_BLACK, gc.C_WHITE)

	return stdscr, nil
}

func NewWindow(stdscr *gc.Window, height int, width int, title string, timeout int) (*gc.Window, error) {
	return NewWindowAt(stdscr, -1, height, width, title, timeout)
}

// NewWindowAt is NewWindow with the window's top row given (-1 = centred
// vertically).
func NewWindowAt(stdscr *gc.Window, top, height, width int, title string, timeout int) (*gc.Window, error) {
	rows, cols := stdscr.MaxYX()
	// Never bigger than the screen (ncurses refuses a window that doesn't
	// fit), and never off its edge.
	height, width = FitSize(stdscr, height, width)
	y, x := (rows-height)/2, (cols-width)/2
	if top >= 0 {
		y = top
	}
	if y+height > rows {
		y = rows - height
	}
	if y < 0 {
		y = 0
	}
	if x < 0 {
		x = 0
	}

	var win *gc.Window
	win, err := gc.NewWindow(height, width, y, x)
	if err != nil {
		return nil, &SetupWindowError{Ctx: err}
	}
	win.Keypad(true)
	win.Timeout(timeout)

	win.Erase()
	win.NoutRefresh()

	win.Box(gc.ACS_VLINE, gc.ACS_HLINE)
	if len(title) > 0 {
		titleX := (width - len(title)) / 2
		win.MovePrint(0, titleX, title)
	}
	win.NoutRefresh()

	return win, nil
}

func DrawBox(win *gc.Window, y int, x int, height int, width int) {
	win.HLine(y, x+1, gc.ACS_HLINE, width-1)
	win.HLine(height, x+1, gc.ACS_HLINE, width-1)
	win.VLine(y+1, x, gc.ACS_VLINE, height-1)
	win.VLine(y+1, x+width-1, gc.ACS_VLINE, height-1)
	win.MoveAddChar(y, x, gc.ACS_ULCORNER)
	win.MoveAddChar(y, x+width-1, gc.ACS_URCORNER)
	win.MoveAddChar(height, x, gc.ACS_LLCORNER)
	win.MoveAddChar(height, x+width-1, gc.ACS_LRCORNER)
	win.NoutRefresh()
}

func DrawActionButtons(win *gc.Window, buttons []string, selected int, _ int) {
	height, width := win.MaxYX()

	// Draw horizontal separator
	win.HLine(height-3, 1, gc.ACS_HLINE, width-2)
	win.MoveAddChar(height-3, 0, gc.ACS_LTEE)
	win.MoveAddChar(height-3, width-1, gc.ACS_RTEE)

	// Clear the whole row where buttons will sit
	win.MovePrint(height-2, 1, strings.Repeat(" ", width-2))

	// Build button texts, with 3-space gaps, or 1 if that won't fit
	buttonTexts := make([]string, len(buttons))
	textWidth, shown := 0, 0
	for i, button := range buttons {
		buttonTexts[i] = "<" + button + ">"
		if button != "" {
			textWidth += len(buttonTexts[i])
			shown++
		}
	}
	gap := 3
	if shown > 1 && textWidth+gap*(shown-1) > width-2 {
		gap = 1
	}
	totalWidth := 0
	for i := range buttons {
		totalWidth += len(buttonTexts[i])
		if i < len(buttons)-1 {
			totalWidth += gap
		}
	}

	// Center the whole row
	leftMargin := (width - totalWidth) / 2
	x := leftMargin

	for i, text := range buttonTexts {
		if i == selected {
			win.ColorOn(1)
		}
		win.MovePrint(height-2, x, text)
		win.ColorOff(1)

		x += len(text)
		if i < len(buttonTexts)-1 {
			x += gap // fixed 3-space gap
		}
	}

	win.NoutRefresh()
}

func InfoBox(stdscr *gc.Window, title string, text string, clear bool, ok bool) error {
	if clear {
		stdscr.Erase()
		stdscr.NoutRefresh()
		gc.Update()
	}

	height := 3
	// if ok {
	// 	height = 5
	// }

	win, err := NewWindow(stdscr, height, len(text)+4, title, -1)
	if err != nil {
		return err
	}
	defer win.Delete()

	gc.Cursor(0)

	win.MovePrint(1, 2, text)

	// if ok {
	// 	DrawActionButtons(win, []string{"OK"}, 0)
	// }

	win.NoutRefresh()
	gc.Update()

	if ok {
		win.GetChar()
	}

	return nil
}

// SwapConfirmBack is the Japanese button layout: B confirms and A backs
// out. MiSTer sends a controller's A as Enter and B as Esc, so the two
// keys are swapped (a keyboard's Enter and Esc swap too).
var SwapConfirmBack bool

// readKey reads a key, applying the button layout.
func readKey(win *gc.Window) gc.Key {
	k := win.GetChar()
	if SwapConfirmBack {
		switch k {
		case gc.KEY_ESC:
			return 10
		case gc.KEY_ENTER, 10, 13:
			return gc.KEY_ESC
		}
	}
	return k
}

// FitSize limits a window size to the screen.
func FitSize(stdscr *gc.Window, height, width int) (int, int) {
	rows, cols := stdscr.MaxYX()
	if height > rows {
		height = rows
	}
	if width > cols {
		width = cols
	}
	return height, width
}
