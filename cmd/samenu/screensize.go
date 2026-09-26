package main

import (
	"fmt"

	gc "github.com/rthornton128/goncurses"
)

// -------------------------
// Screen Size
// -------------------------
//
// The console's size depends on each MiSTer's TV resolution, fb_size and
// font, so the menu sizes itself from the real screen when it starts:
// lists fill the height, and the width fills the screen up to a
// comfortable maximum (very wide lines are hard to read on a big console).

// listHeight is the height of the full-size lists (systems, games,
// search results, tick lists).
var listHeight = 20

const (
	minCols      = 50 // below this the menu can't be laid out usefully
	minRows      = 14
	maxListWidth = 110 // lists stop growing here on very wide consoles
	maxOptsWidth = 72
)

// fitToScreen sets the menu's sizes from the screen, or reports that the
// screen is too small to use.
func fitToScreen(stdscr *gc.Window) error {
	rows, cols := stdscr.MaxYX()
	if cols < minCols || rows < minRows {
		return fmt.Errorf("The screen is %dx%d characters, SAMenu needs at least %dx%d.\n"+
			"Try a larger fb_size in MiSTer.ini (e.g. fb_size=1 or 2).", cols, rows, minCols, minRows)
	}
	systemListWidth = clamp(cols-2, minCols, maxListWidth)
	optionsWidth = clamp(cols-2, minCols, maxOptsWidth)
	listHeight = rows - 2
	return nil
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
