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

func decodePacket(buf []byte) string {
	if len(buf) < 3 {
		return ""
	}
	buttons := buf[0]
	dx := int8(buf[1])
	dy := int8(buf[2])

	pressed := []string{}
	if buttons&0x1 != 0 {
		pressed = append(pressed, "L")
	}
	if buttons&0x2 != 0 {
		pressed = append(pressed, "R")
	}
	if buttons&0x4 != 0 {
		pressed = append(pressed, "M")
	}

	status := "None"
	if len(pressed) > 0 {
		status = fmt.Sprint(pressed)
	}
	return fmt.Sprintf("buttons=%s dx=%d dy=%d", status, dx, dy)
}

func RunMouse() {
	devices := map[string]*MouseDevice{}

	// Initial scan
	paths, _ := filepath.Glob("/dev/input/mouse*")
	if _, err := os.Stat("/dev/input/mice"); err == nil {
		paths = append(paths, "/dev/input/mice")
	}
	for _, path := range paths {
		if dev, err := openMouseDevice(path); err == nil {
			devices[path] = dev
		}
	}

	// Setup inotify watcher on /dev/input
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

	// Event loop
	for {
		// poll devices for input
		var pollfds []unix.PollFd
		for _, dev := range devices {
			if dev.FD >= 0 {
				pollfds = append(pollfds, unix.PollFd{Fd: int32(dev.FD), Events: unix.POLLIN})
			}
		}
		// also watch inotify fd
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
			// handle device events
			if pfd.Fd == int32(inFd) && pfd.Revents&unix.POLLIN != 0 {
				// read inotify buffer
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

			// handle mouse data
			if pfd.Fd != int32(inFd) && pfd.Revents&unix.POLLIN != 0 {
				buf := make([]byte, reportSize)
				n, err := unix.Read(int(pfd.Fd), buf)
				if err != nil {
					fmt.Printf("[DEBUG] read error on fd=%d: %v\n", pfd.Fd, err)
					continue
				}
				if n == reportSize {
					ts := time.Now().UnixMilli()
					fmt.Printf("[%d ms] fd=%d: %s\n", ts, pfd.Fd, decodePacket(buf))
				}
			}
		}
	}
}
