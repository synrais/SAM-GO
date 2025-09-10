package input

import (
	"fmt"
	"path/filepath"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	jsEventSize           = 8
	jsReadFrequency       = 50 * time.Millisecond
	hotplugScanInterval   = 2 * time.Second
)

// JoystickEvent represents a decoded joystick state snapshot.
type JoystickEvent struct {
	Timestamp int64
	Device    string
	Buttons   map[int]int16
	Axes      map[int]int16
}

// JoystickDevice holds state for a joystick.
type JoystickDevice struct {
	Path    string
	Buttons map[int]int16
	Axes    map[int]int16
	FD      int
}

func openJoystickDevice(path string) (*JoystickDevice, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	fmt.Printf("[+] Opened %s\n", path)
	return &JoystickDevice{
		Path:    path,
		FD:      fd,
		Buttons: make(map[int]int16),
		Axes:    make(map[int]int16),
	}, nil
}

func (j *JoystickDevice) close() {
	if j.FD >= 0 {
		_ = unix.Close(j.FD)
		j.FD = -1
	}
}

func (j *JoystickDevice) reopen() {
	j.close()
	fd, err := unix.Open(j.Path, unix.O_RDONLY|unix.O_NONBLOCK, 0)
	if err == nil {
		j.FD = fd
	}
}

func (j *JoystickDevice) readEvents() bool {
	if j.FD < 0 {
		return false
	}
	changed := false
	for {
		buf := make([]byte, jsEventSize)
		n, err := unix.Read(j.FD, buf)
		if err != nil || n < jsEventSize {
			break
		}
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
	}
	return changed
}

// StreamJoysticks streams events for all /dev/input/js* devices.
func StreamJoysticks() <-chan JoystickEvent {
	out := make(chan JoystickEvent, 100)

	go func() {
		defer close(out)
		devices := map[string]*JoystickDevice{}
		lastScan := time.Now()

		for {
			now := time.Now()
			// rescan hotplug
			if now.Sub(lastScan) > hotplugScanInterval {
				lastScan = now
				paths, _ := filepath.Glob("/dev/input/js*")
				for _, path := range paths {
					if _, ok := devices[path]; !ok {
						if dev, err := openJoystickDevice(path); err == nil {
							devices[path] = dev
						}
					}
				}
				// remove vanished
				for path, dev := range devices {
					if _, err := unix.Access(path, unix.F_OK); err != nil {
						dev.close()
						delete(devices, path)
					}
				}
			}

			// poll devices
			for _, dev := range devices {
				if dev.FD < 0 {
					if err := unix.Access(dev.Path, unix.F_OK); err == nil {
						dev.reopen()
					}
					continue
				}
				if dev.readEvents() {
					out <- JoystickEvent{
						Timestamp: time.Now().UnixMilli(),
						Device:    filepath.Base(dev.Path),
						Buttons:   copyBtn(dev.Buttons),
						Axes:      copyAxes(dev.Axes),
					}
				}
				// always reopen quietly (like Python)
				dev.reopen()
			}

			time.Sleep(jsReadFrequency)
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
