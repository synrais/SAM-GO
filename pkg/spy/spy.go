// pkg/spy/spy.go
package spy

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

const (
	ReportSize = 64
	Deadzone   = 15

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

// RetroSpy style name maps
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

// ---------------- Joystick ----------------
type JSDevice struct {
	fd      *os.File
	name    string
	axmap   []uint32
	btnmap  []uint32
	axes    map[string]int16
	buttons map[string]uint8
}

func setupJSDevice(path string) (*JSDevice, error) {
	fd, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}

	// version
	var version uint32
	if err := unix.IoctlGetUint32(int(fd.Fd()), JSIOCGVERSION, &version); err != nil {
		return nil, err
	}

	// num axes
	buf1 := make([]byte, 1)
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd.Fd(), JSIOCGAXES, uintptr(unsafe.Pointer(&buf1[0]))); errno != 0 {
		return nil, fmt.Errorf("JSIOCGAXES: %v", errno)
	}
	numAxes := int(buf1[0])

	// num buttons
	buf2 := make([]byte, 1)
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd.Fd(), JSIOCGBUTTONS, uintptr(unsafe.Pointer(&buf2[0]))); errno != 0 {
		return nil, fmt.Errorf("JSIOCGBUTTONS: %v", errno)
	}
	numButtons := int(buf2[0])

	// name
	nameBuf := make([]byte, 128)
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd.Fd(), JSIOCGNAME, uintptr(unsafe.Pointer(&nameBuf[0]))); errno != 0 {
		return nil, fmt.Errorf("JSIOCGNAME: %v", errno)
	}
	name := strings.TrimRight(string(nameBuf), "\x00")

	// axis map
	axBuf := make([]byte, numAxes*4)
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd.Fd(), JSIOCGAXMAP, uintptr(unsafe.Pointer(&axBuf[0]))); errno != 0 {
		return nil, fmt.Errorf("JSIOCGAXMAP: %v", errno)
	}
	axmap := make([]uint32, numAxes)
	for i := 0; i < numAxes; i++ {
		axmap[i] = binary.LittleEndian.Uint32(axBuf[i*4:])
	}

	// button map
	btnBuf := make([]byte, numButtons*4)
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd.Fd(), JSIOCGAXMAP, uintptr(unsafe.Pointer(&btnBuf[0]))); errno != 0 {
		return nil, fmt.Errorf("JSIOCGAXMAP: %v", errno)
	}
	btnmap := make([]uint32, numButtons)
	for i := 0; i < numButtons; i++ {
		btnmap[i] = binary.LittleEndian.Uint32(btnBuf[i*4:])
	}

	fmt.Printf("Monitoring %s (joystick: %s)\n", path, name)
	fmt.Printf("  Driver version: %d\n", version)
	fmt.Printf("  Axes: %d  Buttons: %d\n", numAxes, numButtons)

	axes := make(map[string]int16)
	for _, a := range axmap {
		axes[axisNames[a]] = 0
	}
	buttons := make(map[string]uint8)
	for _, b := range btnmap {
		buttons[buttonNames[b]] = 0
	}

	return &JSDevice{fd, name, axmap, btnmap, axes, buttons}, nil
}

func (js *JSDevice) handleEvent(evt []byte) {
	var time uint32 = binary.LittleEndian.Uint32(evt[0:4])
	value := int16(binary.LittleEndian.Uint16(evt[4:6]))
	etype := evt[6] & ^uint8(JS_EVENT_INIT)
	num := evt[7]

	if etype == JS_EVENT_AXIS {
		a := js.axmap[num]
		js.axes[axisNames[a]] = value
	} else if etype == JS_EVENT_BUTTON {
		b := js.btnmap[num]
		js.buttons[buttonNames[b]] = uint8(value)
	}

	axstr := []string{}
	for k, v := range js.axes {
		axstr = append(axstr, fmt.Sprintf("%s=%d", k, v))
	}
	btnstr := []string{}
	for k, v := range js.buttons {
		btnstr = append(btnstr, fmt.Sprintf("%s=%d", k, v))
	}
	fmt.Printf("[%s] Axes[%s] Buttons[%s] @%dms\n",
		js.name, strings.Join(axstr, ", "), strings.Join(btnstr, ", "), time)
}

// ---------------- HID ----------------
func decodeHID(data []byte) string {
	// TODO: implement your keyboard/mouse/gamepad decode from Python
	return fmt.Sprintf("HID raw: %x", data)
}

// ---------------- Monitor ----------------
func Run() {
	// hidraw devices
	hids, _ := filepath.Glob("/dev/hidraw*")
	for _, path := range hids {
		fmt.Printf("Monitoring %s (hidraw)\n", path)
	}

	// joystick devices
	jsdevs := []*JSDevice{}
	jslist, _ := filepath.Glob("/dev/input/js*")
	for _, path := range jslist {
		js, err := setupJSDevice(path)
		if err != nil {
			fmt.Println("Error setting up", path, ":", err)
			continue
		}
		jsdevs = append(jsdevs, js)
	}

	// event loop
	for {
		for _, js := range jsdevs {
			buf := make([]byte, 8)
			n, _ := js.fd.Read(buf)
			if n == 8 {
				js.handleEvent(buf)
			}
		}
	}
}
