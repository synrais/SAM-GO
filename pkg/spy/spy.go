// pkg/spy/spy.go
package spy

import (
	"bytes"
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	REPORT_SIZE = 64
	DEADZONE    = 15

	// joystick constants
	JS_EVENT_BUTTON = 0x01
	JS_EVENT_AXIS   = 0x02
	JS_EVENT_INIT   = 0x80

	JSIOCGVERSION  = 0x80046a01
	JSIOCGAXES     = 0x80016a11
	JSIOCGBUTTONS  = 0x80016a12
	JSIOCGNAME     = 0x81006a13
	JSIOCGAXMAP    = 0x80406a32
	JSIOCGBTNMAP   = 0x80406a34
)

// axis + button maps (retrospy style)
var AXIS_NAMES = map[uint32]string{
	0x00: "ABS_X", 0x01: "ABS_Y", 0x02: "ABS_Z",
	0x03: "ABS_RX", 0x04: "ABS_RY", 0x05: "ABS_RZ",
	0x10: "ABS_HAT0X", 0x11: "ABS_HAT0Y",
	0x12: "ABS_HAT1X", 0x13: "ABS_HAT1Y",
}

var BUTTON_NAMES = map[uint32]string{
	0x120: "BTN_SOUTH", 0x121: "BTN_EAST", 0x122: "BTN_C", 0x123: "BTN_NORTH",
	0x124: "BTN_WEST", 0x125: "BTN_Z", 0x126: "BTN_TL", 0x127: "BTN_TR",
	0x128: "BTN_TL2", 0x129: "BTN_TR2", 0x12a: "BTN_SELECT", 0x12b: "BTN_START",
	0x12c: "BTN_MODE", 0x12d: "BTN_THUMBL", 0x12e: "BTN_THUMBR",
}

// small helper
func ioctl(fd int, req uintptr, arg unsafe.Pointer) error {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), req, uintptr(arg))
	if errno != 0 {
		return errno
	}
	return nil
}

// ---------------- joystick setup ----------------
type JSDevice struct {
	fd       int
	path     string
	name     string
	axes     map[string]int16
	buttons  map[string]uint8
	axmap    []uint32
	btnmap   []uint32
	version  uint32
	numAxes  int
	numBtns  int
}

func SetupJSDevice(fd int, path string) (*JSDevice, error) {
	var version uint32
	if err := ioctl(fd, JSIOCGVERSION, unsafe.Pointer(&version)); err != nil {
		return nil, err
	}

	var nAxes uint8
	if err := ioctl(fd, JSIOCGAXES, unsafe.Pointer(&nAxes)); err != nil {
		return nil, err
	}

	var nBtns uint8
	if err := ioctl(fd, JSIOCGBUTTONS, unsafe.Pointer(&nBtns)); err != nil {
		return nil, err
	}

	buf := make([]byte, 128)
	if err := ioctl(fd, JSIOCGNAME, unsafe.Pointer(&buf[0])); err != nil {
		return nil, err
	}
	name := string(buf[:bytes.IndexByte(buf, 0)])

	axmap := make([]uint32, nAxes)
	if err := ioctl(fd, JSIOCGAXMAP, unsafe.Pointer(&axmap[0])); err != nil {
		return nil, err
	}

	btnmap := make([]uint32, nBtns)
	if err := ioctl(fd, JSIOCGBTNMAP, unsafe.Pointer(&btnmap[0])); err != nil {
		return nil, err
	}

	fmt.Printf("Monitoring %s (joystick: %s)\n", path, name)
	fmt.Printf("  Driver version: %d\n", version)
	fmt.Printf("  Axes: %d  Buttons: %d\n", nAxes, nBtns)

	axes := make(map[string]int16)
	for _, a := range axmap {
		axes[AXIS_NAMES[a]] = 0
	}
	buttons := make(map[string]uint8)
	for _, b := range btnmap {
		buttons[BUTTON_NAMES[b]] = 0
	}

	return &JSDevice{
		fd:      fd,
		path:    path,
		name:    name,
		axes:    axes,
		buttons: buttons,
		axmap:   axmap,
		btnmap:  btnmap,
		version: version,
		numAxes: int(nAxes),
		numBtns: int(nBtns),
	}, nil
}

func (js *JSDevice) HandleEvent(evt []byte) {
	var t uint32
	var val int16
	var etype, num uint8
	_ = binary.Read(bytes.NewReader(evt), binary.LittleEndian, &t)
	_ = binary.Read(bytes.NewReader(evt[4:6]), binary.LittleEndian, &val)
	etype = evt[6]
	num = evt[7]

	etype &= ^uint8(JS_EVENT_INIT)

	if etype == JS_EVENT_AXIS {
		code := js.axmap[num]
		name := AXIS_NAMES[code]
		if name == "" {
			name = fmt.Sprintf("0x%x", code)
		}
		js.axes[name] = val
	} else if etype == JS_EVENT_BUTTON {
		code := js.btnmap[num]
		name := BUTTON_NAMES[code]
		if name == "" {
			name = fmt.Sprintf("0x%x", code)
		}
		if val != 0 {
			js.buttons[name] = 1
		} else {
			js.buttons[name] = 0
		}
	}

	// dump RetroSpy-style
	axs := ""
	for k, v := range js.axes {
		axs += fmt.Sprintf("%s=%d ", k, v)
	}
	btns := ""
	for k, v := range js.buttons {
		btns += fmt.Sprintf("%s=%d ", k, v)
	}
	fmt.Printf("[%s] Axes[%s] Buttons[%s]\n", js.name, axs, btns)
}

// ---------------- monitor ----------------
func MonitorAll() {
	var devices []*JSDevice

	// open /dev/input/js*
	entries, _ := filepath.Glob("/dev/input/js*")
	for _, path := range entries {
		fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NONBLOCK, 0)
		if err != nil {
			fmt.Println("open failed:", err)
			continue
		}
		js, err := SetupJSDevice(fd, path)
		if err != nil {
			fmt.Println("setup failed:", err)
			continue
		}
		devices = append(devices, js)
	}

	fds := make([]unix.PollFd, len(devices))
	for i, d := range devices {
		fds[i] = unix.PollFd{Fd: int32(d.fd), Events: unix.POLLIN}
	}

	for {
		_, err := unix.Poll(fds, -1)
		if err != nil {
			fmt.Println("poll failed:", err)
			break
		}
		for i, fd := range fds {
			if fd.Revents&unix.POLLIN != 0 {
				buf := make([]byte, 8)
				n, _ := unix.Read(devices[i].fd, buf)
				if n == 8 {
					devices[i].HandleEvent(buf)
				}
			}
		}
	}
}

