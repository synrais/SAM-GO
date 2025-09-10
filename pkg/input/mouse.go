package input

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
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
	File *os.File
}

func openMouseDevice(path string) (*MouseDevice, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	fmt.Printf("[+] Opened %s\n", path)
	return &MouseDevice{Path: path, File: f}, nil
}

func (m *MouseDevice) Close() {
	if m.File != nil {
		_ = m.File.Close()
		fmt.Printf("[-] Closed %s\n", m.Path)
		m.File = nil
	}
}

func (m *MouseDevice) ReadEvent() (string, error) {
	buf := make([]byte, reportSize)
	n, err := m.File.Read(buf)
	if err != nil {
		if err == io.EOF || err == unix.EAGAIN {
			return "", nil
		}
		return "", err
	}
	if n < reportSize {
		return "", nil
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
	return fmt.Sprintf("buttons=%s dx=%d dy=%d", status, dx, dy), nil
}

// RunMouse starts monitoring /dev/input/mouse* devices.
func RunMouse() {
	devices := make(map[string]*MouseDevice)
	var mu sync.Mutex

	// Setup inotify watcher
	fd, err := unix.InotifyInit()
	if err != nil {
		log.Fatalf("inotify init: %v", err)
	}
	defer unix.Close(fd)

	_, err = unix.InotifyAddWatch(fd, "/dev/input", unix.IN_CREATE|unix.IN_DELETE)
	if err != nil {
		log.Fatalf("add watch: %v", err)
	}

	// Open any existing devices at startup
	initial, _ := filepath.Glob("/dev/input/mouse*")
	if _, err := os.Stat("/dev/input/mice"); err == nil {
		initial = append(initial, "/dev/input/mice")
	}
	for _, path := range initial {
		if dev, err := openMouseDevice(path); err == nil {
			devices[path] = dev
		}
	}

	// Goroutine: inotify event loop
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := unix.Read(fd, buf)
			if err != nil {
				log.Fatalf("inotify read: %v", err)
			}
			offset := 0
			for offset < n {
				raw := (*unix.InotifyEvent)(unsafe.Pointer(&buf[offset]))
				nameBytes := buf[offset+unix.SizeofInotifyEvent : offset+unix.SizeofInotifyEvent+int(raw.Len)]
				name := string(nameBytes[:len(nameBytes)-1]) // trim null
				path := filepath.Join("/dev/input", name)

				if raw.Mask&unix.IN_CREATE != 0 {
					if filepath.Base(path) == "mice" || filepath.HasPrefix(filepath.Base(path), "mouse") {
						if dev, err := openMouseDevice(path); err == nil {
							mu.Lock()
							devices[path] = dev
							mu.Unlock()
						}
					}
				}
				if raw.Mask&unix.IN_DELETE != 0 {
					mu.Lock()
					if dev, ok := devices[path]; ok {
						dev.Close()
						delete(devices, path)
					}
					mu.Unlock()
				}
				offset += unix.SizeofInotifyEvent + int(raw.Len)
			}
		}
	}()

	// Main loop: poll devices
	for {
		mu.Lock()
		for path, dev := range devices {
			if dev.File == nil {
				continue
			}
			if out, err := dev.ReadEvent(); err == nil && out != "" {
				ts := time.Now().UnixMilli()
				fmt.Printf("[%d ms] %s: %s\n", ts, filepath.Base(path), out)
			}
		}
		mu.Unlock()
		time.Sleep(pollInterval)
	}
}
