package input

import (
	"fmt"
	"strings"
)

// Input detection
//
// Watches keyboards, mice and controllers and reports each press as an
// Event. Nothing acts on the events yet; attract mode and the games
// menu's -inputs test mode just print them.
//
// While a core is running, MiSTer holds the normal input devices
// exclusively (the Linux "grab"), so ordinary reads see nothing. Each
// detector works around that:
//   - keyboards and mice are read as raw HID data (hidraw), which the
//     grab doesn't affect
//   - controllers are read by reopening the joystick device each poll,
//     which makes the kernel report its live state even while grabbed

// Event is one detected press.
type Event struct {
	Kind   string // "keyboard", "mouse" or "joystick"
	Device string // the device's name
	Name   string // what was pressed, as used in SAM.ini ("a", "dpleft", "leftx-", ...)
	Hint   string // plain-English meaning, when Name isn't obvious ("left stick up")
}

func (e Event) String() string {
	if e.Hint != "" {
		return fmt.Sprintf("%s (%s): %s (%s)", e.Kind, e.Device, e.Name, e.Hint)
	}
	return fmt.Sprintf("%s (%s): %s", e.Kind, e.Device, e.Name)
}

// Options picks which detectors run ([InputDetector] in SAM.ini).
type Options struct {
	Keyboard bool
	Mouse    bool
	Joystick bool
	Quiet    bool // don't print devices being found or lost
}

// Start runs the enabled detectors in the background and returns the
// channel their events arrive on. If nobody reads the channel fast
// enough, events are dropped rather than holding the detectors up.
func Start(opts Options) <-chan Event {
	quiet = opts.Quiet
	out := make(chan Event, 64)
	if opts.Keyboard || opts.Mouse {
		go watchHID(out, opts.Keyboard, opts.Mouse)
	}
	if opts.Joystick {
		go watchJoysticks(out)
	}
	return out
}

func send(out chan<- Event, ev Event) {
	select {
	case out <- ev:
	default:
	}
}

var quiet bool

func logf(format string, args ...interface{}) {
	if quiet {
		return
	}
	fmt.Printf("[Input] "+format+"\n", args...)
}

// cleanName turns a label like "PAGE UP" into a SAM.ini style name
// ("pageup").
func cleanName(s string) string {
	return strings.ReplaceAll(strings.ToLower(s), " ", "")
}
