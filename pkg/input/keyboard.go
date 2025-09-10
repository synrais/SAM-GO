package input

import (
	"bufio"
	"fmt"
	"strings"
	"time"
	"golang.org/x/sys/unix"
)

const HOTPLUG_SCAN_INTERVAL = 2.0 * time.Second

// The embedded SCAN_CODES in Go, copied from the Python format
var SCAN_CODES = map[int][]string{
	0x04: {'a', 'A'}, 0x05: {'b', 'B'}, 0x06: {'c', 'C'}, 0x07: {'d', 'D'},
	0x08: {'e', 'E'}, 0x09: {'f', 'F'}, 0x0A: {'g', 'G'}, 0x0B: {'h', 'H'},
	0x0C: {'i', 'I'}, 0x0D: {'j', 'J'}, 0x0E: {'k', 'K'}, 0x0F: {'l', 'L'},
	0x10: {'m', 'M'}, 0x11: {'n', 'N'}, 0x12: {'o', 'O'}, 0x13: {'p', 'P'},
	0x14: {'q', 'Q'}, 0x15: {'r', 'R'}, 0x16: {'s', 'S'}, 0x17: {'t', 'T'},
	0x18: {'u', 'U'}, 0x19: {'v', 'V'}, 0x1A: {'w', 'W'}, 0x1B: {'x', 'X'},
	0x1C: {'y', 'Y'}, 0x1D: {'z', 'Z'},
	0x1E: {'1', '!'}, 0x1F: {'2', '@'}, 0x20: {'3', '#'}, 0x21: {'4', '$'},
	0x22: {'5', '%'}, 0x23: {'6', '^'}, 0x24: {'7', '&'}, 0x25: {'8', '*'},
	0x26: {'9', '('}, 0x27: {'0', ')'},
	0x28: {'ENTER',}, 0x29: {'ESC',}, 0x2A: {'BACKSPACE',}, 0x2B: {'TAB',},
	0x2C: {'SPACE',}, 0x2D: {'-', '_'}, 0x2E: {'=', '+'}, 0x2F: {'[', '{'},
	0x30: {']', '}'}, 0x31: {'\\', '|'}, 0x32: {'#','~'}, 0x33: {';', ':'},
	0x34: {"'", '"'}, 0x35: {'`', '~'}, 0x36: {',', '<'}, 0x37: {'.', '>'},
	0x38: {'/', '?'}, 0x39: {'CAPS LOCK',},
	0x3A: {'F1',}, 0x3B: {'F2',}, 0x3C: {'F3',}, 0x3D: {'F4',},
	0x3E: {'F5',}, 0x3F: {'F6',}, 0x40: {'F7',}, 0x41: {'F8',},
	0x42: {'F9',}, 0x43: {'F10',}, 0x44: {'F11',}, 0x45: {'F12',},
	0x46: {'PRINT SCREEN',}, 0x47: {'SCROLL LOCK',}, 0x48: {'PAUSE',},
	0x49: {'INSERT',}, 0x4A: {'HOME',}, 0x4B: {'PAGE UP',},
	0x4C: {'DELETE',}, 0x4D: {'END',}, 0x4E: {'PAGE DOWN',},
	0x4F: {'RIGHT',}, 0x50: {'LEFT',}, 0x51: {'DOWN',}, 0x52: {'UP',},
	0x53: {'NUM LOCK',},
	0x54: {'NUMPAD /',}, 0x55: {'NUMPAD *',}, 0x56: {'NUMPAD -',},
	0x57: {'NUMPAD +',}, 0x58: {'NUMPAD ENTER',},
	0x59: {'NUMPAD 1', 'END'}, 0x5A: {'NUMPAD 2', 'DOWN'},
	0x5B: {'NUMPAD 3', 'PAGE DOWN'}, 0x5C: {'NUMPAD 4', 'LEFT'},
	0x5D: {'NUMPAD 5',}, 0x5E: {'NUMPAD 6', 'RIGHT'},
	0x5F: {'NUMPAD 7', 'HOME'}, 0x60: {'NUMPAD 8', 'UP'},
	0x61: {'NUMPAD 9', 'PAGE UP'}, 0x62: {'NUMPAD 0', 'INSERT'},
	0x63: {'NUMPAD .', 'DELETE'},
	0x81: {'SYSTEM POWER',},
	0x82: {'SYSTEM SLEEP',},
	0x83: {'SYSTEM WAKE',},
	0xB0: {'PLAY',},
	0xB1: {'PAUSE',},
	0xB2: {'RECORD',},
	0xB3: {'FAST FORWARD',},
	0xB4: {'REWIND',},
	0xB5: {'NEXT TRACK',},
	0xB6: {'PREVIOUS TRACK',},
	0xB7: {'STOP',},
	0xB8: {'EJECT',},
	0xCD: {'PLAY/PAUSE',},  // most common on keyboards
	0xE2: {'MUTE',},
	0xE9: {'VOLUME UP',},
	0xEA: {'VOLUME DOWN',},
	0x194: {'CALCULATOR',},
	0x196: {'BROWSER',},
	0x197: {'MAIL',},
	0x198: {'MEDIA PLAYER',},
	0x199: {'MY COMPUTER',},
	0x19C: {'SEARCH',},
	0x19D: {'HOME PAGE',},
	0x1A6: {'BROWSER BACK',},
	0x1A7: {'BROWSER FORWARD',},
	0x1A8: {'BROWSER REFRESH',},
	0x1A9: {'BROWSER STOP',},
	0x1AB: {'BROWSER FAVORITES',},
	0x006F: {'BRIGHTNESS DOWN',},
	0x0070: {'BRIGHTNESS UP',},
	0x0072: {'DISPLAY TOGGLE',},      // switch display / presentation mode
	0x0075: {'SCREEN LOCK',},
}

type KeyboardDevice struct {
	devnode string
	name    string
	fd      int
}

// Open device
func (k *KeyboardDevice) open() {
	fd, err := unix.Open(k.devnode, unix.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		fmt.Printf("Failed to open %s\n", k.devnode)
		return
	}
	k.fd = fd
	fmt.Printf("[+] Opened %s → %s\n", k.devnode, k.name)
}

// Close device
func (k *KeyboardDevice) close() {
	if k.fd >= 0 {
		unix.Close(k.fd)
		k.fd = -1
		fmt.Printf("[-] Closed %s → %s\n", k.devnode, k.name)
	}
}

// Read keyboard event
func (k *KeyboardDevice) readEvent() string {
	if k.fd < 0 {
		return ""
	}

	buf := make([]byte, 8)
	n, err := unix.Read(k.fd, buf)
	if err != nil || n < 8 {
		return ""
	}

	// Decode the key event here
	keycode := buf[2]
	if key, found := SCAN_CODES[int(keycode)]; found {
		return key[0] // For simplicity, returning the first key found
	}
	return ""
}

// Monitor keyboard events
func monitorKeyboards() {
	devices := map[string]*KeyboardDevice{}
	lastScan := time.Now()

	for {
		now := time.Now()
		if now.Sub(lastScan) > HOTPLUG_SCAN_INTERVAL {
			lastScan = now
			// Rescan for keyboards here (parse /proc/bus/input/devices and match hidraws)
		}

		// Main loop to process events
		for _, dev := range devices {
			if dev.readEvent() != "" {
				// If an event was read, print or process the result
				fmt.Println(dev.readEvent())
			}
		}

		time.Sleep(0.2 * time.Second)
	}
}

func main() {
	// Initialize and monitor keyboards
	monitorKeyboards()
}
