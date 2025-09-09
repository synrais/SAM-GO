package spy

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/unix"
)

const (
	reportSize = 64
	deadzone   = 15

	// Joystick event types
	JS_EVENT_BUTTON = 0x01
	JS_EVENT_AXIS   = 0x02
	JS_EVENT_INIT   = 0x80

	// ioctl constants
	JSIOCGVERSION  = 0x80046a01
	JSIOCGAXES     = 0x80016a11
	JSIOCGBUTTONS  = 0x80016a12
	JSIOCGNAME     = 0x81006a13
	JSIOCGAXMAP    = 0x80406a32
	JSIOCGBTNMAP   = 0x80406a34
)

// ---------------- HID Keyboard keycodes ----------------
var keymap = map[byte]string{
	0x04: "A", 0x05: "B", 0x06: "C", 0x07: "D",
	0x1C: "Y", 0x1D: "Z",
	0x28: "Enter", 0x29: "Esc", 0x2C: "Space",
	0x4F: "Right", 0x50: "Left", 0x51: "Down", 0x52: "Up",
}

// ---------------- Axis/Button name maps ----------------
var axisNames = map[uint32]string{
	0x00: "ABS_X", 0x01: "ABS_Y", 0x02: "ABS_Z",
	0x03: "ABS_RX", 0x04: "ABS_RY", 0x05: "ABS_RZ",
	0x10: "ABS_HAT0X", 0x11: "ABS_HAT0Y",
}
var buttonNames = map[uint32]string{
	0x120: "BTN_SOUTH", 0x121: "BTN_EAST", 0x123: "BTN_NORTH", 0x124: "BTN_WEST",
	0x126: "BTN_TL", 0x127: "BTN_TR", 0x128: "BTN_TL2", 0x129: "BTN_TR2",
	0x12a: "BTN_SELECT", 0x12b: "BTN_START", 0x12c: "BTN_MODE",
	0x12d: "BTN_THUMBL", 0x12e: "BTN_THUMBR",
}

// ---------------- HID helpers ----------------
func decodeKeyboard(data []byte) string {
	if len(data) == 8 {
		pressed := []string{}
		for _, code := range data[2:] {
			if code != 0 {
				if name, ok := keymap[code]; ok {
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
	if len(data) < 3 {
		return ""
	}
	buttons := data[1]
	x := int16(binary.LittleEndian.Uint16(data[2:4]))
	y := int16(binary.LittleEndian.Uint16(data[4:6]))
	if buttons == 0 && x == 0 && y == 0 {
		return ""
	}
	btns := []string{}
	if buttons&0x1 != 0 {
		btns = append(btns, "L")
	}
	if buttons&0x2 != 0 {
		btns = append(btns, "R")
	}
	if buttons&0x4 != 0 {
		btns = append(btns, "M")
	}
	return fmt.Sprintf("MOUSE buttons=%v dx=%d dy=%d", btns, x, y)
}

// ---------------- Joystick (RetroSpy style) ----------------
type jsDevice struct {
	fd       int
	path     string
	name     string
	axes     map[string]int16
	buttons  map[string]int
	axmap    []uint32
	btnmap   []uint32
}

func setupJSDevice(fd int, path string) *jsDevice {
	// Version
	buf := make([]byte, 4)
	unix.IoctlGetInt(fd, JSIOCGVERSION)
	version := binary.LittleEndian.Uint32(buf)

	// Axes
	numAxes, _ := unix.IoctlGetInt(fd, JSIOCGAXES)
	numButtons, _ := unix.IoctlGetInt(fd, JSIOCGBUTTONS)

	// Name
	nameBuf := make([]byte, 128)
	unix.Ioctl(fd, JSIOCGNAME, uintptr(unsafe.Pointer(&nameBuf[0])))
	name := string(nameBuf[:])

	// Axis map
	axmap := make([]uint32, numAxes)
	unix.Ioctl(fd, JSIOCGAXMAP, uintptr(unsafe.Pointer(&axmap[0])))

	// Button map
	btnmap := make([]uint32, numButtons)
	unix.Ioctl(fd, JSIOCGBTNMAP, uintptr(unsafe.Pointer(&btnmap[0])))

	fmt.Printf("Monitoring %s (joystick: %s)\n", path, name)
	fmt.Printf("  Driver version: %d\n", version)
	fmt.Printf("  Axes: %d  Buttons: %d\n", numAxes, numButtons)

	stateAxes := make(map[string]int16)
	for _, a := range axmap {
		stateAxes[axisNames[a]] = 0
	}
	stateButtons := make(map[string]int)
	for _, b := range btnmap {
		stateButtons[buttonNames[b]] = 0
	}

	return &jsDevice{fd, path, name, stateAxes, stateButtons, axmap, btnmap}
}

func handleJSEvent(dev *jsDevice, evt []byte) {
	var t uint32
	var val int16
	var etype, num uint8
	binary.Read(bytes.NewReader(evt), binary.LittleEndian, &t)
	binary.Read(bytes.NewReader(evt[4:6]), binary.LittleEndian, &val)
	etype = evt[6]
	num = evt[7]

	if etype&^JS_EVENT_INIT == JS_EVENT_AXIS {
		axis := axisNames[dev.axmap[num]]
		dev.axes[axis] = val
	} else if etype&^JS_EVENT_INIT == JS_EVENT_BUTTON {
		btn := buttonNames[dev.btnmap[num]]
		if val != 0 {
			dev.buttons[btn] = 1
		} else {
			dev.buttons[btn] = 0
		}
	}

	// Full RetroSpy-style dump
	axs := []string{}
	for k, v := range dev.axes {
		axs = append(axs, fmt.Sprintf("%s=%d", k, v))
	}
	btns := []string{}
	for k, v := range dev.buttons {
		btns = append(btns, fmt.Sprintf("%s=%d", k, v))
	}
	fmt.Printf("[%s] Axes[%s] Buttons[%s]\n", dev.name, strings.Join(axs, ", "), strings.Join(btns, ", "))
}

// ---------------- Monitor loop ----------------
func monitorAllInputs() {
	devices := []*jsDevice{}

	// Joysticks
	matches, _ := filepath.Glob("/dev/input/js*")
	for _, path := range matches {
		fd, err := unix.Open(path, os.O_RDONLY|unix.O_NONBLOCK, 0)
		if err != nil {
			continue
		}
		dev := setupJSDevice(fd, path)
		devices = append(devices, dev)
	}

	// HID raw
	hidMatches, _ := filepath.Glob("/dev/hidraw*")
	for _, path := range hidMatches {
		fmt.Printf("Monitoring %s (hidraw)\n", path)
		// TODO: implement HID read loop like python version
	}

	// TODO: use select/poll to multiplex HID + js events
	for _, dev := range devices {
		buf := make([]byte, 8)
		for {
			n, _ := unix.Read(dev.fd, buf)
			if n == 8 {
				handleJSEvent(dev, buf)
			}
		}
	}
}

// ---------------- Exported entrypoint ----------------
func Run(args []string) {
	fmt.Println("Starting Spy monitor…")
	monitorAllInputs()
}
