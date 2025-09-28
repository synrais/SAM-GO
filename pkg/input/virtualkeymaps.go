package input

import (
	"fmt"
	"time"

	"github.com/bendahl/uinput"
)

const keyDelay = 40 * time.Millisecond

// USKeyboard maps runes to keycodes and shift requirements for US layout.
var USKeyboard = map[rune]struct {
	code  int
	shift bool
}{
	// Letters
	'a': {uinput.KeyA, false}, 'b': {uinput.KeyB, false}, 'c': {uinput.KeyC, false},
	'd': {uinput.KeyD, false}, 'e': {uinput.KeyE, false}, 'f': {uinput.KeyF, false},
	'g': {uinput.KeyG, false}, 'h': {uinput.KeyH, false}, 'i': {uinput.KeyI, false},
	'j': {uinput.KeyJ, false}, 'k': {uinput.KeyK, false}, 'l': {uinput.KeyL, false},
	'm': {uinput.KeyM, false}, 'n': {uinput.KeyN, false}, 'o': {uinput.KeyO, false},
	'p': {uinput.KeyP, false}, 'q': {uinput.KeyQ, false}, 'r': {uinput.KeyR, false},
	's': {uinput.KeyS, false}, 't': {uinput.KeyT, false}, 'u': {uinput.KeyU, false},
	'v': {uinput.KeyV, false}, 'w': {uinput.KeyW, false}, 'x': {uinput.KeyX, false},
	'y': {uinput.KeyY, false}, 'z': {uinput.KeyZ, false},

	// Uppercase letters
	'A': {uinput.KeyA, true}, 'B': {uinput.KeyB, true}, 'C': {uinput.KeyC, true},
	'D': {uinput.KeyD, true}, 'E': {uinput.KeyE, true}, 'F': {uinput.KeyF, true},
	'G': {uinput.KeyG, true}, 'H': {uinput.KeyH, true}, 'I': {uinput.KeyI, true},
	'J': {uinput.KeyJ, true}, 'K': {uinput.KeyK, true}, 'L': {uinput.KeyL, true},
	'M': {uinput.KeyM, true}, 'N': {uinput.KeyN, true}, 'O': {uinput.KeyO, true},
	'P': {uinput.KeyP, true}, 'Q': {uinput.KeyQ, true}, 'R': {uinput.KeyR, true},
	'S': {uinput.KeyS, true}, 'T': {uinput.KeyT, true}, 'U': {uinput.KeyU, true},
	'V': {uinput.KeyV, true}, 'W': {uinput.KeyW, true}, 'X': {uinput.KeyX, true},
	'Y': {uinput.KeyY, true}, 'Z': {uinput.KeyZ, true},

	// Numbers
	'0': {uinput.Key0, false}, '1': {uinput.Key1, false}, '2': {uinput.Key2, false},
	'3': {uinput.Key3, false}, '4': {uinput.Key4, false}, '5': {uinput.Key5, false},
	'6': {uinput.Key6, false}, '7': {uinput.Key7, false}, '8': {uinput.Key8, false},
	'9': {uinput.Key9, false},

	// Shifted numbers (symbols)
	')': {uinput.Key0, true}, '!': {uinput.Key1, true}, '@': {uinput.Key2, true},
	'#': {uinput.Key3, true}, '$': {uinput.Key4, true}, '%': {uinput.Key5, true},
	'^': {uinput.Key6, true}, '&': {uinput.Key7, true}, '*': {uinput.Key8, true},
	'(': {uinput.Key9, true},

	// Symbols and punctuation
	' ': {uinput.KeySpace, false},
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

// TypeRune types a single rune using the virtual keyboard.
func (k *VirtualKeyboard) TypeRune(r rune) error {
	entry, ok := USKeyboard[r]
	if !ok {
		return fmt.Errorf("unsupported rune: %q", r)
	}

	if entry.shift {
		if err := k.Device.KeyDown(uinput.KeyLeftshift); err != nil {
			return err
		}
		time.Sleep(keyDelay)
	}

	if err := k.Press(entry.code); err != nil {
		return err
	}

	if entry.shift {
		if err := k.Device.KeyUp(uinput.KeyLeftshift); err != nil {
			return err
		}
	}

	return nil
}

// TypeString types an entire string.
func (k *VirtualKeyboard) TypeString(s string) error {
	for _, r := range s {
		if err := k.TypeRune(r); err != nil {
			return err
		}
		time.Sleep(keyDelay)
	}
	return nil
}
