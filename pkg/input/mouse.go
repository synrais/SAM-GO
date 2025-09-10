package input

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
)

const (
	reportSize   = 3
	hotplugScan  = 2 * time.Second
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

// RunMouse scans /dev/input for mouse devices and monitors them.
func RunMouse() {
	devices := map[string]*MouseDevice{}
	lastScan := time.Time{}

	for {
		now := time.Now()
		if now.Sub(lastScan) > hotplugScan {
			lastScan = now
			fmt.Println("[DEBUG] rescanning /dev/input/mouse* + /dev/input/mice")

			paths, _ := filepath.Glob("/dev/input/mouse*")
			if _, err := os.Stat("/dev/input/mice"); err == nil {
				paths = append(paths, "/dev/input/mice")
			}

			found := map[string]bool{}
			for _, path := range paths {
				found[path] = true
				if _, ok := devices[path]; !ok {
					if dev, err := openMouseDevice(path); err == nil {
						devices[path] = dev
					} else {
						fmt.Printf("[DEBUG] failed to open %s: %v\n", path, err)
					}
				}
			}

			for path, dev := range devices {
				if !found[path] {
					dev.Close()
					delete(devices, path)
				}
			}
		}

		if len(devices) == 0 {
			time.Sleep(pollInterval)
			continue
		}

		// build pollfds
		var pollfds []unix.PollFd
		for _, dev := range devices {
			if dev.FD >= 0 {
				pollfds = append(pollfds, unix.PollFd{Fd: int32(dev.FD), Events: unix.POLLIN})
			}
		}

		n, err := unix.Poll(pollfds, int(pollInterval.Milliseconds()))
		if err != nil {
			fmt.Println("[DEBUG] poll error:", err)
			continue
		}
		if n == 0 {
			continue // timeout, no events
		}

		for _, pfd := range pollfds {
			if pfd.Revents&unix.POLLIN != 0 {
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
