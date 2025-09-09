package spy

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	// HID
	ReportSize = 64
	Deadzone   = 15

	// Joystick constants
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

// Lookup tables (RetroSpy style)
var axisNames = map[uint32]string{
	0x00: "ABS_X", 0x01: "ABS_Y", 0x02: "ABS_Z",
	0x03: "ABS_RX", 0x04: "ABS_RY", 0x05: "ABS_RZ",
	0x10: "ABS_HAT0X", 0x11: "ABS_HAT0Y",
	0x12: "ABS_HAT1X", 0x13: "ABS_HAT1Y",
}

var buttonNames = map[uint32]string{
	0x120: "BTN_SOUTH", 0x121: "BTN_EAST", 0x122: "BTN_C", 0x123: "BTN_NORTH",
	0x124: "BTN_WEST", 0x125: "BTN_Z", 0x126: "BTN_TL", 0x127: "BTN_TR",
	0x128: "BTN_TL2", 0x129: "BTN_TR2", 0x12a: "BTN_SELECT", 0x12b: "BTN_START",
	0x12c: "BTN_MODE", 0x12d: "BTN_THUMBL", 0x12e: "BTN_THUMBR",
}

var keymap = map[byte]string{
	0x04: "A", 0x05: "B", 0x06: "C", 0x07: "D", 0x08: "E", 0x09: "F",
	0x0A: "G", 0x0B: "H", 0x0C: "I", 0x0D: "J", 0x0E: "K", 0x0F: "L",
	0x10: "M", 0x11: "N", 0x12: "O", 0x13: "P", 0x14: "Q", 0x15: "R",
	0x16: "S", 0x17: "T", 0x18: "U", 0x19: "V", 0x1A: "W", 0x1B: "X",
	0x1C: "Y", 0x1D: "Z", 0x1E: "1", 0x1F: "2", 0x20: "3", 0x21: "4",
	0x22: "5", 0x23: "6", 0x24: "7", 0x25: "8", 0x26: "9", 0x27: "0",
	0x28: "Enter", 0x29: "Esc", 0x2A: "Backspace", 0x2B: "Tab", 0x2C: "Space",
	0x4F: "Right", 0x50: "Left", 0x51: "Down", 0x52: "Up",
}

// Device structs
type Device struct {
	kind    string
	fd      int
	path    string
	name    string
	axes    map[string]int16
	buttons map[string]bool
	axmap   []uint32
	btnmap  []uint32
}

func decodeKeyboard(data []byte) string {
	if data[0] == 0x20 && len(data) >= 5 {
		k := data[len(data)-1]
		return fmt.Sprintf("KEYBOARD pressed: %s", keymap[k])
	}
	if len(data) == 8 {
		var pressed []string
		for _, code := range data[2:] {
			if code != 0 {
				pressed = append(pressed, keymap[code])
			}
		}
		if len(pressed) > 0 {
			return "KEYBOARD pressed: " + strings.Join(pressed, ", ")
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

func setupJS(path string) *Device {
	fd, err := unix.Open(path, os.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		fmt.Println("Cannot open js:", err)
		return nil
	}
	// Name
	nameBuf := make([]byte, 128)
	unix.IoctlGetInt(fd, JSIOCGNAME) // just trigger
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), uintptr(JSIOCGNAME), uintptr(unsafe.Pointer(&nameBuf[0])))
	if errno != 0 {
		return nil
	}
	name := string(bytes.TrimRight(nameBuf, "\x00"))

	// Axes/buttons
	axmap := make([]uint32, 64)
	btnmap := make([]uint32, 256)
	unix.Ioctl(fd, JSIOCGAXMAP, uintptr(unsafe.Pointer(&axmap[0])))
	unix.Ioctl(fd, JSIOCGBTNMAP, uintptr(unsafe.Pointer(&btnmap[0])))

	stateAxes := make(map[string]int16)
	for _, a := range axmap {
		stateAxes[axisNames[a]] = 0
	}
	stateButtons := make(map[string]bool)
	for _, b := range btnmap {
		stateButtons[buttonNames[b]] = false
	}

	fmt.Printf("Monitoring %s (joystick: %s)\n", path, name)
	return &Device{kind: "js", fd: fd, path: path, name: name, axes: stateAxes, buttons: stateButtons, axmap: axmap, btnmap: btnmap}
}

func setupHID(path string) *Device {
	fd, err := unix.Open(path, os.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil
	}
	fmt.Printf("Monitoring %s (hidraw)\n", path)
	return &Device{kind: "hid", fd: fd, path: path}
}

func handleJSEvent(d *Device, evt []byte) {
	var t uint32
	var val int16
	var etype, num uint8
	binary.Read(bytes.NewReader(evt), binary.LittleEndian, &t)
	binary.Read(bytes.NewReader(evt[4:]), binary.LittleEndian, &val)
	etype = evt[6]
	num = evt[7]
	etype &= ^JS_EVENT_INIT

	if etype == JS_EVENT_AXIS {
		axis := axisNames[d.axmap[num]]
		d.axes[axis] = val
	} else if etype == JS_EVENT_BUTTON {
		btn := buttonNames[d.btnmap[num]]
		d.buttons[btn] = val != 0
	}

	axStr := []string{}
	for k, v := range d.axes {
		axStr = append(axStr, fmt.Sprintf("%s=%d", k, v))
	}
	btnStr := []string{}
	for k, v := range d.buttons {
		if v {
			btnStr = append(btnStr, fmt.Sprintf("%s=1", k))
		} else {
			btnStr = append(btnStr, fmt.Sprintf("%s=0", k))
		}
	}
	fmt.Printf("[%s] Axes[%s] Buttons[%s]\n", d.name, strings.Join(axStr, ", "), strings.Join(btnStr, ", "))
}

func Run() {
	fmt.Println("Starting Spy monitor…")
	devices := []*Device{}

	// Initial scan
	filepath.Walk("/dev/input", func(path string, info os.FileInfo, err error) error {
		if strings.HasPrefix(filepath.Base(path), "js") {
			if dev := setupJS(path); dev != nil {
				devices = append(devices, dev)
			}
		}
		return nil
	})
	filepath.Walk("/dev", func(path string, info os.FileInfo, err error) error {
		if strings.HasPrefix(filepath.Base(path), "hidraw") {
			if dev := setupHID(path); dev != nil {
				devices = append(devices, dev)
			}
		}
		return nil
	})

	fds := []unix.PollFd{}
	for _, d := range devices {
		fds = append(fds, unix.PollFd{Fd: int32(d.fd), Events: unix.POLLIN})
	}

	for {
		_, err := unix.Poll(fds, -1)
		if err != nil {
			continue
		}
		for i, fd := range fds {
			if fd.Revents&unix.POLLIN != 0 {
				d := devices[i]
				if d.kind == "hid" {
					buf := make([]byte, ReportSize)
					n, _ := unix.Read(d.fd, buf)
					if n > 0 {
						if s := decodeKeyboard(buf[:n]); s != "" {
							fmt.Printf("[%s] %s\n", d.path, s)
						} else if s := decodeMouse(buf[:n]); s != "" {
							fmt.Printf("[%s] %s\n", d.path, s)
						}
					}
				} else {
					evt := make([]byte, 8)
					n, _ := unix.Read(d.fd, evt)
					if n == 8 {
						handleJSEvent(d, evt)
					}
				}
			}
		}
	}
}
