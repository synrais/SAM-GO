package input

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
	"path/filepath"
	"golang.org/x/sys/unix"
)

const HOTPLUG_SCAN_INTERVAL = 2 * time.Second // seconds between rescans

var SCAN_CODES = map[int][]string{
	0x04: {"a", "A"}, 0x05: {"b", "B"}, 0x06: {"c", "C"}, 0x07: {"d", "D"},
	0x08: {"e", "E"}, 0x09: {"f", "F"}, 0x0A: {"g", "G"}, 0x0B: {"h", "H"},
	0x0C: {"i", "I"}, 0x0D: {"j", "J"}, 0x0E: {"k", "K"}, 0x0F: {"l", "L"},
	0x10: {"m", "M"}, 0x11: {"n", "N"}, 0x12: {"o", "O"}, 0x13: {"p", "P"},
	0x14: {"q", "Q"}, 0x15: {"r", "R"}, 0x16: {"s", "S"}, 0x17: {"t", "T"},
	0x18: {"u", "U"}, 0x19: {"v", "V"}, 0x1A: {"w", "W"}, 0x1B: {"x", "X"},
	0x1C: {"y", "Y"}, 0x1D: {"z", "Z"}, 0x1E: {"1", "!"}, 0x1F: {"2", "@"},
	0x20: {"3", "#"}, 0x21: {"4", "$"}, 0x22: {"5", "%"}, 0x23: {"6", "^"},
	0x24: {"7", "&"}, 0x25: {"8", "*"}, 0x26: {"9", "("}, 0x27: {"0", ")"},
	0x28: {"ENTER"}, 0x29: {"ESC"}, 0x2A: {"BACKSPACE"}, 0x2B: {"TAB"},
	0x2C: {"SPACE"}, 0x2D: {"-", "_"}, 0x2E: {"=", "+"}, 0x2F: {"[", "{"},
	0x30: {"]", "}"}, 0x31: {"\\", "|"}, 0x32: {"#", "~"}, 0x33: {";", ":"},
	0x34: {"'", "\""}, 0x35: {"`", "~"}, 0x36: {",", "<"}, 0x37: {".", ">"},
	0x38: {"/", "?"}, 0x39: {"CAPS LOCK"}, 0x3A: {"F1"}, 0x3B: {"F2"},
	0x3C: {"F3"}, 0x3D: {"F4"}, 0x3E: {"F5"}, 0x3F: {"F6"}, 0x40: {"F7"},
	0x41: {"F8"}, 0x42: {"F9"}, 0x43: {"F10"}, 0x44: {"F11"}, 0x45: {"F12"},
	0x46: {"PRINT SCREEN"}, 0x47: {"SCROLL LOCK"}, 0x48: {"PAUSE"}, 0x49: {"INSERT"},
	0x4A: {"HOME"}, 0x4B: {"PAGE UP"}, 0x4C: {"DELETE"}, 0x4D: {"END"},
	0x4E: {"PAGE DOWN"}, 0x4F: {"RIGHT"}, 0x50: {"LEFT"}, 0x51: {"DOWN"},
	0x52: {"UP"}, 0x53: {"NUM LOCK"}, 0x54: {"NUMPAD /"}, 0x55: {"NUMPAD *"},
	0x56: {"NUMPAD -"}, 0x57: {"NUMPAD +"}, 0x58: {"NUMPAD ENTER"},
	0x59: {"NUMPAD 1", "END"}, 0x5A: {"NUMPAD 2", "DOWN"}, 0x5B: {"NUMPAD 3", "PAGE DOWN"},
	0x5C: {"NUMPAD 4", "LEFT"}, 0x5D: {"NUMPAD 5"}, 0x5E: {"NUMPAD 6", "RIGHT"},
	0x5F: {"NUMPAD 7", "HOME"}, 0x60: {"NUMPAD 8", "UP"}, 0x61: {"NUMPAD 9", "PAGE UP"},
	0x62: {"NUMPAD 0", "INSERT"}, 0x63: {"NUMPAD .", "DELETE"},
	0x81: {"SYSTEM POWER"}, 0x82: {"SYSTEM SLEEP"}, 0x83: {"SYSTEM WAKE"},
	0xB0: {"PLAY"}, 0xB1: {"PAUSE"}, 0xB2: {"RECORD"}, 0xB3: {"FAST FORWARD"},
	0xB4: {"REWIND"}, 0xB5: {"NEXT TRACK"}, 0xB6: {"PREVIOUS TRACK"},
	0xB7: {"STOP"}, 0xB8: {"EJECT"}, 0xCD: {"PLAY/PAUSE"},
	0xE2: {"MUTE"}, 0xE9: {"VOLUME UP"}, 0xEA: {"VOLUME DOWN"},
	0x194: {"CALCULATOR"}, 0x196: {"BROWSER"}, 0x197: {"MAIL"}, 0x198: {"MEDIA PLAYER"},
	0x199: {"MY COMPUTER"}, 0x19C: {"SEARCH"}, 0x19D: {"HOME PAGE"},
	0x1A6: {"BROWSER BACK"}, 0x1A7: {"BROWSER FORWARD"}, 0x1A8: {"BROWSER REFRESH"},
	0x1A9: {"BROWSER STOP"}, 0x1AB: {"BROWSER FAVORITES"},
	0x006F: {"BRIGHTNESS DOWN"}, 0x0070: {"BRIGHTNESS UP"}, 0x0072: {"DISPLAY TOGGLE"},
	0x0075: {"SCREEN LOCK"},
}

