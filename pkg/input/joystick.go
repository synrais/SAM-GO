package input

import (
	"fmt"
	"path/filepath"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	jsEventSize = 8
	jsPollDelay = 50 * time.Millisecond
)

// JoystickEvent represents a decoded joystick input event.
type JoystickEvent struct {
	Timestamp int64
	Device    string
	Buttons   map[int]int16
	Axes      map[int]int16
}

// JoystickDevice holds state for a joystick.
type JoystickDevice struct {
	Path    string
	FD      int
	Buttons map[int]int16
	Axes    map[int]int16
}

func openJoystickDevice(path string) (*JoystickDevice, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	fmt.Printf("[+] Opened %s (fd=%d)\n", path, fd)
	return &JoystickDevice{
		Path:    path,
		FD:      fd,
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
				j.Axes[num] = val
				changed = true
			}
		}
		_ = t // kernel timestamp not used
	}
	return changed
}

// StreamJoysticks watches /sys/class/input for js* devices and streams events.
func StreamJoysticks() <-chan JoystickEvent {
	out := make(chan JoystickEvent, 100)

	go func() {
		defer close(out)
		devices := map[string]*JoystickDevice{}

		// initial scan
		paths, _ := filepath.Glob("/dev/input/js*")
		for _, path := range paths {
			if dev, err := openJoystickDevice(path); err == nil {
				devices[path] = dev
			}
		}

		// inotify watch on /sys/class/input
		inFd, err := unix.InotifyInit()
		if err != nil {
			fmt.Println("inotify init failed:", err)
			return
		}
		defer unix.Close(inFd)
		_, _ = unix.InotifyAddWatch(inFd, "/sys/class/input", unix.IN_CREATE|unix.IN_DELETE)

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
				// handle sysfs hotplug events
				if pfd.Fd == int32(inFd) && pfd.Revents&unix.POLLIN != 0 {
					buf := make([]byte, 4096)
					n, _ := unix.Read(inFd, buf)
					offset := 0
					for offset < n {
						raw := (*unix.InotifyEvent)(unsafe.Pointer(&buf[offset]))
						nameBytes := buf[offset+unix.SizeofInotifyEvent : offset+unix.SizeofInotifyEvent+int(raw.Len)]
						name := string(nameBytes[:len(nameBytes)-1])

						if len(name) >= 2 && name[:2] == "js" {
							path := filepath.Join("/dev/input", name)
							if raw.Mask&unix.IN_CREATE != 0 {
								if dev, err := openJoystickDevice(path); err == nil {
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

				// handle joystick input events
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
