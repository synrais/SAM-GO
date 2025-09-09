package spy

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	reportSize   = 64
	deadzone     = 15
	scanInterval = 2 * time.Second

	// joystick event types
	jsEventButton = 0x01
	jsEventAxis   = 0x02
	jsEventInit   = 0x80

	// ioctl constants
	JSIOCGVERSION = 0x80046a01
	JSIOCGAXES    = 0x80016a11
	JSIOCGBUTTONS = 0x80016a12
	JSIOCGNAME    = 0x81006a13
	JSIOCGAXMAP   = 0x80406a32
	JSIOCGBTNMAP  = 0x80406a34
)

var axisNames = map[uint32]string{
	0x00: "ABS_X", 0x01: "ABS_Y", 0x02: "ABS_Z",
	0x03: "ABS_RX", 0x04: "ABS_RY", 0x05: "ABS_RZ",
	0x10: "ABS_HAT0X", 0x11: "ABS_HAT0Y",
	0x12: "ABS_HAT1X", 0x13: "ABS_HAT1Y",
}
var buttonNames = map[uint32]string{
	0x120: "BTN_SOUTH", 0x121: "BTN_EAST", 0x123: "BTN_NORTH", 0x124: "BTN_WEST",
	0x126: "BTN_TL", 0x127: "BTN_TR", 0x128: "BTN_TL2", 0x129: "BTN_TR2",
	0x12a: "BTN_SELECT", 0x12b: "BTN_START", 0x12c: "BTN_MODE",
	0x12d: "BTN_THUMBL", 0x12e: "BTN_THUMBR",
}

var keymap = map[byte]string{
	0x04: "A", 0x05: "B", 0x06: "C", 0x07: "D",
	0x08: "E", 0x09: "F", 0x0A: "G", 0x0B: "H",
	0x0C: "I", 0x0D: "J", 0x0E: "K", 0x0F: "L",
	0x10: "M", 0x11: "N", 0x12: "O", 0x13: "P",
	0x14: "Q", 0x15: "R", 0x16: "S", 0x17: "T",
	0x18: "U", 0x19: "V", 0x1A: "W", 0x1B: "X",
	0x1C: "Y", 0x1D: "Z",
	0x1E: "1", 0x1F: "2", 0x20: "3", 0x21: "4", 0x22: "5",
	0x23: "6", 0x24: "7", 0x25: "8", 0x26: "9", 0x27: "0",
	0x28: "Enter", 0x29: "Esc", 0x2A: "Backspace",
	0x2B: "Tab", 0x2C: "Space",
	0x4F: "Right", 0x50: "Left", 0x51: "Down", 0x52: "Up",
}

type device struct {
	fd      *os.File
	path    string
	kind    string
	name    string
	axmap   []uint32
	btnmap  []uint32
	axes    map[string]int16
	buttons map[string]byte
}

func ioctl(fd uintptr, req, arg uintptr) error {
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, req, arg)
	if e != 0 {
		return e
	}
	return nil
}

func setupJoystick(path string) (*device, error) {
	fd, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}

	nameBuf := make([]byte, 128)
	if err := ioctl(fd.Fd(), JSIOCGNAME, uintptr(unsafe.Pointer(&nameBuf[0]))); err != nil {
		fd.Close()
		return nil, err
	}
	name := strings.TrimRight(string(nameBuf), "\x00")

	axBuf := make([]byte, 64)
	if err := ioctl(fd.Fd(), JSIOCGAXMAP, uintptr(unsafe.Pointer(&axBuf[0]))); err != nil {
		fd.Close()
		return nil, err
	}
	axmap := make([]uint32, len(axBuf)/4)
	_ = binary.Read(bytes.NewReader(axBuf), binary.LittleEndian, &axmap)

	btnBuf := make([]byte, 512)
	if err := ioctl(fd.Fd(), JSIOCGBTNMAP, uintptr(unsafe.Pointer(&btnBuf[0]))); err != nil {
		fd.Close()
		return nil, err
	}
	btnmap := make([]uint32, len(btnBuf)/4)
	_ = binary.Read(bytes.NewReader(btnBuf), binary.LittleEndian, &btnmap)

	fmt.Printf("Monitoring %s (joystick: %s)\n", path, name)

	return &device{
		fd:      fd,
		path:    path,
		kind:    "js",
		name:    name,
		axmap:   axmap,
		btnmap:  btnmap,
		axes:    make(map[string]int16),
		buttons: make(map[string]byte),
	}, nil
}

