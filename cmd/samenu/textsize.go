package main

import (
	gc "github.com/rthornton128/goncurses"

	"github.com/synrais/SAMenu/pkg/mister"
)

// -------------------------
// Text Size
// -------------------------
//
// Display & Sorting -> Menu list options -> Text size. The menu sets the
// framebuffer divider itself while it's open (bigger divider = bigger
// text), and puts the original back when it exits. Only on the MiSTer's
// own screen, never over SSH.

var optTextSize = labelOption{"Text size", []string{"Auto", "Normal", "Large", "Extra large", "Huge"}, 0}

// Auto picks the biggest text that still gives at least this grid.
const autoCols, autoRows = 60, 20

var textSize struct {
	measured bool
	fullRows int // console size at divider 1 (worked out, never used directly)
	fullCols int
	outW     int // TV output in pixels
	outH     int
	origDiv  int // divider when the menu started, restored on exit
	current  int
}

// measureText works out the console size at full resolution and the
// divider in use, once. It measures at divider 4 (a quarter-size
// framebuffer always fits MiSTer's memory; full size may not, and trying
// it garbles the screen) and multiplies up.
func measureText() bool {
	if textSize.measured {
		return true
	}
	_, cols, err := mister.ConsoleSize()
	if err != nil || cols == 0 {
		return false
	}
	if err := mister.SetFbDivider(4); err != nil {
		return false
	}
	r4, c4, err := mister.ConsoleSize()
	if err != nil || c4 == 0 {
		return false
	}
	w4, h4 := mister.FbSize()
	textSize.fullRows, textSize.fullCols = r4*4, c4*4
	textSize.outW, textSize.outH = w4*4, h4*4
	textSize.origDiv = (c4*4 + cols/2) / cols // rounded
	if textSize.origDiv < 1 || textSize.origDiv > 4 {
		textSize.origDiv = 2
	}
	textSize.current = 4
	textSize.measured = true
	return true
}

// dividerFits reports whether divider d stays inside MiSTer's framebuffer
// memory. If the size couldn't be read, only the dividers MiSTer's own
// auto setting would use at 1080p and above (2 to 4) are allowed.
func dividerFits(d int) bool {
	if textSize.outW == 0 || textSize.outH == 0 {
		return d >= 2
	}
	return mister.FbFits(textSize.outW/d, textSize.outH/d)
}

// wantedDivider is the divider for the Text size setting: the chosen one if
// it's allowed and the menu still fits, otherwise the nearest that works.
func wantedDivider() int {
	fits := func(d, cols, rows int) bool {
		return dividerFits(d) && textSize.fullCols/d >= cols && textSize.fullRows/d >= rows
	}
	want := optTextSize.index // Normal=1 ... Huge=4, Auto=0
	best := 0
	for d := 1; d <= 4; d++ {
		if want == 0 && fits(d, autoCols, autoRows) {
			best = d
		}
		if want > 0 && d <= want && fits(d, minCols, minRows) {
			best = d
		}
	}
	if best == 0 { // nothing fits the rules: the smallest allowed divider
		for d := 1; d <= 4; d++ {
			if dividerFits(d) {
				return d
			}
		}
		return 4
	}
	return best
}

// applyTextSize sets the text size before the menu draws, or does nothing
// off the MiSTer's own screen.
func applyTextSize() {
	if !mister.OnConsole() || !measureText() {
		return
	}
	if d := wantedDivider(); d != textSize.current {
		if mister.SetFbDivider(d) == nil {
			textSize.current = d
		}
	}
}

// applyTextSizeLive changes the text size while the menu is showing.
func applyTextSizeLive(stdscr *gc.Window) {
	if !mister.OnConsole() {
		return
	}
	applyTextSize()
	if rows, cols, err := mister.ConsoleSize(); err == nil {
		_ = gc.ResizeTerm(rows, cols)
	}
	clearScreen(stdscr)
	_ = fitToScreen(stdscr)
}

// restoreTextSize puts the original text size back (on exit).
func restoreTextSize() {
	if textSize.measured && textSize.current != textSize.origDiv {
		if mister.SetFbDivider(textSize.origDiv) == nil {
			textSize.current = textSize.origDiv
		}
	}
}
