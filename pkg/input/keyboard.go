// pkg/keyboard.go
package input

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// interval between rescans for new/removed keyboards
const hotplugScanInterval = 2 * time.Second

// map of HID usage codes → key names, filled from keyboardscancodes.txt
var scanCodes map[byte]string

// load scan codes from keyboardscancodes.txt (same format as Python file)
func loadScanCodes() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	scanFile := filepath.Join(filepath.Dir(exe), "keyboardscancodes.txt")

	f, err := os.Open(scanFile)
	if err != nil {
		return fmt.Errorf("%s not found", scanFile)
	}
	defer f.Close()

	scanCodes = make(map[byte]string)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var code uint64
		var key string
		if _, err := fmt.Sscanf(line, "0x%x %s", &code, &key); err == nil {
			scanCodes[byte(code)] = strings.Trim(key, "\"")
		}
	}
	return sc.Err()
}

// parseKeyboards returns map[sysfsID]name from /proc/bus/input/devices
func parseKeyboards() map[string]string {
	f, err := os.Open("/proc/bus/input/devices")
	if err != nil {
		return nil
	}
	defer f.Close()

	kb := make(map[string]string)
	var block []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			handleDeviceBlock(block, kb)
			block = nil
			continue
		}
		block = append(block, line)
	}
	handleDeviceBlock(block, kb)
	return kb
}

func handleDeviceBlock(block []string, out map[string]string) {
	hasKbd := false
	for _, l := range block {
		if strings.Contains(l, "Handlers=") && strings.Contains(l, "kbd") {
			hasKbd = true
			break
		}
	}
	if !hasKbd {
		return
	}
	var name, sysfs string
	for _, l := range block {
		switch {
		case strings.HasPrefix(l, "N: "):
			name = strings.Trim(strings.SplitN(l, "=", 2)[1], "\"")
		case strings.HasPrefix(l, "S: Sysfs="):
			sysfs = strings.TrimSpace(strings.SplitN(l, "=", 2)[1])
		}
	}
	parts := strings.Split(sysfs, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if strings.HasPrefix(parts[i], "0003:") {
			out[parts[i]] = name
			break
		}
	}
}

type hidMatch struct {
	Devnode string
	Name    string
}

// matchHidraws links sysfs IDs to /dev/hidrawX devices
func matchHidraws(kb map[string]string) []hidMatch {
	var matches []hidMatch
	files, _ := filepath.Glob("/sys/class/hidraw/hidraw*/device")
	for _, p := range files {
		target, err := filepath.EvalSymlinks(p)
		if err != nil {
			continue
		}
		sysfsID := filepath.Base(target)
		if name, ok := kb[sysfsID]; ok {
			devnode := "/dev/" + filepath.Base(filepath.Dir(p))
			matches = append(matches, hidMatch{Devnode: devnode, Name: name})
		}
	}
	return matches
}

// decodeReport converts an 8‑byte HID report into text, similar to the Python version
func decodeReport(r []byte) string {
	if len(r) != 8 || r[0] == 0x02 {
		return ""
	}
	if r[0] != 0 {
		allZero := true
		for _, b := range r[1:] {
			if b != 0 {
				allZero = false
				break
			}
		}
		if allZero {
			return ""
		}
	}

	var out []string
	for _, code := range r[2:] {
		if code == 0 {
			continue
		}
		if key, ok := scanCodes[code]; ok {
			switch key {
			case "SPACE":
				out = append(out, " ")
			case "ENTER":
				out = append(out, "\n")
			default:
				if len(key) == 1 {
					out = append(out, key)
				} else {
					out = append(out, "<"+key+">")
				}
			}
		}
	}
	return strings.Join(out, "")
}

// KeyboardDevice wraps a hidraw device
type KeyboardDevice struct {
	Devnode string
	Name    string
	fd      int
}

func newKeyboardDevice(devnode, name string) (*KeyboardDevice, error) {
	fd, err := unix.Open(devnode, unix.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	return &KeyboardDevice{Devnode: devnode, Name: name, fd: fd}, nil
}

func (k *KeyboardDevice) Close() {
	if k.fd >= 0 {
		unix.Close(k.fd)
		k.fd = -1
	}
}

func (k *KeyboardDevice) ReadEvent() string {
	buf := make([]byte, 8)
	n, err := unix.Read(k.fd, buf)
	if err != nil || n != 8 {
		k.Close()
		return ""
	}
	return decodeReport(buf)
}

// MonitorKeyboards continuously monitors all attached keyboards and prints decoded keystrokes
func MonitorKeyboards() {
	if err := loadScanCodes(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}

	devices := map[string]*KeyboardDevice{}
	lastScan := time.Time{}

	for {
		if time.Since(lastScan) > hotplugScanInterval {
		lastScan = time.Now()
			kb := parseKeyboards()
			matches := matchHidraws(kb)
			found := make(map[string]struct{})

			for _, m := range matches {
				found[m.Devnode] = struct{}{}
				if _, ok := devices[m.Devnode]; !ok {
					if d, err := newKeyboardDevice(m.Devnode, m.Name); err == nil {
						fmt.Printf("[+] Opened %s → %s\n", m.Devnode, m.Name)
						devices[m.Devnode] = d
					}
				}
			}
			for devnode, d := range devices {
				if _, ok := found[devnode]; !ok {
					fmt.Printf("[-] Closed %s → %s\n", d.Devnode, d.Name)
					d.Close()
					delete(devices, devnode)
				}
			}
		}

		if len(devices) == 0 {
			time.Sleep(200 * time.Millisecond)
			continue
		}

		var fds []unix.PollFd
		for _, d := range devices {
			fds = append(fds, unix.PollFd{Fd: int32(d.fd), Events: unix.POLLIN})
		}
		unix.Poll(fds, 200)

		for _, p := range fds {
			if p.Revents&unix.POLLIN != 0 {
				for _, dev := range devices {
					if int32(dev.fd) == p.Fd {
						if out := dev.ReadEvent(); out != "" {
							fmt.Print(out)
						}
						break
					}
				}
			}
		}
	}
}
