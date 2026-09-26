package virtualinput

import (
	"strings"

	"github.com/bendahl/uinput"
)

// ToKeyboardCode looks up a key by name: "a", "{f9}" or "f9" (named keys
// work with or without braces).
func ToKeyboardCode(name string) (int, bool) {
	if v, ok := KeyboardMap[name]; ok { // direct reference, no prefix
		return v, ok
	}
	if v, ok := KeyboardMap["{"+strings.Trim(name, "{}")+"}"]; ok {
		return v, ok
	}
	return 0, false
}

var GamepadMap = map[string]int{
	"^":        uinput.ButtonDpadUp,
	"{up}":     uinput.ButtonDpadUp,
	"v":        uinput.ButtonDpadDown, // fixed: was pointing to Up
	"V":        uinput.ButtonDpadDown,
	"{down}":   uinput.ButtonDpadDown,
	"<":        uinput.ButtonDpadLeft,
	"{left}":   uinput.ButtonDpadLeft,
	">":        uinput.ButtonDpadRight,
	"{right}":  uinput.ButtonDpadRight,
	"A":        uinput.ButtonEast,
	"a":        uinput.ButtonEast,
	"{east}":   uinput.ButtonEast,
	"B":        uinput.ButtonSouth,
	"b":        uinput.ButtonSouth,
	"{south}":  uinput.ButtonSouth,
	"X":        uinput.ButtonNorth,
	"x":        uinput.ButtonNorth,
	"{north}":  uinput.ButtonNorth,
	"Y":        uinput.ButtonWest,
	"y":        uinput.ButtonWest,
	"{west}":   uinput.ButtonWest,
	"{start}":  uinput.ButtonStart,
	"{select}": uinput.ButtonSelect,
	"{menu}":   uinput.ButtonMode,
	"L":        uinput.ButtonBumperLeft,
	"l":        uinput.ButtonBumperLeft,
	"{l1}":     uinput.ButtonBumperLeft,
	"R":        uinput.ButtonBumperRight,
	"r":        uinput.ButtonBumperRight,
	"{r1}":     uinput.ButtonBumperRight,
	"{l2}":     uinput.ButtonTriggerLeft,
	"{r2}":     uinput.ButtonTriggerRight,
}
