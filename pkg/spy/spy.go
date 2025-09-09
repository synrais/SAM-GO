package spy

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/fsnotify/fsnotify"
	"golang.org/x/sys/unix"
)

const (
	ReportSize  = 64
	Deadzone    = 15
	JS_EVENT_BUTTON = 0x01
	JS_EVENT_AXIS   = 0x02
	JS_EVENT_INIT   = 0x80
)

type Device struct {
	fd     int
	path   string
	kind   string // "js" or "hid"
	name   string
	axes   map[string]int16
	buttons map[string]bool
	axmap  []uint32
	btnmap []uint32
}

// ---------------- Keyboard/mouse maps (from Python) ----------------
var Keymap = map[byte]string{
	0x04: "A", 0x05: "B", 0x06: "C", 0x07: "D", 0x08: "E", 0x09: "F",
	0x0A: "G", 0x0B: "H", 0x0C: "I", 0x0D: "J", 0x0E: "K", 0x0F: "L",
	0x10: "M", 0x11: "N", 0x12: "O", 0x13: "P", 0x14: "Q", 0x15: "R",
	0x16: "S", 0x17: "T", 0x18: "U", 0x19: "V", 0x1A: "W", 0x1B: "X",
	0x1C: "Y", 0x1D: "Z", 0x1E: "1", 0x1F: "2", 0x20: "3", 0x21: "4",
	0x22: "5", 0x23: "6", 0x24: "7", 0x25: "8", 0x26: "9", 0x27: "0",
	0x28: "Enter", 0x29: "Esc", 0x2A: "Backspace", 0x2B: "Tab", 0x2C: "Space",
	0x4F: "Right", 0x50: "Left", 0x51: "Down", 0x52: "Up",
}

// ---------------- Joystick setup (RetroSpy style) ----------------
func setupJsDevice(path string) (*Device, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}

	// Get name
	buf := make([]byte, 128)
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(unix.JSIOCGNAME(len(buf))), uintptr((unsafe.Pointer(&buf[0])))); errno != 0 {
		return nil, errno
	}
	name := string(buf[:bytes.IndexByte(buf, 0)])

	fmt.Printf("Monitoring %s (joystick: %s)\n", path, name)
	return &Device{
		fd: fd, path: path, kind: "js", name: name,
		axes: make(map[string]int16), buttons: make(map[string]bool),
	}, nil
}

// ---------------- HID decoding ----------------
func decodeHidraw(path string, data []byte) string {
	if len(data) == 8 { // Keyboard report
		var pressed []string
		for _, code := range data[2:] {
			if code != 0 {
				if key, ok := Keymap[code]; ok {
					pressed = append(pressed, key)
				} else {
					pressed = append(pressed, fmt.Sprintf("0x%02x", code))
				}
			}
		}
		if len(pressed) > 0 {
			return fmt.Sprintf("KEYBOARD pressed: %s", strings.Join(pressed, ", "))
		}
		return "KEYBOARD released"
	}
	if len(data) >= 4 { // Mouse
		buttons := data[1]
		x := int16(binary.LittleEndian.Uint16(data[2:4]))
		y := int16(binary.LittleEndian.Uint16(data[4:6]))
		if buttons == 0 && x == 0 && y == 0 {
			return ""
		}
		return fmt.Sprintf("MOUSE buttons=0x%x dx=%d dy=%d", buttons, x, y)
	}
	return fmt.Sprintf("HIDRAW raw: %v", data)
}

// ---------------- Event loop ----------------
func handleJsEvent(dev *Device) {
	buf := make([]byte, 8)
	for {
		n, err := unix.Read(dev.fd, buf)
		if err != nil {
			if err == unix.EAGAIN {
				return
			}
			fmt.Printf("%s closed\n", dev.path)
			unix.Close(dev.fd)
			return
		}
		if n != 8 {
			return
		}
		var t uint32
		var val int16
		var etype, num byte
		binary.Read(bytes.NewReader(buf), binary.LittleEndian, &t)
		binary.Read(bytes.NewReader(buf[4:6]), binary.LittleEndian, &val)
		etype = buf[6]
		num = buf[7]
		etype &= ^JS_EVENT_INIT

		if etype == JS_EVENT_AXIS {
			dev.axes[fmt.Sprintf("AXIS_%d", num)] = val
		} else if etype == JS_EVENT_BUTTON {
			dev.buttons[fmt.Sprintf("BTN_%d", num)] = val != 0
		}

		var axs []string
		for k, v := range dev.axes {
			axs = append(axs, fmt.Sprintf("%s=%d", k, v))
		}
		var btns []string
		for k, v := range dev.buttons {
			btns = append(btns, fmt.Sprintf("%s=%t", k, v))
		}
		fmt.Printf("[%s] Axes[%s] Buttons[%s]\n", dev.name, strings.Join(axs, ", "), strings.Join(btns, ", "))
	}
}

func handleHidEvent(dev *Device) {
	buf := make([]byte, ReportSize)
	n, err := unix.Read(dev.fd, buf)
	if err != nil || n == 0 {
		return
	}
	msg := decodeHidraw(dev.path, buf[:n])
	if msg != "" {
		fmt.Printf("[%s] %s\n", dev.path, msg)
	}
}

// ---------------- Hotplug watcher ----------------
func Run(args []string) {
	fmt.Println("Starting Spy monitor…")
	var mu sync.Mutex
	devices := make(map[int]*Device)

	openDevice := func(path string) {
		if strings.HasPrefix(filepath.Base(path), "js") {
			if dev, err := setupJsDevice(path); err == nil {
				mu.Lock()
				devices[dev.fd] = dev
				mu.Unlock()
			}
		} else if strings.HasPrefix(filepath.Base(path), "hidraw") {
			fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NONBLOCK, 0)
			if err == nil {
				fmt.Printf("Monitoring %s (hidraw)\n", path)
				devices[fd] = &Device{fd: fd, path: path, kind: "hid"}
			}
		}
	}

	// Initial scan
	files, _ := filepath.Glob("/dev/input/js*")
	for _, f := range files {
		openDevice(f)
	}
	files, _ = filepath.Glob("/dev/hidraw*")
	for _, f := range files {
		openDevice(f)
	}

	// Watch /dev and /dev/input for hotplug
	watcher, _ := fsnotify.NewWatcher()
	defer watcher.Close()
	watcher.Add("/dev")
	watcher.Add("/dev/input")

	go func() {
		for ev := range watcher.Events {
			if ev.Op&(fsnotify.Create) != 0 {
				openDevice(ev.Name)
			}
			if ev.Op&(fsnotify.Remove) != 0 {
				// Device removed
				mu.Lock()
				for fd, d := range devices {
					if d.path == ev.Name {
						unix.Close(fd)
						delete(devices, fd)
						fmt.Printf("Closed %s\n", ev.Name)
					}
				}
				mu.Unlock()
			}
		}
	}()

	// Poll loop
	for {
		mu.Lock()
		var fds []unix.PollFd
		for fd := range devices {
			fds = append(fds, unix.PollFd{Fd: int32(fd), Events: unix.POLLIN})
		}
		mu.Unlock()
		if len(fds) == 0 {
			continue
		}
		_, err := unix.Poll(fds, -1)
		if err != nil {
			continue
		}
		mu.Lock()
		for _, p := range fds {
			if p.Revents&unix.POLLIN != 0 {
				if dev, ok := devices[int(p.Fd)]; ok {
					if dev.kind == "js" {
						handleJsEvent(dev)
					} else {
						handleHidEvent(dev)
					}
				}
			}
		}
		mu.Unlock()
	}
}
