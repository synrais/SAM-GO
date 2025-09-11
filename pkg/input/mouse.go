package input

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	reportSize = 3
)

// MouseEvent is a decoded mouse packet
type MouseEvent struct {
	Timestamp int64
	Device    string
	Buttons   []string
	DX, DY    int8
}

// direction converts x and y deltas into a human readable description such as
// "up", "down-right", etc. A zero movement returns "none".
func direction(dx, dy int8) string {
	var dir string

	if dy < 0 {
		dir += "up"
	} else if dy > 0 {
		dir += "down"
	}

	if dx < 0 {
		if dir != "" {
			dir += "-"
		}
		dir += "left"
	} else if dx > 0 {
		if dir != "" {
			dir += "-"
		}
		dir += "right"
	}

	if dir == "" {
		dir = "none"
	}

	return dir
}

// String renders a human-friendly representation of the mouse event.
func (e MouseEvent) String() string {
	return fmt.Sprintf("[%d ms] %s: Buttons[%s] Move[%s]",
		e.Timestamp,
		e.Device,
		strings.Join(e.Buttons, ","),
		direction(e.DX, e.DY))
}

type MouseDevice struct {
	Path string
	FD   int
}

func openMouseDevice(path string) (*MouseDevice, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	fmt.Printf("[+] Opened %s (fd=%d)\n", path, fd)
	return &MouseDevice{Path: path, FD: fd}, nil
}

func (m *MouseDevice) Close() {
	if m.FD >= 0 {
		_ = unix.Close(m.FD)
		fmt.Printf("[-] Closed %s\n", m.Path)
		m.FD = -1
	}
}

// StreamMouse returns a channel of MouseEvent.
// It starts goroutines to monitor /dev/input/mouse* and /dev/input/mice.
func StreamMouse() <-chan MouseEvent {
	out := make(chan MouseEvent, 100) // buffered channel

	go func() {
		defer close(out)
		devices := map[string]*MouseDevice{}

		// initial scan
		paths, _ := filepath.Glob("/dev/input/mouse*")
		if _, err := os.Stat("/dev/input/mice"); err == nil {
			paths = append(paths, "/dev/input/mice")
		}
		for _, path := range paths {
			if dev, err := openMouseDevice(path); err == nil {
				devices[path] = dev
			}
		}

		// inotify watch on /dev/input
		inFd, err := unix.InotifyInit()
		if err != nil {
			fmt.Println("inotify init failed:", err)
			return
		}
		defer unix.Close(inFd)

		_, err = unix.InotifyAddWatch(inFd, "/dev/input", unix.IN_CREATE|unix.IN_DELETE)
		if err != nil {
			fmt.Println("inotify addwatch failed:", err)
			return
		}

		for {
			// poll devices + inotify
			var pollfds []unix.PollFd
			for _, dev := range devices {
				if dev.FD >= 0 {
					pollfds = append(pollfds, unix.PollFd{Fd: int32(dev.FD), Events: unix.POLLIN})
				}
			}
			pollfds = append(pollfds, unix.PollFd{Fd: int32(inFd), Events: unix.POLLIN})

			n, err := unix.Poll(pollfds, -1)
			if err != nil {
				fmt.Println("[DEBUG] poll error:", err)
				continue
			}
			if n == 0 {
				continue
			}

			for _, pfd := range pollfds {
				// Handle inotify events
				if pfd.Fd == int32(inFd) && pfd.Revents&unix.POLLIN != 0 {
					buf := make([]byte, 4096)
					n, _ := unix.Read(inFd, buf)
					offset := 0
					for offset < n {
						raw := (*unix.InotifyEvent)(unsafe.Pointer(&buf[offset]))
						nameBytes := buf[offset+unix.SizeofInotifyEvent : offset+unix.SizeofInotifyEvent+int(raw.Len)]
						name := string(nameBytes[:len(nameBytes)-1]) // trim null
						path := filepath.Join("/dev/input", name)

						if raw.Mask&unix.IN_CREATE != 0 {
							if filepath.Base(path) == "mice" || filepath.HasPrefix(filepath.Base(path), "mouse") {
								if dev, err := openMouseDevice(path); err == nil {
									devices[path] = dev
								}
							}
						}
						if raw.Mask&unix.IN_DELETE != 0 {
							if dev, ok := devices[path]; ok {
								dev.Close()
								delete(devices, path)
							}
						}
						offset += unix.SizeofInotifyEvent + int(raw.Len)
					}
				}

				// Handle mouse data
				if pfd.Fd != int32(inFd) && pfd.Revents&unix.POLLIN != 0 {
					buf := make([]byte, reportSize)
					n, err := unix.Read(int(pfd.Fd), buf)
					if err != nil || n < reportSize {
						continue
					}

					buttons := []string{}
					if buf[0]&0x1 != 0 {
						buttons = append(buttons, "L")
					}
					if buf[0]&0x2 != 0 {
						buttons = append(buttons, "R")
					}
					if buf[0]&0x4 != 0 {
						buttons = append(buttons, "M")
					}

					out <- MouseEvent{
						Timestamp: time.Now().UnixMilli(),
						Device:    fmt.Sprintf("fd=%d", pfd.Fd),
						Buttons:   buttons,
						DX:        int8(buf[1]),
						DY:        int8(buf[2]),
					}
				}
			}
		}
	}()

	return out
}