func handleJSEvent(d *device, evt []byte) {
	var t uint32
	var val int16
	var etype, num uint8
	_ = binary.Read(bytes.NewReader(evt), binary.LittleEndian, &t)
	_ = binary.Read(bytes.NewReader(evt[4:]), binary.LittleEndian, &val)
	etype = evt[6]
	num = evt[7]

	// fix overflow by casting
	if etype&jsEventInit != 0 {
		etype &= ^uint8(jsEventInit)
	}

	if etype == jsEventAxis {
		ax := axisNames[d.axmap[num]]
		if ax == "" {
			ax = fmt.Sprintf("AXIS_%d", num)
		}
		d.axes[ax] = val
	} else if etype == jsEventButton {
		btn := buttonNames[d.btnmap[num]]
		if btn == "" {
			btn = fmt.Sprintf("BTN_%d", num)
		}
		d.buttons[btn] = byte(val)
	}
	axs := []string{}
	for k, v := range d.axes {
		axs = append(axs, fmt.Sprintf("%s=%d", k, v))
	}
	btns := []string{}
	for k, v := range d.buttons {
		btns = append(btns, fmt.Sprintf("%s=%d", k, v))
	}
	fmt.Printf("[%s] Axes[%s] Buttons[%s]\n", d.name, strings.Join(axs, ", "), strings.Join(btns, ", "))
}

func handleHIDRaw(d *device) {
	buf := make([]byte, reportSize)
	n, err := d.fd.Read(buf)
	if err != nil || n == 0 {
		return
	}
	data := buf[:n]
	if len(data) == 8 {
		pressed := []string{}
		for _, c := range data[2:] {
			if c != 0 {
				pressed = append(pressed, keymap[c])
			}
		}
		if len(pressed) > 0 {
			fmt.Printf("[%s] KEYBOARD pressed: %s\n", d.path, strings.Join(pressed, ", "))
		}
	}
}

func monitor(devs map[string]*device) {
	for {
		for path, d := range devs {
			if d.kind == "js" {
				buf := make([]byte, 8)
				for {
					_, err := d.fd.Read(buf)
					if err == io.EOF || (err != nil && err != syscall.EAGAIN) {
						break
					}
					if err != nil {
						break
					}
					handleJSEvent(d, buf)
				}
			} else {
				handleHIDRaw(d)
			}
			if _, err := os.Stat(path); os.IsNotExist(err) {
				d.fd.Close()
				delete(devs, path)
				fmt.Println("Removed:", path)
			}
		}
		// hotplug: rescan
		filepath.Walk("/dev", func(path string, info os.FileInfo, err error) error {
			if strings.HasPrefix(path, "/dev/input/js") || strings.HasPrefix(path, "/dev/hidraw") {
				if _, ok := devs[path]; !ok {
					if strings.Contains(path, "js") {
						jsdev, err := setupJoystick(path)
						if err == nil {
							devs[path] = jsdev
						}
					} else {
						fd, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
						if err == nil {
							fmt.Println("Monitoring", path, "(hidraw)")
							devs[path] = &device{fd: fd, path: path, kind: "hid"}
						}
					}
				}
			}
			return nil
		})
		time.Sleep(scanInterval)
	}
}

func Run(args []string) {
	fmt.Println("Starting Spy monitor…")
	devs := make(map[string]*device)
	monitor(devs)
}

