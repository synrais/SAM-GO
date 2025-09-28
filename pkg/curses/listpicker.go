package curses

import (
	"fmt"
	s "strings"

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
}

func ListPicker(stdscr *gc.Window, opts ListPickerOpts, items []string) (int, int, error) {
	selectedItem := 0
	selectedButton := opts.DefaultButton

	pgUpName := "PgUp"
	pgDownName := "PgDn"

	height := opts.Height
	width := opts.Width

	viewStart := 0
	viewHeight := height - 4
	viewWidth := width - 4
	pgAmount := viewHeight - 1

	// scrolling state
	scrollOffset := 0
	lastSelected := -1

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

	// 🔹 Make input non-blocking with ~120ms tick
	win.Timeout(120)

	var ch gc.Key
	for ch != gc.KEY_ESC {
		// list items
		max := utils.Min([]int{len(items), viewHeight})

		for i := 0; i < max; i++ {
			item := items[viewStart+i]
			display := item

			// reset scroll if selection changed
			if viewStart+i == selectedItem && selectedItem != lastSelected {
				scrollOffset = 0
				lastSelected = selectedItem
			}

			if len(item) > viewWidth {
				if viewStart+i == selectedItem {
					// 🔹 auto-scroll highlighted item
					if scrollOffset+viewWidth <= len(item) {
						display = item[scrollOffset : scrollOffset+viewWidth]
					} else {
						display = item[len(item)-viewWidth:]
					}
					scrollOffset++
					if scrollOffset > len(item)-viewWidth {
						scrollOffset = 0
					}
				} else {
					// truncate non-highlighted
					display = item[:viewWidth-3] + "..."
				}
			}

			if viewStart+i == selectedItem {
				win.ColorOn(1)
			}
			win.MovePrint(i+1, 2, s.Repeat(" ", viewWidth))
			win.MovePrint(i+1, 2, display)
			win.ColorOff(1)
		}

		// --- Scroll bar ---
		scrollHeight := viewHeight
		if scrollHeight > 0 {
			var gripHeight int
			if len(items) <= scrollHeight {
				gripHeight = scrollHeight
			} else {
				gripHeight = int(float64(scrollHeight) * (float64(scrollHeight) / float64(len(items))))
				if gripHeight < 1 {
					gripHeight = 1
				}
			}

			gripOffset := 0
			if len(items) > scrollHeight {
				gripOffset = int(float64(viewStart) * float64(scrollHeight-gripHeight) / float64(len(items)-scrollHeight))
			}

			for i := 0; i < scrollHeight; i++ {
				if i >= gripOffset && i < gripOffset+gripHeight {
					win.ColorOn(1)
					win.MoveAddChar(i+1, width-2, ' ')
					win.ColorOff(1)
				} else {
					win.MoveAddChar(i+1, width-2, gc.ACS_CKBOARD)
				}
			}
		}

		// --- Buttons ---
		buttons := make([]string, len(opts.Buttons))
		copy(buttons, opts.Buttons)
		if opts.DynamicActionLabel != nil && len(buttons) > opts.ActionButton {
			buttons[opts.ActionButton] = opts.DynamicActionLabel(selectedItem)
		}

		DrawActionButtons(win, buttons, selectedButton, 4)
		win.NoutRefresh()

		// location indicators
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

		win.Move(viewStart+selectedItem+1, width-3)

		win.NoutRefresh()
		gc.Update()

		ch = win.GetChar()
		if ch == -1 {
			continue // 🔹 no key → keep auto-scrolling
		}

		// --- Key handling ---
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
			} else if opts.Buttons[selectedButton] == pgUpName {
				pageUp()
			} else if opts.Buttons[selectedButton] == pgDownName {
				pageDown()
			} else {
				return selectedButton, -1, nil
			}
		}
	}

	return -1, -1, nil
}
