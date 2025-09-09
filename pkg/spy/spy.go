package spy

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	ReportSize = 64
	Deadzone   = 15

	// Joystick events
	JS_EVENT_BUTTON = 0x01
	JS_EVENT_AXIS   = 0x02
	JS_EVENT_INIT   = 0x80

	JSIOCGVERSION = 0x80046a01
	JSIOCGAXES    = 0x80016a11
	JSIOCGBUTTONS = 0x80016a12
	JSIOCGNAME    = 0x81006a13
	JSIOCGAXMAP   = 0x80406a32
	JSIOCGBTNMAP  = 0x80406a34
)

// Axis / Button name maps (subset)
var AxisNames = map[uint32]string{
	0x00: "ABS_X", 0x01: "ABS_Y", 0x02: "ABS_Z",
	0x03: "ABS_RX", 0x04: "ABS_RY", 0x05: "ABS_RZ",
	0x10: "ABS_HAT0X", 0x11: "ABS_HAT0Y",
}
var ButtonNames = map[uint32]string{
	0x120: "BTN_SOUTH", 0x121: "BTN_EAST", 0x123: "BTN_NORTH",
	0x124: "BTN_WEST", 0x12a: "BTN_SELECT", 0x12b: "BTN_START",
	0x12c: "BTN_MODE", 0x12d: "BTN_THUMBL", 0x12e: "BTN_THUMBR",
}

// HID key map
var KeyMap = map[byte]string{
	0x04: "A", 0x05: "B", 0x06: "C", 0x07: "D", 0x08: "E", 0x09: "F",
	0x0A: "G", 0x0B: "H", 0x0C: "I", 0x0D: "J", 0x0E: "K", 0x0F: "L",
	0x10: "M", 0x11: "N", 0x12: "O", 0x13: "P", 0x14: "Q", 0x15: "R",
	0x16: "S", 0x17: "T", 0x18: "U", 0x19: "V", 0x1A: "W", 0x1B: "X",
	0x1C: "Y", 0x1D: "Z",
	0x1E: "1", 0x1F: "2", 0x20: "3", 0x21: "4", 0x22: "5",
	0x23: "6", 0x24: "7", 0x25: "8", 0x26: "9", 0x27: "0",
	0x28: "Enter", 0x29: "Esc", 0x2A: "Backspace", 0x2B: "Tab", 0x2C: "Space",
	0x4F: "Right", 0x50: "Left", 0x51: "Down", 0x52: "Up",
}

func applyDeadzone(val int16, center int16) int16 {
	diff := val - center
	if diff < Deadzone && diff > -Deadzone {
		return 0
	}
	return diff
}

// ---------------- HID decoders ----------------
func decodeKeyboard(data []byte) string {
	if len(data) == 8 {
		var pressed []string
		for _, code := range data[2:] {
			if code != 0 {
				if name, ok := KeyMap[code]; ok {
					pressed = append(pressed, name)
				} else {
					pressed = append(pressed, fmt.Sprintf("0x%02x", code))
				}
			}
		}
		if len(pressed) > 0 {
			return fmt.Sprintf("KEYBOARD pressed: %v", pressed)
		}
		return "KEYBOARD released"
	}
	return ""
}

func decodeMouse(data []byte) string {
	if len(data) < 6 {
		return ""
	}
	buttons := data[1]
	x := int16(binary.LittleEndian.Uint16(data[2:4]))
	y := int16(binary.LittleEndian.Uint16(data[4:6]))
	if buttons == 0 && x == 0 && y == 0 {
		return ""
	}
	btns := []string{}
	if buttons&1 != 0 {
		btns = append(btns, "L")
	}
	if buttons&2 != 0 {
		btns = append(btns, "R")
	}
	if buttons&4 != 0 {
		btns = append(btns, "M")
	}
	return fmt.Sprintf("MOUSE buttons=%v dx=%d dy=%d", btns, x, y)
}

