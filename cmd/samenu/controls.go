package main

import (
	"fmt"
	"strings"

	gc "github.com/rthornton128/goncurses"

	"github.com/synrais/SAMenu/pkg/config"
)

// -------------------------
// Menu Controls
// -------------------------
//
// SAMenu shortcuts, fixed: Y (tab) = Search, X (space) = Options. A
// controller reaches the menu as keys (MiSTer sends A = enter, B = esc,
// X = space, Y = tab, L/R = page up/down).

// menuControls is action -> key names (the fixed defaults).
var menuControls = config.DefaultMenuControlsCopy()

// namedKeys are the keys with names; any other printable character is its
// own name ("/", "o", "`").
var namedKeys = map[string][]gc.Key{
	"esc":       {27},
	"backspace": {gc.KEY_BACKSPACE, 127, 8},
	"tab":       {9},
	"space":     {' '},
	"delete":    {gc.KEY_DC},
	"insert":    {gc.KEY_IC},
	"home":      {gc.KEY_HOME},
	"end":       {gc.KEY_END},
}

func init() {
	for i := 1; i <= 12; i++ {
		namedKeys[fmt.Sprintf("f%d", i)] = []gc.Key{gc.KEY_F1 + gc.Key(i-1)}
	}
}

// keysFor returns the ncurses keys for a key name.
func keysFor(name string) []gc.Key {
	name = strings.ToLower(strings.TrimSpace(name))
	if k, ok := namedKeys[name]; ok {
		return k
	}
	if len(name) == 1 && name[0] > ' ' && name[0] < 127 {
		return []gc.Key{gc.Key(name[0])}
	}
	return nil
}

// buttonFor is the list button each menu action presses.
var buttonFor = map[string]string{"search": "Search", "options": "Options"}

// menuShortcuts builds the list picker shortcuts from the bindings.
func menuShortcuts() map[gc.Key]string {
	out := map[gc.Key]string{}
	for act, names := range menuControls {
		for _, n := range names {
			for _, k := range keysFor(n) {
				out[k] = buttonFor[act]
			}
		}
	}
	return out
}
