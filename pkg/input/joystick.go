package input

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	jsEventSize  = 8
	jsPollDelay  = 50 * time.Millisecond
	jsDBFile     = "/media/fat/gamecontrollerdb.txt"
	deadzone     = 15
)

// JoystickEvent represents a decoded joystick input event.
type JoystickEvent struct {
	Timestamp int64
	Device    string
	Buttons   map[int]int16
	Axes      map[int]int16
}

// mapping structures
type jsMapping struct {
	ButtonMap map[int]string
	AxisMap   map[int]string
}

// JoystickDevice holds state for a joystick.
type JoystickDevice struct {
	Path    string
	Name    string
	GUID    string
	FD      int
	Mapping jsMapping
	Buttons map[int]int16
	Axes    map[int]int16
}

func openJoystickDevice(path string, mapping jsMapping) (*JoystickDevice, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	name := filepath.Base(path)
	fmt.Printf("[+] Opened %s (fd=%d)\n", path, fd)
	return &JoystickDevice{
		Path:    path,
		Name:    name,
		FD:      fd,
		Mapping: mapping,
		Buttons: make(map[int]int16),
		Axes:    make(map[int]int16),
	}, nil
}

func (j *JoystickDevice) Close() {
	if j.FD >= 0 {
		_ = unix.Close(j.FD)
		fmt.Printf("[-] Closed %s\n", j.Path)
		j.FD = -1
	}
}

func (j *JoystickDevice) readEvents() bool {
	changed := false
	for {
		buf := make([]byte, jsEventSize)
		n, err := unix.Read(j.FD, buf)
		if err != nil || n < jsEventSize {
			break
		}
		// struct js_event: __u32 time; __s16 value; __u8 type; __u8 number
		t := *(*uint32)(unsafe.Pointer(&buf[0]))
		val := *(*int16)(unsafe.Pointer(&buf[4]))
		etype := buf[6] & 0x7F
		num := int(buf[7])

		switch etype {
		case 0x01: // button
			if j.Buttons[num] != val {
				j.Buttons[num] = val
				changed = true
			}
		case 0x02: // axis
			if j.Axes[num] != val {
				// apply deadzone
				centered := val
				if centered > -deadzone && centered < deadzone {
					centered = 0
				}
				j.Axes[num] = centered
				changed = true
			}
		}
		_ = t // we don’t use kernel timestamp
	}
	return changed
}

// SDL DB parsing (simplified, Linux-only)
func loadSDLDB(path string) map[string]jsMapping {
	out := map[string]jsMapping{}

	file, err := os.Open(path)
	if err != nil {
		return out
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, ",platform:Linux") {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) < 3 {
			continue
		}
		guid := strings.ToLower(parts[0])
		buttonMap := map[int]string{}
		axisMap := map[int]string{}
		for _, p := range parts[2:] {
			if !strings.Contains(p, ":") {
				continue
			}
			kv := strings.SplitN(p, ":", 2)
			if len(kv) != 2 {
				continue
			}
			k, v := kv[0], kv[1]
			if strings.HasPrefix(v, "b") {
				if idx, err := strconv.Atoi(v[1:]); err == nil {
					buttonMap[idx] = k
				}
			} else if strings.HasPrefix(v, "a") || strings.HasPrefix(v, "+a") || strings.HasPrefix(v, "-a") {
				idxStr := strings.TrimLeft(v, "+a-")
				if idx, err := strconv.Atoi(idxStr); err == nil {
					axisMap[idx] = k
				}
			}
		}
		out[guid] = jsMapping{ButtonMap: buttonMap, AxisMap: axisMap}
	}
	return out
}

// StreamJoysticks watches /dev/input/js* and streams events.
func StreamJoysticks() <-chan JoystickEvent {
	out := make(chan JoystickEvent, 100)

	go func() {
		defer close(out)
		devices := map[string]*JoystickDevice{}

		// load SDL DB once
		db := loadSDLDB(jsDBFile)
		if len(db) > 0 {
			fmt.Printf("Using SDL DB: %s\n", jsDBFile)
		} else {
			fmt.Println("No SDL DB found.")
		}

		// initial scan
		paths, _ := filepath.Glob("/dev/input/js*")
		for _, path := range paths {
			if dev, err := openJoystickDevice(path, jsMapping{}); err == nil {
				devices[path] = dev
			}
		}

		// inotify watch
		inFd, err := unix.InotifyInit()
		if err != nil {
			fmt.Println("inotify init failed:", err)
			return
		}
		defer unix.Close(inFd)
		_, _ = unix.InotifyAddWatch(inFd, "/dev/input", unix.IN_CREATE|unix.IN_DELETE)

		for {
			// build pollfds
			var pollfds []unix.PollFd
			for _, dev := range devices {
				if dev.FD >= 0 {
					pollfds = append(pollfds, unix.PollFd{Fd: int32(dev.FD), Events: unix.POLLIN})
				}
			}
			pollfds = append(pollfds, unix.PollFd{Fd: int32(inFd), Events: unix.POLLIN})

			n, err := unix.Poll(pollfds, int(jsPollDelay.Milliseconds()))
			if err != nil || n == 0 {
				continue
			}

			for _, pfd := range pollfds {
				if pfd.Fd == int32(inFd) && pfd.Revents&unix.POLLIN != 0 {
					// handle hotplug
					buf := make([]byte, 4096)
					n, _ := unix.Read(inFd, buf)
					offset := 0
					for offset < n {
						raw := (*unix.InotifyEvent)(unsafe.Pointer(&buf[offset]))
						nameBytes := buf[offset+unix.SizeofInotifyEvent : offset+unix.SizeofInotifyEvent+int(raw.Len)]
						name := string(nameBytes[:len(nameBytes)-1])
						path := filepath.Join("/dev/input", name)
						if strings.HasPrefix(name, "js") {
							if raw.Mask&unix.IN_CREATE != 0 {
								if dev, err := openJoystickDevice(path, jsMapping{}); err == nil {
									devices[path] = dev
								}
							}
							if raw.Mask&unix.IN_DELETE != 0 {
								if dev, ok := devices[path]; ok {
									dev.Close()
									delete(devices, path)
								}
							}
						}
						offset += unix.SizeofInotifyEvent + int(raw.Len)
					}
				}
				if pfd.Fd != int32(inFd) && pfd.Revents&unix.POLLIN != 0 {
					var dev *JoystickDevice
					for _, d := range devices {
						if int32(d.FD) == pfd.Fd {
							dev = d
							break
						}
					}
					if dev != nil && dev.readEvents() {
						out <- JoystickEvent{
							Timestamp: time.Now().UnixMilli(),
							Device:    filepath.Base(dev.Path),
							Buttons:   copyBtn(dev.Buttons),
							Axes:      copyAxes(dev.Axes),
						}
					}
				}
			}
		}
	}()

	return out
}

func copyBtn(src map[int]int16) map[int]int16 {
	dst := make(map[int]int16)
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
func copyAxes(src map[int]int16) map[int]int16 {
	dst := make(map[int]int16)
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