// Parse the `/proc/bus/input/devices` file to find keyboard devices
func parseKeyboards() (map[string]string, error) {
	devices := make(map[string]string)
	file, err := os.Open("/proc/bus/input/devices")
	if err != nil {
		return nil, fmt.Errorf("Error opening /proc/bus/input/devices: %v", err)
	}
	defer file.Close()

	var block []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if contains(line, "Handlers=") && contains(line, "kbd") {
				name, sysfsID := extractDeviceInfo(block)
				if sysfsID != "" {
					devices[sysfsID] = name
				}
			}
			block = nil
		} else {
			block = append(block, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("Error reading /proc/bus/input/devices: %v", err)
	}

	return devices, nil
}

// Helper function to check if a string contains a substring
func contains(str, substr string) bool {
	return strings.Contains(str, substr)
}

// Extract device info (name and sysfs ID) from the block
func extractDeviceInfo(block []string) (string, string) {
	var name, sysfsID string
	for _, line := range block {
		if strings.HasPrefix(line, "N: ") {
			name = strings.TrimSpace(strings.Split(line, "=")[1])
		}
		if strings.HasPrefix(line, "S: Sysfs=") {
			sysfsID = strings.TrimSpace(strings.Split(line, "=")[1])
		}
	}
	return name, sysfsID
}

// Match the keyboards' sysfs IDs to the actual `/dev/hidraw*` devices
func matchHidraws(keyboards map[string]string) ([]string, error) {
	matches := []string{}
	files, err := filepath.Glob("/sys/class/hidraw/hidraw*/device")
	if err != nil {
		return nil, fmt.Errorf("Error in globbing hidraw devices: %v", err)
	}

	for _, hiddev := range files {
		sysfsID := filepath.Base(hiddev)
		if _, found := keyboards[sysfsID]; found {
			matches = append(matches, fmt.Sprintf("/dev/%s", filepath.Base(filepath.Dir(hiddev))))
		}
	}
	return matches, nil
}

// Decode HID report based on the scan codes (returns key events as string)
func decodeReport(report []byte) string {
	if len(report) != 8 {
		return ""
	}

	// Skip the invalid reports
	if report[0] == 0x02 || (report[0] != 0 && allZero(report[1:])) {
		return ""
	}

	var output []string
	for _, code := range report[2:8] {
		if code == 0 {
			continue
		}
		if keys, ok := SCAN_CODES[int(code)]; ok {
			output = append(output, keys[0]) // Using lowercase key, can extend to shift/uppercase logic
		}
	}
	return strings.Join(output, "")
}

// Helper function to check if all bytes are zero
func allZero(slice []byte) bool {
	for _, b := range slice {
		if b != 0 {
			return false
		}
	}
	return true
}

// KeyboardDevice handles opening and reading keyboard device files
type KeyboardDevice struct {
	devnode string
	name    string
	fd      int
}

func NewKeyboardDevice(devnode, name string) (*KeyboardDevice, error) {
	fd, err := unix.Open(devnode, unix.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	return &KeyboardDevice{
		devnode: devnode,
		name:    name,
		fd:      fd,
	}, nil
}

func (kd *KeyboardDevice) Close() {
	if kd.fd >= 0 {
		_ = unix.Close(kd.fd)
		kd.fd = -1
	}
}

func (kd *KeyboardDevice) ReadEvent() string {
	report := make([]byte, 8) // Read 8-byte report
	n, err := unix.Read(kd.fd, report)
	if err != nil || n < 8 {
		return ""
	}
	return decodeReport(report)
}

// Monitor keyboards: Poll for keyboard events, match sysfs, and decode events
func monitorKeyboards(out chan<- string) {
	devices := make(map[string]*KeyboardDevice)
	lastScan := time.Now()

	for {
		now := time.Now()
		if now.Sub(lastScan) > HOTPLUG_SCAN_INTERVAL {
			lastScan = now
			// Rescan for keyboards
			keyboards, err := parseKeyboards()
			if err != nil {
				fmt.Println(err)
				continue
			}

			matches, err := matchHidraws(keyboards)
			if err != nil {
				fmt.Println(err)
				continue
			}

			// Add new devices
			for _, devnode := range matches {
				if _, found := devices[devnode]; !found {
					dev, err := NewKeyboardDevice(devnode, "Keyboard")
					if err == nil {
						devices[devnode] = dev
					}
				}
			}

			// Remove vanished devices
			for devnode := range devices {
				if !stringInSlice(devnode, matches) {
					devices[devnode].Close()
					delete(devices, devnode)
				}
			}
		}

		// Poll for keyboard events and decode them
		for _, dev := range devices {
			output := dev.ReadEvent()
			if output != "" {
				out <- output // Send decoded event to channel
			}
		}

		time.Sleep(200 * time.Millisecond) // Avoid busy loop
	}
}

// Helper function to check if a string is in the list
func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// StreamKeyboards starts the keyboard monitoring and returns a channel of events
func StreamKeyboards() <-chan string {
	out := make(chan string, 100) // Buffered channel
	go monitorKeyboards(out)
	return out
}
