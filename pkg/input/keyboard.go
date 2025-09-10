package input

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

const (
	kbEventSize         = 8
	kbReadFrequency     = 50 * time.Millisecond
	scanCodeFile        = "keyboardscancodes.txt"
	hotplugWatchPath    = "/dev/hidraw"
)

// SCAN_CODES loaded from file
var SCAN_CODES = map[int][]string{}

// Load scan codes (Python used exec, we just parse Go map literal syntax)
func loadScanCodes(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Expect something like: SCAN_CODES[30] = ["A"]
		if !strings.Contains(line, "=") {
			continue
		}
		parts := strings.Split(line, "=")
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		if strings.HasPrefix(key, "SCAN_CODES[") {
			numStr := strings.TrimSuffix(strings.TrimPrefix(key, "SCAN_CODES["), "]")
			num, _ := strconv.Atoi(numStr)
			val = strings.Trim(val, " []\"")
			items := strings.Split(val, ",")
			for i := range items {
				items[i] = strings.Trim(items[i], "\" ")
			}
			SCAN_CODES[num] = items
		}
	}
	return nil
}

// Parse keyboards from /proc/bus/input/devices
func parseKeyboards() map[string]string {
	keyboards := make(map[string]string)
	f, err := os.Open("/proc/bus/input/devices")
	if err != nil {
		return keyboards
	}
	defer f.Close()

	block := []string{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			// process block
			hasKbd := false
			for _, l := range block {
				if strings.Contains(l, "Handlers=") && strings.Contains(l, "kbd") {
					hasKbd = true
					break
				}
			}
			if hasKbd {
				var name, sysfs string
				for _, l := range block {
					if strings.HasPrefix(l, "N: ") {
						name = strings.SplitN(l, "=", 2)[1]
						name = strings.Trim(name, "\" ")
					}
					if strings.HasPrefix(l, "S: Sysfs=") {
						sysfs = strings.TrimSpace(strings.SplitN(l, "=", 2)[1])
					}
				}
				if sysfs != "" {
					parts := strings.Split(sysfs, "/")
					for i := len(parts) - 1; i >= 0; i-- {
						if strings.HasPrefix(parts[i], "0003:") {
							keyboards[parts[i]] = name
							break
						}
					}
				}
			}
			block = nil
		} else {
			block = append(block, line)
		}
	}
	return keyboards
}

// Match keyboards with /dev/hidraw*
func matchHidraws(keyboards map[string]string) [][2]string {
	matches := [][2]string{}
	paths, _ := filepath.Glob("/sys/class/hidraw/hidraw*/device")
	for _, p := range paths {
		target, _ := filepath.EvalSymlinks(p)
		sysfsID := filepath.Base(target)
		if name, ok := keyboards[sysfsID]; ok {
			devnode := "/dev/" + filepath.Base(filepath.Dir(p))
			matches = append(matches, [2]string{devnode, name})
		}
	}
	return matches
}

// Decode HID report
func decodeReport(report []byte) string {
	if len(report) != kbEventSize {
		return ""
	}
	if report[0] == 0x02 {
		return ""
	}
	if report[0] != 0 {
		allZero := true
		for _, b := range report[1:] {
			if b != 0 {
				allZero = false
				break
			}
		}
		if allZero {
			return ""
		}
	}
	output := ""
	for _, code := range report[2:8] {
		if code == 0 {
			continue
		}
		if names, ok := SCAN_CODES[int(code)]; ok && len(names) > 0 {
			key := names[0]
			switch key {
			case "SPACE":
				output += " "
			case "ENTER":
				output += "\n"
			default:
				if len(key) == 1 {
					output += key
				} else {
					output += "<" + key + ">"
				}
			}
		}
	}
	return output
}

type KeyboardDevice struct {
	Devnode string
	Name    string
	FD      int
}

func (k *KeyboardDevice) open() {
	fd, err := unix.Open(k.Devnode, unix.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		k.FD = -1
		return
	}
	k.FD = fd
	fmt.Printf("[+] Opened %s → %s\n", k.Devnode, k.Name)
}

func (k *KeyboardDevice) close() {
	if k.FD >= 0 {
		unix.Close(k.FD)
		fmt.Printf("[-] Closed %s → %s\n", k.Devnode, k.Name)
		k.FD = -1
	}
}

func (k *KeyboardDevice) readEvent() string {
	if k.FD < 0 {
		return ""
	}
	buf := make([]byte, kbEventSize)
	n, err := unix.Read(k.FD, buf)
	if err != nil || n < kbEventSize {
		return ""
	}
	return decodeReport(buf)
}

// -------- Streaming monitor with hotplug ----------

func StreamKeyboards() <-chan string {
	out := make(chan string, 100)

	go func() {
		defer close(out)
		devices := map[string]*KeyboardDevice{}

		rescan := func() {
			keyboards := parseKeyboards()
			matches := matchHidraws(keyboards)
			found := map[string]bool{}
			for _, pair := range matches {
				devnode, name := pair[0], pair[1]
				found[devnode] = true
				if _, ok := devices[devnode]; !ok {
					k := &KeyboardDevice{Devnode: devnode, Name: name, FD: -1}
					k.open()
					if k.FD >= 0 {
						devices[devnode] = k
					}
				}
			}
			for devnode, k := range devices {
				if !found[devnode] {
					k.close()
					delete(devices, devnode)
				}
			}
		}

		// Initial scan
		rescan()

		// Inotify on /dev/hidraw
		inFd, err := unix.InotifyInit()
		if err == nil {
			defer unix.Close(inFd)
			unix.InotifyAddWatch(inFd, "/dev/hidraw", unix.IN_CREATE|unix.IN_DELETE|unix.IN_MOVED_FROM|unix.IN_MOVED_TO)
			go func() {
				buf := make([]byte, 4096)
				for {
					_, _ = unix.Read(inFd, buf)
					rescan()
				}
			}()
		}

		for {
			for _, k := range devices {
				if k.FD < 0 {
					continue
				}
				if outStr := k.readEvent(); outStr != "" {
					out <- outStr
				}
			}
			time.Sleep(kbReadFrequency)
		}
	}()

	return out
}
