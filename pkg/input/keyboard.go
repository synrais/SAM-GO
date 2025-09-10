package input

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unsafe"
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

type KeyboardDevice struct {
	Path string
	FD   int
}

func openKeyboardDevice(path string) (*KeyboardDevice, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	fmt.Printf("[+] Opened %s (fd=%d)\n", path, fd)
	return &KeyboardDevice{Path: path, FD: fd}, nil
}

func (kd *KeyboardDevice) Close() {
	if kd.FD >= 0 {
		_ = unix.Close(kd.FD)
		fmt.Printf("[-] Closed %s\n", kd.Path)
		kd.FD = -1
	}
}

// StreamKeyboards returns a channel of decoded keyboard events
func StreamKeyboards() <-chan string {
	out := make(chan string, 100) // Buffered channel

	go func() {
		defer close(out)
		devices := map[string]*KeyboardDevice{}

		// initial scan for /dev/input/event*
		paths, _ := filepath.Glob("/dev/input/event*")
		for _, path := range paths {
			if dev, err := openKeyboardDevice(path); err == nil {
				devices[path] = dev
			}
		}

		// inotify watch on /dev/input
		inFd, err := unix.InotifyInit()
		if err != nil {
			fmt.Println("inotify init failed:", err)
			return
		}
		defer unix.Close(inFd)

		_, err = unix.InotifyAddWatch(inFd, "/dev/input", unix.IN_CREATE|unix.IN_DELETE)
		if err != nil {
			fmt.Println("inotify addwatch failed:", err)
			return
		}

		for {
			// Poll devices + inotify events
			var pollfds []unix.PollFd
			for _, dev := range devices {
				if dev.FD >= 0 {
					pollfds = append(pollfds, unix.PollFd{Fd: int32(dev.FD), Events: unix.POLLIN})
				}
			}
			pollfds = append(pollfds, unix.PollFd{Fd: int32(inFd), Events: unix.POLLIN})

			n, err := unix.Poll(pollfds, int(HOTPLUG_SCAN_INTERVAL.Milliseconds()))
			if err != nil {
				fmt.Println("[DEBUG] poll error:", err)
				continue
			}
			if n == 0 {
				continue
			}

			for _, pfd := range pollfds {
				// Handle inotify events
				if pfd.Fd == int32(inFd) && pfd.Revents&unix.POLLIN != 0 {
					buf := make([]byte, 4096)
					n, _ := unix.Read(inFd, buf)
					offset := 0
					for offset < n {
						raw := (*unix.InotifyEvent)(unsafe.Pointer(&buf[offset]))
						nameBytes := buf[offset+unix.SizeofInotifyEvent : offset+unix.SizeofInotifyEvent+int(raw.Len)]
						name := string(nameBytes[:len(nameBytes)-1]) // trim null
						path := filepath.Join("/dev/input", name)

						if raw.Mask&unix.IN_CREATE != 0 {
							if filepath.HasPrefix(filepath.Base(path), "event") {
								if dev, err := openKeyboardDevice(path); err == nil {
									devices[path] = dev
								}
							}
						}
						if raw.Mask&unix.IN_DELETE != 0 {
							if dev, ok := devices[path]; ok {
								dev.Close()
								delete(devices, path)
							}
						}
						offset += unix.SizeofInotifyEvent + int(raw.Len)
					}
				}

				// Handle keyboard data
				if pfd.Fd != int32(inFd) && pfd.Revents&unix.POLLIN != 0 {
					buf := make([]byte, 8) // Report size for keyboards
					n, err := unix.Read(int(pfd.Fd), buf)
					if err != nil || n < 8 {
						continue
					}

					output := decodeReport(buf) // Decoding the key event
					if output != "" {
						out <- output // Send decoded key event to channel
					}
				}
			}
		}
	}()

	return out
}

func decodeReport(report []byte) string {
	if len(report) != 8 {
		return ""
	}

	var output []string
	for _, code := range report[2:8] {
		if code == 0 {
			continue
		}
		if keys, ok := SCAN_CODES[int(code)]; ok {
			output = append(output, keys[0]) // Return the first key (you could also handle shift here)
		}
	}
	return strings.Join(output, "")
}
