package curses

import (
	"fmt"
	"strings"
	"time"

	gc "github.com/rthornton128/goncurses"
	"github.com/synrais/SAM-GO/pkg/utils"
)

type ListPickerOpts struct {
	Title              string
	Buttons            []string
	DefaultButton      int
	ActionButton       int
	ShowTotal          bool
	Width              int
	Height             int
	DynamicActionLabel func(selectedItem int) string
	InitialIndex       int // 🔹 new: where to start highlight
}

func ListPicker(stdscr *gc.Window, opts ListPickerOpts, items []string) (int, int, error) {
	// apply InitialIndex safely
	selectedItem := opts.InitialIndex
	if selectedItem < 0 || selectedItem >= len(items) {
		selectedItem = 0
	}
	selectedButton := opts.DefaultButton

	height := opts.Height
	width := opts.Width

	viewStart := 0
	viewHeight := height - 4
	viewWidth := opts.Width - 4
	pgAmount := viewHeight - 1

	// 🔹 ensure selected item is visible, without forcing it to the top
	if selectedItem < viewStart {
		viewStart = selectedItem
	} else if selectedItem >= viewStart+viewHeight {
		viewStart = selectedItem - viewHeight + 1
	}

	// marquee tracking
	currentSelection := -1
	scrollOffset := 0
	lastScroll := time.Now()

	pageUp := func() {
		if viewStart == 0 {
			selectedItem = 0
		} else if (viewStart - pgAmount) < 0 {
			viewStart = 0
			selectedItem = 0
		} else {
			viewStart -= pgAmount
			selectedItem = viewStart
		}
		if selectedItem >= len(items) {
			selectedItem = len(items) - 1
		}
	}

	pageDown := func() {
		if len(items) <= viewHeight {
			return
		}
		if viewStart+viewHeight >= len(items) {
			selectedItem = len(items) - 1
		} else {
			viewStart += pgAmount
			if viewStart+viewHeight > len(items) {
				viewStart = len(items) - viewHeight
			}
			selectedItem = viewStart
		}
		if selectedItem >= len(items) {
			selectedItem = len(items) - 1
		}
	}

	win, err := NewWindow(stdscr, height, width, opts.Title, -1)
	if err != nil {
		return -1, -1, err
	}
	defer win.Delete()

	// non-blocking input with 100ms tick
	win.Timeout(100)
	var ch gc.Key

	for ch != gc.KEY_ESC {
		// reset scroll when selection changes
		if selectedItem != currentSelection {
			currentSelection = selectedItem
			scrollOffset = 0
			lastScroll = time.Now()
		}

		// advance marquee scroll every 200ms
		if time.Since(lastScroll) > 200*time.Millisecond {
			scrollOffset++
			lastScroll = time.Now()
		}

		// list items
		max := utils.Min([]int{len(items), viewHeight})

		for i := 0; i < max; i++ {
			item := items[viewStart+i]
			display := item

			if len(item) > viewWidth {
				if viewStart+i == selectedItem {
					// build marquee string with gap
					marquee := item + "   " // always a 3-space gap
					marquee = marquee + marquee

					offset := scrollOffset % (len(item) + 3)
					display = marquee[offset : offset+viewWidth]
				} else {
					// normal truncation
					display = item[:viewWidth-3] + "..."
				}
			}

			if viewStart+i == selectedItem {
				win.ColorOn(1)
			}
			win.MovePrint(i+1, 2, strings.Repeat(" ", viewWidth))
			win.MovePrint(i+1, 2, display)
			win.ColorOff(1)
		}

		// --- Buttons ---
		buttons := make([]string, len(opts.Buttons))
		copy(buttons, opts.Buttons)
		if opts.DynamicActionLabel != nil && len(buttons) > opts.ActionButton {
			buttons[opts.ActionButton] = opts.DynamicActionLabel(selectedItem)
		}

		DrawActionButtons(win, buttons, selectedButton, 4)

		// --- Position/scroll indicators ---
		if opts.ShowTotal {
			totalStatus := fmt.Sprintf("%*d/%d", len(fmt.Sprint(len(items))), selectedItem+1, len(items))
			win.MovePrint(0, 2, totalStatus)
		}
		if viewStart > 0 {
			win.MoveAddChar(0, width-3, gc.ACS_UARROW)
		} else {
			win.MoveAddChar(0, width-3, gc.ACS_HLINE)
		}
		if viewStart+viewHeight < len(items) {
			win.MoveAddChar(height-3, width-3, gc.ACS_DARROW)
		} else {
			win.MoveAddChar(height-3, width-3, gc.ACS_HLINE)
		}

		win.NoutRefresh()
		gc.Update()

		// non-blocking read
		ch = win.GetChar()

		switch ch {
		case gc.KEY_DOWN:
			if selectedItem < len(items)-1 {
				selectedItem++
				if selectedItem >= viewStart+viewHeight && viewStart+viewHeight < len(items) {
					viewStart++
				}
			}
		case gc.KEY_UP:
			if selectedItem > 0 {
				selectedItem--
				if selectedItem < viewStart && viewStart > 0 {
					viewStart--
				}
			}
		case gc.KEY_LEFT:
			if selectedButton > 0 {
				selectedButton--
			} else {
				selectedButton = len(opts.Buttons) - 1
			}
		case gc.KEY_RIGHT:
			if selectedButton < len(opts.Buttons)-1 {
				selectedButton++
			} else {
				selectedButton = 0
			}
		case gc.KEY_PAGEUP:
			pageUp()
		case gc.KEY_PAGEDOWN:
			pageDown()
		case gc.KEY_ENTER, 10, 13:
			if selectedButton == opts.ActionButton {
				return selectedButton, selectedItem, nil
			} else {
				return selectedButton, -1, nil
			}
		}
	}

	return -1, -1, nil
}
