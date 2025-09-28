package input

import (
	"fmt"
	"strings"

	"github.com/bendahl/uinput"
)

// map of runes to their base key and whether they need Shift
type keyMapping struct {
	key   int
	shift bool
}

var runeToKey = map[rune]keyMapping{
	// Letters
	'a': {uinput.KeyA, false}, 'A': {uinput.KeyA, true},
	'b': {uinput.KeyB, false}, 'B': {uinput.KeyB, true},
	'c': {uinput.KeyC, false}, 'C': {uinput.KeyC, true},
	'd': {uinput.KeyD, false}, 'D': {uinput.KeyD, true},
	'e': {uinput.KeyE, false}, 'E': {uinput.KeyE, true},
	'f': {uinput.KeyF, false}, 'F': {uinput.KeyF, true},
	'g': {uinput.KeyG, false}, 'G': {uinput.KeyG, true},
	'h': {uinput.KeyH, false}, 'H': {uinput.KeyH, true},
	'i': {uinput.KeyI, false}, 'I': {uinput.KeyI, true},
	'j': {uinput.KeyJ, false}, 'J': {uinput.KeyJ, true},
	'k': {uinput.KeyK, false}, 'K': {uinput.KeyK, true},
	'l': {uinput.KeyL, false}, 'L': {uinput.KeyL, true},
	'm': {uinput.KeyM, false}, 'M': {uinput.KeyM, true},
	'n': {uinput.KeyN, false}, 'N': {uinput.KeyN, true},
	'o': {uinput.KeyO, false}, 'O': {uinput.KeyO, true},
	'p': {uinput.KeyP, false}, 'P': {uinput.KeyP, true},
	'q': {uinput.KeyQ, false}, 'Q': {uinput.KeyQ, true},
	'r': {uinput.KeyR, false}, 'R': {uinput.KeyR, true},
	's': {uinput.KeyS, false}, 'S': {uinput.KeyS, true},
	't': {uinput.KeyT, false}, 'T': {uinput.KeyT, true},
	'u': {uinput.KeyU, false}, 'U': {uinput.KeyU, true},
	'v': {uinput.KeyV, false}, 'V': {uinput.KeyV, true},
	'w': {uinput.KeyW, false}, 'W': {uinput.KeyW, true},
	'x': {uinput.KeyX, false}, 'X': {uinput.KeyX, true},
	'y': {uinput.KeyY, false}, 'Y': {uinput.KeyY, true},
	'z': {uinput.KeyZ, false}, 'Z': {uinput.KeyZ, true},

	// Numbers and shifted symbols
	'0': {uinput.Key0, false}, ')': {uinput.Key0, true},
	'1': {uinput.Key1, false}, '!': {uinput.Key1, true},
	'2': {uinput.Key2, false}, '@': {uinput.Key2, true},
	'3': {uinput.Key3, false}, '#': {uinput.Key3, true},
	'4': {uinput.Key4, false}, '$': {uinput.Key4, true},
	'5': {uinput.Key5, false}, '%': {uinput.Key5, true},
	'6': {uinput.Key6, false}, '^': {uinput.Key6, true},
	'7': {uinput.Key7, false}, '&': {uinput.Key7, true},
	'8': {uinput.Key8, false}, '*': {uinput.Key8, true},
	'9': {uinput.Key9, false}, '(': {uinput.Key9, true},

	// Whitespace
	' ': {uinput.KeySpace, false},
	'\n': {uinput.KeyEnter, false},
	'\t': {uinput.KeyTab, false},

	// Punctuation
	'-': {uinput.KeyMinus, false}, '_': {uinput.KeyMinus, true},
	'=': {uinput.KeyEqual, false}, '+': {uinput.KeyEqual, true},
	'[': {uinput.KeyLeftbrace, false}, '{': {uinput.KeyLeftbrace, true},
	']': {uinput.KeyRightbrace, false}, '}': {uinput.KeyRightbrace, true},
	'\\': {uinput.KeyBackslash, false}, '|': {uinput.KeyBackslash, true},
	';': {uinput.KeySemicolon, false}, ':': {uinput.KeySemicolon, true},
	'\'': {uinput.KeyApostrophe, false}, '"': {uinput.KeyApostrophe, true},
	',': {uinput.KeyComma, false}, '<': {uinput.KeyComma, true},
	'.': {uinput.KeyDot, false}, '>': {uinput.KeyDot, true},
	'/': {uinput.KeySlash, false}, '?': {uinput.KeySlash, true},
	'`': {uinput.KeyGrave, false}, '~': {uinput.KeyGrave, true},
}

// TypeRune presses a single rune if supported
func (k *VirtualKeyboard) TypeRune(r rune) error {
	mapping, ok := runeToKey[r]
	if !ok {
		return fmt.Errorf("unsupported character: %q", r)
	}
	if mapping.shift {
		return k.Combo(uinput.KeyLeftshift, mapping.key)
	}
	return k.Press(mapping.key)
}

// TypeString types an entire string
func (k *VirtualKeyboard) TypeString(s string) error {
	for _, r := range s {
		if err := k.TypeRune(r); err != nil {
			return err
		}
	}
	return nil
}
