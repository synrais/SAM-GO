package input

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
	"path/filepath"
	"golang.org/x/sys/unix"
	"glob"
)

const HOTPLUG_SCAN_INTERVAL = 2 * time.Second // seconds between rescans

// --- Load SCAN_CODES from external file (keyboardscancodes.txt) ---
var SCAN_CODES = map[int][]string{}

// Load scan codes from the text file, following the Python script logic
func loadScanCodes() error {
	here := "./" // Replace with the correct path if needed
	scanFile := filepath.Join(here, "keyboardscancodes.txt")

	if _, err := os.Stat(scanFile); os.IsNotExist(err) {
		return fmt.Errorf("Error: %s not found", scanFile)
	}

	// Open the scan codes file
	file, err := os.Open(scanFile)
	if err != nil {
		return fmt.Errorf("Error opening scan codes file: %v", err)
	}
	defer file.Close()

	codeEnv := make(map[string]interface{})
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "SCAN_CODES") {
			// Parse the scan codes and store them
			// This part would need to adapt to the format of the scan codes in the file
			// You would need to manually extract the values into SCAN_CODES from the file
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("Error reading scan codes file: %v", err)
	}

	return nil
}

func parseKeyboards() (map[string]string, error) {
	devices := make(map[string]string)
	file, err := os.Open("/proc/bus/input/devices")
	if err != nil {
		return nil, fmt.Errorf("Error opening /proc/bus/input/devices: %v", err)
	}
	defer file.Close()

	var block []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if contains(line, "Handlers=") && contains(line, "kbd") {
				name, sysfsID := extractDeviceInfo(block)
				if sysfsID != "" {
					devices[sysfsID] = name
				}
			}
			block = nil
		} else {
			block = append(block, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("Error reading /proc/bus/input/devices: %v", err)
	}

	return devices, nil
}

func contains(str, substr string) bool {
	return strings.Contains(str, substr)
}

func extractDeviceInfo(block []string) (string, string) {
	var name, sysfsID string
	for _, line := range block {
		if strings.HasPrefix(line, "N: ") {
			name = strings.TrimSpace(strings.Split(line, "=")[1])
		}
		if strings.HasPrefix(line, "S: Sysfs=") {
			sysfsID = strings.TrimSpace(strings.Split(line, "=")[1])
		}
	}
	return name, sysfsID
}

func matchHidraws(keyboards map[string]string) ([]string, error) {
	matches := []string{}
	files, err := glob.Glob("/sys/class/hidraw/hidraw*/device")
	if err != nil {
		return nil, fmt.Errorf("Error in globbing hidraw devices: %v", err)
	}

	for _, hiddev := range files {
		sysfsID := filepath.Base(hiddev)
		if _, found := keyboards[sysfsID]; found {
			matches = append(matches, fmt.Sprintf("/dev/%s", filepath.Base(filepath.Dir(hiddev))))
		}
	}
	return matches, nil
}

func decodeReport(report []byte) string {
	if len(report) != 8 {
		return ""
	}

	// Skip the invalid reports
	if report[0] == 0x02 || (report[0] != 0 && allZero(report[1:])) {
		return ""
	}

	var output []string
	for _, code := range report[2:8] {
		if code == 0 {
			continue
		}
		if keys, ok := SCAN_CODES[int(code)]; ok {
			output = append(output, keys[0]) // Using lowercase key, can extend to shift/uppercase logic
		}
	}
	return strings.Join(output, "")
}

func allZero(slice []byte) bool {
	for _, b := range slice {
		if b != 0 {
			return false
		}
	}
	return true
}

type KeyboardDevice struct {
	devnode string
	name    string
	fd      int
}

func NewKeyboardDevice(devnode, name string) (*KeyboardDevice, error) {
	fd, err := unix.Open(devnode, unix.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	return &KeyboardDevice{
		devnode: devnode,
		name:    name,
		fd:      fd,
	}, nil
}

func (kd *KeyboardDevice) Close() {
	if kd.fd >= 0 {
		_ = unix.Close(kd.fd)
		kd.fd = -1
	}
}

func (kd *KeyboardDevice) ReadEvent() string {
	report := make([]byte, 8) // Read 8-byte report
	n, err := unix.Read(kd.fd, report)
	if err != nil || n < 8 {
		return ""
	}
	return decodeReport(report)
}

func monitorKeyboards(out chan<- string) {
	devices := make(map[string]*KeyboardDevice)
	lastScan := time.Now()

	for {
		now := time.Now()
		if now.Sub(lastScan) > HOTPLUG_SCAN_INTERVAL {
			lastScan = now
			// Rescan for keyboards
			keyboards, err := parseKeyboards()
			if err != nil {
				fmt.Println(err)
				continue
			}

			matches, err := matchHidraws(keyboards)
			if err != nil {
				fmt.Println(err)
				continue
			}

			// Add new devices
			for _, devnode := range matches {
				if _, found := devices[devnode]; !found {
					dev, err := NewKeyboardDevice(devnode, "Keyboard")
					if err == nil {
						devices[devnode] = dev
					}
				}
			}

			// Remove vanished devices
			for devnode := range devices {
				if !stringInSlice(devnode, matches) {
					devices[devnode].Close()
					delete(devices, devnode)
				}
			}
		}

		// Poll for keyboard events and decode them
		for _, dev := range devices {
			output := dev.ReadEvent()
			if output != "" {
				out <- output // Send decoded event to channel
			}
		}

		time.Sleep(200 * time.Millisecond) // Avoid busy loop
	}
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func StreamKeyboards() <-chan string {
	out := make(chan string, 100) // Buffered channel
	go monitorKeyboards(out)
	return out
}
