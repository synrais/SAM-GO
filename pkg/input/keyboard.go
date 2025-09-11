
package input

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/synrais/SAM-GO/pkg/assets"
	"golang.org/x/sys/unix"
)

// hotplugScanInterval defines how often to rescan for keyboards.
const hotplugScanInterval = 2 * time.Second

// scanCodes maps HID usage codes to printable strings.
var scanCodes map[byte]string

func init() {
	scanCodes = make(map[byte]string)
	// Parse the embedded keyboard scan codes file.
	re := regexp.MustCompile(`0x([0-9A-Fa-f]+):\s*{\"([^\"]+)\"`)
	matches := re.FindAllStringSubmatch(assets.KeyboardScanCodes, -1)
	for _, m := range matches {
		v, err := strconv.ParseInt(m[1], 16, 0)
		if err != nil {
			continue
		}
		scanCodes[byte(v)] = m[2]
	}
}

// decodeReport converts an 8 byte keyboard report into text.
func decodeReport(report []byte) string {
	if len(report) != 8 {
		return ""
	}
	if report[0] == 0x02 {
		return "" // ignore mouse/touchpad junk
	}
	allZero := true
	for _, b := range report[1:] {
		if b != 0 {
			allZero = false
			break
		}
	}
	if report[0] != 0 && allZero {
		return ""
	}

	var out strings.Builder
	for _, code := range report[2:] {
		if code == 0 {
			continue
		}
		if key, ok := scanCodes[code]; ok {
			switch key {
			case "SPACE":
				out.WriteByte(' ')
			case "ENTER":
				out.WriteByte('\n')
			default:
				if len(key) == 1 {
					out.WriteString(key)
				} else {
					out.WriteString("<" + key + ">")
				}
			}
		}
	}
	return out.String()
}

// KeyboardDevice represents a single hidraw keyboard device.
type KeyboardDevice struct {
	Path string
	Name string
	FD   int
}

func openKeyboardDevice(path, name string) (*KeyboardDevice, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	fmt.Printf("[+] Opened %s → %s\n", path, name)
	return &KeyboardDevice{Path: path, Name: name, FD: fd}, nil
}

func (k *KeyboardDevice) Close() {
	if k.FD >= 0 {
		_ = unix.Close(k.FD)
		fmt.Printf("[-] Closed %s → %s\n", k.Path, k.Name)
		k.FD = -1
	}
}

func parseKeyboards() map[string]string {
	keyboards := map[string]string{}
	f, err := os.Open("/proc/bus/input/devices")
	if err != nil {
		return keyboards
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	block := []string{}
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			handlers := false
			for _, l := range block {
				if strings.Contains(l, "Handlers=") && strings.Contains(l, "kbd") {
					handlers = true
					break
				}
			}
			if handlers {
				var nameLine, sysfsLine string
				for _, l := range block {
					if strings.HasPrefix(l, "N: ") {
						nameLine = l
					} else if strings.HasPrefix(l, "S: Sysfs=") {
						sysfsLine = l
					}
				}
				if nameLine != "" && sysfsLine != "" {
					name := strings.Trim(strings.SplitN(nameLine, "=", 2)[1], " \"")
					sysfsPath := strings.TrimSpace(strings.SplitN(sysfsLine, "=", 2)[1])
					parts := strings.Split(sysfsPath, "/")
					var sysfsID string
					for i := len(parts) - 1; i >= 0; i-- {
						if strings.HasPrefix(parts[i], "0003:") {
							sysfsID = parts[i]
							break
						}
					}
					if sysfsID != "" {
						keyboards[sysfsID] = name
					}
				}
			}
			block = block[:0]
		} else {
			block = append(block, line)
		}
	}
	return keyboards
}

func matchHidraws(kbs map[string]string) [][2]string {
	matches := [][2]string{}
	paths, _ := filepath.Glob("/sys/class/hidraw/hidraw*/device")
	for _, p := range paths {
		target, err := filepath.EvalSymlinks(p)
		if err != nil {
			continue
		}
		sysfsID := filepath.Base(target)
		if name, ok := kbs[sysfsID]; ok {
			devnode := "/dev/" + filepath.Base(filepath.Dir(p))
			matches = append(matches, [2]string{devnode, name})
		}
	}
	return matches
}

// StreamKeyboards streams decoded keyboard input as text lines.
func StreamKeyboards() <-chan string {
	out := make(chan string, 100)

	go func() {
		defer close(out)
		devices := map[string]*KeyboardDevice{}

		rescan := func() {
			kbs := parseKeyboards()
			matches := matchHidraws(kbs)
			found := map[string]bool{}
			for _, m := range matches {
				devnode, name := m[0], m[1]
				found[devnode] = true
				if _, ok := devices[devnode]; !ok {
					if dev, err := openKeyboardDevice(devnode, name); err == nil {
						devices[devnode] = dev
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

		rescan()
		inFd, err := unix.InotifyInit()
		if err != nil {
			fmt.Println("inotify init failed:", err)
			return
		}
		defer unix.Close(inFd)

		_, err = unix.InotifyAddWatch(inFd, "/dev", unix.IN_CREATE|unix.IN_DELETE)
		if err != nil {
			fmt.Println("inotify addwatch failed:", err)
			return
		}

		for {
			var pollfds []unix.PollFd
			fdmap := map[int]*KeyboardDevice{}
			for _, dev := range devices {
				if dev.FD >= 0 {
					pollfds = append(pollfds, unix.PollFd{Fd: int32(dev.FD), Events: unix.POLLIN})
					fdmap[dev.FD] = dev
				}
			}
			pollfds = append(pollfds, unix.PollFd{Fd: int32(inFd), Events: unix.POLLIN})

			n, err := unix.Poll(pollfds, -1)
			if err != nil || n == 0 {
				continue
			}

			for _, pfd := range pollfds {
				if pfd.Fd == int32(inFd) && pfd.Revents&unix.POLLIN != 0 {
					buf := make([]byte, 4096)
					n, _ := unix.Read(inFd, buf)
					offset := 0
					for offset < n {
						raw := (*unix.InotifyEvent)(unsafe.Pointer(&buf[offset]))
						nameBytes := buf[offset+unix.SizeofInotifyEvent : offset+unix.SizeofInotifyEvent+int(raw.Len)]
						name := string(nameBytes[:len(nameBytes)-1])
						if strings.HasPrefix(name, "hidraw") {
							rescan()
						}
						offset += unix.SizeofInotifyEvent + int(raw.Len)
					}
				} else if pfd.Fd != int32(inFd) && pfd.Revents&unix.POLLIN != 0 {
					buf := make([]byte, 8)
					if _, err := unix.Read(int(pfd.Fd), buf); err == nil {
						if s := decodeReport(buf); s != "" {
							dev := fdmap[int(pfd.Fd)]
							line := fmt.Sprintf("[%d ms] %s: %s", time.Now().UnixMilli(), dev.Path, s)
							out <- line
						}
					}
				}
			}
		}
	}()

	return out
}