func decodeGenericGamepad(data []byte) string {
	if len(data) < 7 {
		return ""
	}
	buttons := binary.LittleEndian.Uint32(data[1:5])
	x := int16(data[5])
	y := int16(data[6])
	rx, ry, hat := int16(0), int16(0), byte(0x0F)
	if len(data) > 7 {
		rx = int16(data[7])
	}
	if len(data) > 8 {
		ry = int16(data[8])
	}
	if len(data) > 9 {
		hat = data[9]
	}
	pressed := []string{}
	for i := 0; i < 30; i++ {
		if buttons&(1<<i) != 0 {
			pressed = append(pressed, fmt.Sprintf("B%d", i+1))
		}
	}
	hatmap := map[byte]string{
		0: "Up", 1: "UpRight", 2: "Right", 3: "DownRight",
		4: "Down", 5: "DownLeft", 6: "Left", 7: "UpLeft", 8: "Neutral",
	}
	return fmt.Sprintf("GAMEPAD btns=%v X=%d Y=%d RX=%d RY=%d Hat=%s",
		pressed, applyDeadzone(x, 0x80), applyDeadzone(y, 0x80),
		applyDeadzone(rx, 0x80), applyDeadzone(ry, 0x80),
		hatmap[hat],
	)
}

// ---------------- Joystick setup (RetroSpy style) ----------------
type JSDev struct {
	fd      int
	name    string
	axes    map[string]int16
	buttons map[string]uint8
}

func setupJSDevice(path string) (*JSDev, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}

	// Name
	buf := make([]byte, 128)
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(JSIOCGNAME), uintptr(unsafe.Pointer(&buf[0]))); errno != 0 {
		return nil, errno
	}
	name := string(bytes.Trim(buf, "\x00"))

	fmt.Printf("Monitoring %s (joystick: %s)\n", path, name)
	return &JSDev{
		fd:      fd,
		name:    name,
		axes:    map[string]int16{},
		buttons: map[string]uint8{},
	}, nil
}

func (js *JSDev) handleEvent(evt []byte) {
	var t uint32
	var val int16
	var etype, num uint8
	_ = binary.Read(bytes.NewReader(evt[0:4]), binary.LittleEndian, &t)
	_ = binary.Read(bytes.NewReader(evt[4:6]), binary.LittleEndian, &val)
	etype = evt[6]
	etype &= ^uint8(JS_EVENT_INIT) // fix: mask off INIT
	num = evt[7]

	if etype == JS_EVENT_AXIS {
		js.axes[fmt.Sprintf("A%d", num)] = val
	} else if etype == JS_EVENT_BUTTON {
		js.buttons[fmt.Sprintf("B%d", num)] = uint8(val)
	}

	axs := []string{}
	for k, v := range js.axes {
		axs = append(axs, fmt.Sprintf("%s=%d", k, v))
	}
	btns := []string{}
	for k, v := range js.buttons {
		btns = append(btns, fmt.Sprintf("%s=%d", k, v))
	}
	fmt.Printf("[%s] Axes[%s] Buttons[%s]\n", js.name, join(axs), join(btns))
}

func join(s []string) string {
	if len(s) == 0 {
		return ""
	}
	return fmt.Sprint(s)
}

// ---------------- Main Monitor ----------------
func MonitorAll() {
	// hidraw
	hiddevs, _ := filepath.Glob("/dev/hidraw*")
	for _, path := range hiddevs {
		fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NONBLOCK, 0)
		if err != nil {
			fmt.Printf("Permission denied opening %s\n", path)
			continue
		}
		fmt.Printf("Monitoring %s (hidraw)\n", path)
		go func(fd int, path string) {
			buf := make([]byte, ReportSize)
			for {
				n, err := unix.Read(fd, buf)
				if err != nil || n == 0 {
					continue
				}
				data := bytes.TrimRight(buf[:n], "\x00")
				if out := decodeKeyboard(data); out != "" {
					fmt.Printf("[%s] %s\n", path, out)
				} else if out := decodeMouse(data); out != "" {
					fmt.Printf("[%s] %s\n", path, out)
				} else if out := decodeGenericGamepad(data); out != "" {
					fmt.Printf("[%s] %s\n", path, out)
				}
			}
		}(fd, path)
	}

	// js
	jsdevs, _ := filepath.Glob("/dev/input/js*")
	for _, path := range jsdevs {
		js, err := setupJSDevice(path)
		if err != nil {
			fmt.Printf("Error setting up js device %s: %v\n", path, err)
			continue
		}
		go func(js *JSDev) {
			buf := make([]byte, 8)
			for {
				_, err := unix.Read(js.fd, buf)
				if err == io.EOF || err != nil {
					continue
				}
				js.handleEvent(buf)
			}
		}(js)
	}

	select {} // block forever
}
