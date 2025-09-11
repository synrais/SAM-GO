package input

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"golang.org/x/sys/unix"
)

// HOTPLUG_SCAN_INTERVAL determines the delay between rescans
const HOTPLUG_SCAN_INTERVAL = 2 * time.Second

// SCAN_CODES is the map for keyboard scan codes (Placeholder: Replace with actual scan codes)
var SCAN_CODES = map[int][]string{
	// Example placeholder values. Replace with your actual scan codes.
	0x04: {"a", "A"}, 0x05: {"b", "B"}, 0x06: {"c", "C"}, 0x07: {"d", "D"},
	0x08: {"e", "E"}, 0x09: {"f", "F"}, 0x0A: {"g", "G"}, 0x0B: {"h", "H"},
	// Add more codes here...
}

// parseKeyboards parses the /proc/bus/input/devices file and returns a map of keyboards (sysfsID -> name)
func parseKeyboards() (map[string]string, error) {
	devices := make(map[string]string)
	block := []string{}
	file, err := os.Open("/proc/bus/input/devices")
	if err != nil {
		return nil, fmt.Errorf("Error opening /proc/bus/input/devices: %v", err)
	}
	defer file.Close()

	// Read the file line-by-line
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		// When we encounter an empty line, we process the accumulated block
		if line == "" {
			if anyKeyboardHandlerInBlock(block) {
				name, sysfsID := extractDeviceInfo(block)
				if sysfsID != "" {
					devices[sysfsID] = name
					// Debug: Show sysfsID extraction result
					fmt.Printf("Extracted keys from cat /proc/bus/input/devices: sysfsID: %s → Name: %s\n", sysfsID, name)
				}
			}
			block = []string{} // Reset for the next device
		} else {
			block = append(block, line)
		}
	}

	// Handle any remaining device block
	if len(block) > 0 && anyKeyboardHandlerInBlock(block) {
		name, sysfsID := extractDeviceInfo(block)
		if sysfsID != "" {
			devices[sysfsID] = name
			// Debug: Show sysfsID extraction result
			fmt.Printf("Extracted keys from cat /proc/bus/input/devices: sysfsID: %s → Name: %s\n", sysfsID, name)
		}
	}

	return devices, nil
}

// Check if any block contains a keyboard handler
func anyKeyboardHandlerInBlock(block []string) bool {
	for _, line := range block {
		if strings.Contains(line, "Handlers=") && strings.Contains(line, "kbd") {
			return true
		}
	}
	return false
}

// extractDeviceInfo extracts device name and sysfs ID from a block of lines in /proc/bus/input/devices
func extractDeviceInfo(block []string) (string, string) {
	var name, sysfsID string
	sysfsPattern := regexp.MustCompile(`\b[0-9a-fA-F]+:[0-9a-fA-F]+:[0-9a-fA-F]+(?:\.[0-9]+)?\b`)

	for _, line := range block {
		if strings.HasPrefix(line, "N: ") {
			name = strings.TrimSpace(strings.Split(line, "=")[1])
		}
		if strings.HasPrefix(line, "S: Sysfs=") {
			sysfsPath := strings.TrimSpace(strings.Split(line, "=")[1])
			// Match the sysfsID using regex (find the pattern)
			match := sysfsPattern.FindString(sysfsPath)
			if match != "" {
				sysfsID = match
			}
		}
	}
	return name, sysfsID
}

// matchHidraws matches the sysfs IDs from keyboards to HIDraw device paths
func matchHidraws(keyboards map[string]string) ([]string, error) {
	matches := []string{}
	files, err := filepath.Glob("/sys/class/hidraw/hidraw*/device")
	if err != nil {
		return nil, fmt.Errorf("Error in globbing hidraw devices: %v", err)
	}

	for _, hiddev := range files {
		realpath, err := os.Readlink(hiddev)
		if err != nil {
			fmt.Println("Error resolving symlink:", err)
			continue
		}
		sysfsID := filepath.Base(realpath)

		// If the sysfsID is found in the keyboards map, add the match
		if name, found := keyboards[sysfsID]; found {
			// Construct devnode correctly: /dev/hidrawX
			devnode := fmt.Sprintf("/dev/%s", filepath.Base(hiddev)) // This should give you /dev/hidrawX

			// Add to matches list
			matches = append(matches, fmt.Sprintf("%s → %s", devnode, name))

			// Debugging: Show sysfsID matching result with the full path
			fmt.Printf("Match found! HID device: %s → SysfsID: %s → \"%s\"\n", devnode, sysfsID, name)
		}
	}

	return matches, nil
}

// KeyboardDevice represents a keyboard device, managing its file descriptor and events
type KeyboardDevice struct {
	devnode string
	name    string
	fd      int
}

// NewKeyboardDevice opens a keyboard device for reading
func NewKeyboardDevice(devnode, name string) (*KeyboardDevice, error) {
	fd, err := unix.Open(devnode, unix.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	fmt.Printf("[+] Opened device: %s → %s\n", devnode, name) // Debugging device opening
	return &KeyboardDevice{
		devnode: devnode,
		name:    name,
		fd:      fd,
	}, nil
}

// Close closes the keyboard device
func (kd *KeyboardDevice) Close() {
	if kd.fd >= 0 {
		_ = unix.Close(kd.fd)
		kd.fd = -1
	}
}

// ReadEvent reads a keyboard event from the device and decodes it
func (kd *KeyboardDevice) ReadEvent() string {
	report := make([]byte, 8) // Read 8-byte report
	n, err := unix.Read(kd.fd, report)
	if err != nil || n < 8 {
		fmt.Printf("Failed to read event from %s → %v\n", kd.devnode, err) // Debugging failed read
		return ""
	}
	// Debugging: Show raw data being read
	fmt.Printf("Raw event data: %v\n", report)
	return decodeReport(report)
}

// decodeReport decodes a keyboard event into human-readable output
func decodeReport(report []byte) string {
	if len(report) != 8 {
		fmt.Println("Invalid report length") // Debugging invalid report length
		return ""
	}

	if report[0] == 0x02 {
		return ""
	}
	if report[0] != 0 && allZero(report[1:]) {
		return ""
	}

	keycodes := report[2:8]
	fmt.Printf("Decoded report: %v\n", keycodes) // Debugging the decoded keycodes
	output := []string{}
	for _, code := range keycodes {
		if code == 0 {
			continue
		}
		if keys, ok := SCAN_CODES[int(code)]; ok {
			fmt.Printf("Key found: %s\n", keys[0]) // Debugging individual key matches
			output = append(output, keys[0])
		}
	}
	if len(output) > 0 {
		return strings.Join(output, "")
	}
	return ""
}

// allZero checks if all bytes in a slice are zero
func allZero(slice []byte) bool {
	for _, b := range slice {
		if b != 0 {
			return false
		}
	}
	return true
}

// monitorKeyboards monitors and processes keyboard events, matching devices and parsing reports
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
				fmt.Printf("Key output: %s\n", output) // Debugging output
				out <- output // Send decoded event to channel
			}
		}

		time.Sleep(200 * time.Millisecond) // Avoid busy loop
	}
}

// stringInSlice checks if a string is in the list
func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// StreamKeyboards starts the keyboard monitoring and returns a channel of events
func StreamKeyboards() <-chan string {
	out := make(chan string, 100) // Buffered channel
	go monitorKeyboards(out)
	return out
}

// main function to run the monitor
func main() {
	out := StreamKeyboards()
	for event := range out {
		fmt.Println("Received event:", event) // Print the received event
	}
}
