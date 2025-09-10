package input

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	reportSize   = 3
	pollInterval = 10 * time.Millisecond
)

// MouseEvent is a decoded mouse packet
type MouseEvent struct {
	Timestamp int64
	Device    string
	Buttons   []string
	DX, DY    int8
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

			n, err := unix.Poll(pollfds, int(pollInterval.Milliseconds()))
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
