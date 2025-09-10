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

const (
	keyboardScanInterval = 2 * time.Second
	scanCodesFile        = "/media/fat/keyboardscancodes.txt"
)

var scanCodes = map[int][]string{}

func loadScanCodes(path string) {
	f, err := os.Open(path)
	if err != nil {
		fmt.Printf("Error: %s not found\n", path)
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		// ⚠️ Stub loader: you’ll want to implement parsing of your text file here.
		// For now this function just confirms the file exists.
	}
}

func parseKeyboards() map[string]string {
	keyboards := map[string]string{}
	block := []string{}
	f, err := os.Open("/proc/bus/input/devices")
	if err != nil {
		return keyboards
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
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
						parts := strings.SplitN(l, "=", 2)
						if len(parts) == 2 {
							name = strings.Trim(parts[1], "\" ")
						}
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
			block = []string{}
		} else {
			block = append(block, line)
		}
	}
	return keyboards
}

func matchHidraws(keyboards map[string]string) [][2]string {
	matches := [][2]string{}
	paths, _ := filepath.Glob("/sys/class/hidraw/hidraw*/device")
	for _, p := range paths {
		target, err := filepath.EvalSymlinks(p)
		if err != nil {
			continue
		}
		sysfsID := filepath.Base(target)
		if name, ok := keyboards[sysfsID]; ok {
			devnode := filepath.Join("/dev", filepath.Base(filepath.Dir(p)))
			matches = append(matches, [2]string{devnode, name})
		}
	}
	return matches
}

// -------- Report decoding ----------
func decodeReport(report []byte) string {
	if len(report) != 8 {
		return ""
	}
	if report[0] == 0x02 {
		return ""
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

	keycodes := report[2:8]
	out := ""
	for _, code := range keycodes {
		if code == 0 {
			continue
		}
		if names, ok := scanCodes[int(code)]; ok && len(names) > 0 {
			key := names[0]
			switch key {
			case "SPACE":
				out += " "
			case "ENTER":
				out += "\n"
			default:
				if len(key) == 1 {
					out += key
				} else {
					out += fmt.Sprintf("<%s>", key)
				}
			}
		}
	}
	return out
}

// -------- Device wrapper ----------
type KeyboardDevice struct {
	Devnode string
	Name    string
	fd      int
}

func newKeyboardDevice(devnode, name string) *KeyboardDevice {
	k := &KeyboardDevice{Devnode: devnode, Name: name, fd: -1}
	k.open()
	return k
}

func (k *KeyboardDevice) open() {
	if k.fd >= 0 {
		return
	}
	fd, err := unix.Open(k.Devnode, unix.O_RDONLY|unix.O_NONBLOCK, 0)
	if err == nil {
		k.fd = fd
		fmt.Printf("[+] Opened %s → %s\n", k.Devnode, k.Name)
	}
}

func (k *KeyboardDevice) close() {
	if k.fd >= 0 {
		unix.Close(k.fd)
		fmt.Printf("[-] Closed %s → %s\n", k.Devnode, k.Name)
		k.fd = -1
	}
}

func (k *KeyboardDevice) fileno() int {
	return k.fd
}

func (k *KeyboardDevice) readEvent() string {
	if k.fd < 0 {
		return ""
	}
	buf := make([]byte, 8)
	n, err := unix.Read(k.fd, buf)
	if err != nil || n < 8 {
		k.close()
		return ""
	}
	return decodeReport(buf)
}

// -------- Streaming monitor ----------
func StreamKeyboards() <-chan string {
	out := make(chan string, 100)
	go func() {
		defer close(out)
		devices := map[string]*KeyboardDevice{}
		lastScan := time.Now().Add(-keyboardScanInterval)

		for {
			now := time.Now()
			if now.Sub(lastScan) > keyboardScanInterval {
				lastScan = now
				keyboards := parseKeyboards()
				matches := matchHidraws(keyboards)
				found := map[string]bool{}
				for _, m := range matches {
					devnode, name := m[0], m[1]
					found[devnode] = true
					if _, ok := devices[devnode]; !ok {
						dev := newKeyboardDevice(devnode, name)
						if dev.fd >= 0 {
							devices[devnode] = dev
						}
					}
				}
				for devnode, dev := range devices {
					if !found[devnode] {
						dev.close()
						delete(devices, devnode)
					}
				}
			}

			if len(devices) > 0 {
				fds := []unix.PollFd{}
				for _, dev := range devices {
					if dev.fd >= 0 {
						fds = append(fds, unix.PollFd{Fd: int32(dev.fd), Events: unix.POLLIN})
					}
				}
				if len(fds) > 0 {
					_, _ = unix.Poll(fds, 200)
					for _, pfd := range fds {
						if pfd.Revents&unix.POLLIN != 0 {
							for _, dev := range devices {
								if int32(dev.fd) == pfd.Fd {
									if outStr := dev.readEvent(); outStr != "" {
										out <- outStr
									}
								}
							}
						}
					}
				}
			} else {
				time.Sleep(200 * time.Millisecond)
			}
		}
	}()
	return out
}
