package spy

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// Constants from linux/joystick.h
const (
	jsEventButton = 0x01
	jsEventAxis   = 0x02
	jsEventInit   = 0x80

	jsIOCGVersion  = 0x80046a01
	jsIOCGAxes     = 0x80016a11
	jsIOCButtons   = 0x80016a12
	jsIOCGName     = 0x81006a13
	jsIOCGAXMAP    = 0x80406a32
	jsIOCGBTNMAP   = 0x80406a34
	JSIOCGBTNMAP_L = 0x80406a34
	JSIOCGBTNMAP_S = 0x80406a33
)

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

type jsEvent struct {
	Time   uint32
	Value  int16
	Type   uint8
	Number uint8
}

type jsDevice struct {
	fd       *os.File
	path     string
	name     string
	axes     map[string]int16
	buttons  map[string]bool
	axmap    []uint32
	btnmap   []uint32
	numAxes  int
	numBtns  int
}

func ioctl(fd uintptr, cmd uintptr, arg unsafe.Pointer) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, cmd, uintptr(arg))
	if errno != 0 {
		return errno
	}
	return nil
}

func setupJS(path string) (*jsDevice, error) {
	fd, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}

	// Name
	nameBuf := make([]byte, 128)
	if err := ioctl(fd.Fd(), jsIOCGName, unsafe.Pointer(&nameBuf[0])); err != nil {
		return nil, err
	}
	name := strings.TrimRight(string(nameBuf), "\x00")

	// Num axes
	var numAxes uint8
	if err := ioctl(fd.Fd(), jsIOCGAxes, unsafe.Pointer(&numAxes)); err != nil {
		return nil, err
	}

	// Num buttons
	var numBtns uint8
	if err := ioctl(fd.Fd(), jsIOCButtons, unsafe.Pointer(&numBtns)); err != nil {
		return nil, err
	}

	// Axis map
	axmap := make([]uint32, numAxes)
	if err := ioctl(fd.Fd(), jsIOCGAXMAP, unsafe.Pointer(&axmap[0])); err != nil {
		return nil, err
	}

	// Button map (try multiple ioctl variants)
	btnmap := make([]uint32, numBtns)
	if err := ioctl(fd.Fd(), jsIOCGBTNMAP, unsafe.Pointer(&btnmap[0])); err != nil {
		// try fallback
		ioctl(fd.Fd(), JSIOCGBTNMAP_L, unsafe.Pointer(&btnmap[0]))
		ioctl(fd.Fd(), JSIOCGBTNMAP_S, unsafe.Pointer(&btnmap[0]))
	}

	fmt.Printf("Monitoring %s (joystick: %s)\n", path, name)
	fmt.Printf("  Axes: %d  Buttons: %d\n", numAxes, numBtns)

	axes := make(map[string]int16)
	for _, a := range axmap {
		axes[axisNames[a]] = 0
	}
	buttons := make(map[string]bool)
	for _, b := range btnmap {
		buttons[buttonNames[b]] = false
	}

	return &jsDevice{fd, path, name, axes, buttons, axmap, btnmap, int(numAxes), int(numBtns)}, nil
}

func (dev *jsDevice) readEvents() {
	buf := make([]byte, int(unsafe.Sizeof(jsEvent{})))
	for {
		n, err := dev.fd.Read(buf)
		if n <= 0 || err != nil {
			break
		}
		var e jsEvent
		binary.Read(bytes.NewReader(buf), binary.LittleEndian, &e)
		etype := e.Type & ^uint8(jsEventInit)

		if etype == jsEventAxis && int(e.Number) < len(dev.axmap) {
			name := axisNames[dev.axmap[e.Number]]
			if name == "" {
				name = fmt.Sprintf("AXIS_%d", e.Number)
			}
			dev.axes[name] = e.Value
		} else if etype == jsEventButton && int(e.Number) < len(dev.btnmap) {
			name := buttonNames[dev.btnmap[e.Number]]
			if name == "" {
				name = fmt.Sprintf("BTN_%d", e.Number)
			}
			dev.buttons[name] = e.Value != 0
		}

		axOut := []string{}
		for k, v := range dev.axes {
			axOut = append(axOut, fmt.Sprintf("%s=%d", k, v))
		}
		btnOut := []string{}
		for k, v := range dev.buttons {
			val := 0
			if v {
				val = 1
			}
			btnOut = append(btnOut, fmt.Sprintf("%s=%d", k, val))
		}
		fmt.Printf("[%s] Axes[%s] Buttons[%s]\n", dev.name, strings.Join(axOut, ", "), strings.Join(btnOut, ", "))
	}
}

// HID device (just raw dump for now)
type hidDevice struct {
	fd   *os.File
	path string
}

func setupHID(path string) (*hidDevice, error) {
	fd, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	fmt.Printf("Monitoring %s (hidraw)\n", path)
	return &hidDevice{fd, path}, nil
}

func (dev *hidDevice) readEvents() {
	buf := make([]byte, 64)
	for {
		n, err := dev.fd.Read(buf)
		if n <= 0 || err != nil {
			break
		}
		fmt.Printf("[%s] RAW: % x\n", dev.path, buf[:n])
	}
}

func Run(args []string) {
	fmt.Println("Starting Spy monitor…")

	jsdevs := []*jsDevice{}
	hiddevs := []*hidDevice{}

	// Initial scan
	scan := func() {
		filepath.Walk("/dev/input", func(path string, info os.FileInfo, err error) error {
			if strings.HasPrefix(filepath.Base(path), "js") {
				for _, d := range jsdevs {
					if d.path == path {
						return nil
					}
				}
				if dev, err := setupJS(path); err == nil {
					jsdevs = append(jsdevs, dev)
				}
			}
			return nil
		})
		filepath.Walk("/dev", func(path string, info os.FileInfo, err error) error {
			if strings.HasPrefix(filepath.Base(path), "hidraw") {
				for _, d := range hiddevs {
					if d.path == path {
						return nil
					}
				}
				if dev, err := setupHID(path); err == nil {
					hiddevs = append(hiddevs, dev)
				}
			}
			return nil
		})
	}
	scan()

	// Poll loop
	for {
		for _, d := range jsdevs {
			d.readEvents()
		}
		for _, d := range hiddevs {
			d.readEvents()
		}
		time.Sleep(10 * time.Millisecond)
		scan()
	}
}
