package input

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
	"path/filepath"
	"golang.org/x/sys/unix"
)

const HOTPLUG_SCAN_INTERVAL = 2 * time.Second // seconds between rescans

var SCAN_CODES = map[int][]string{}

// --- Load SCAN_CODES from external file (keyboardscancodes.txt) ---
func loadScanCodes() error {
	here := "./" // Path to the current directory
	scanFile := filepath.Join(here, "keyboardscancodes.txt")

	// Check if the scan codes file exists
	if _, err := os.Stat(scanFile); os.IsNotExist(err) {
		return fmt.Errorf("Error: %s not found", scanFile)
	}

	// Open the scan codes file
	file, err := os.Open(scanFile)
	if err != nil {
		return fmt.Errorf("Error opening scan codes file: %v", err)
	}
	defer file.Close()

	// Parse the scan codes from the file (you may need to adjust this part based on your format)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "SCAN_CODES") {
			// Parse and store the scan codes (adjust based on actual file format)
			// SCAN_CODES should be populated here
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("Error reading scan codes file: %v", err)
	}

	return nil
}

// parseKeyboards parses the /proc/bus/input/devices file and returns a map of keyboards (sysfsID -> name)
func parseKeyboards() (map[string]string, error) {
	devices := make(map[string]string)
	file, err := os.Open("/proc/bus/input/devices")
	if err != nil {
		return nil, fmt.Errorf("Error opening /proc/bus/input/devices: %v", err)
	}
	defer file.Close()

	// Debug: Print the full content of /proc/bus/input/devices
	fmt.Println("Full /proc/bus/input/devices content:")
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Println(line) // Print each line to debug

		// Look for lines that contain 'Handlers=kbd'
		if strings.Contains(line, "Handlers=") && strings.Contains(line, "kbd") {
			// Log the handlers line
			fmt.Println("Found keyboard handler:", line)

			// Collect the block of lines for each device
			block := []string{line}
			for scanner.Scan() {
				line = scanner.Text()
				if line == "" {
					break
				}
				block = append(block, line)
			}

			// Extract device info from the block
			name, sysfsID := extractDeviceInfo(block)
			if sysfsID != "" {
				devices[sysfsID] = name
				// Log the detected device and sysfsID
				fmt.Printf("Detected keyboard: %s with sysfsID: %s\n", name, sysfsID)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("Error reading /proc/bus/input/devices: %v", err)
	}

	return devices, nil
}

// contains checks if a substring is present in a string
func contains(str, substr string) bool {
	return strings.Contains(str, substr)
}

// extractDeviceInfo extracts device name and sysfs ID from a block of lines in /proc/bus/input/devices
func extractDeviceInfo(block []string) (string, string) {
	var name, sysfsID string
	for _, line := range block {
		if strings.HasPrefix(line, "N: ") {
			name = strings.TrimSpace(strings.Split(line, "=")[1])
		}
		if strings.HasPrefix(line, "S: Sysfs=") {
			// Extract only the last part of the sysfs path (the sysfsID)
			sysfsPath := strings.TrimSpace(strings.Split(line, "=")[1])
			parts := strings.Split(sysfsPath, "/")
			sysfsID = parts[len(parts)-2] // Get the last instance (0003:258A:002A.0001)
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

	// Debug: Print all HIDraw devices found in /sys/class/hidraw/
	fmt.Println("HIDraw devices in /sys/class/hidraw/:")
	for _, hiddev := range files {
		fmt.Println("  HIDraw device:", hiddev) // Print each hidraw device path

		// Resolve the symlink to get the real sysfs path
		realpath, err := filepath.EvalSymlinks(hiddev)
		if err != nil {
			fmt.Println("Error resolving symlink:", err)
			continue
		}
		// The sysfs ID should be the last part of the path
		sysfsID := filepath.Base(realpath)

		// Debug: Show the sysfs ID and check the match with keyboards
		fmt.Printf("  Checking HIDraw sysfsID: %s\n", sysfsID)

		// Add debug output to see what keys and sysfsIDs are present
		for k, v := range keyboards {
			// Show what sysfsID we are attempting to match
			fmt.Printf("    Keyboard sysfsID: %s, Name: %s\n", k, v)
		}

		// Compare the extracted sysfsID from HIDraw and the keyboard sysfsID
		if name, found := keyboards[sysfsID]; found {
			// Match found: add it to the matched list
			devnode := fmt.Sprintf("/dev/%s", filepath.Base(filepath.Dir(realpath)))
			matches = append(matches, fmt.Sprintf("%s → %s", devnode, name))
			// Debug: Log the successful match
			fmt.Printf("  Match found! %s → %s\n", devnode, name)
		} else {
			// Debug: No match found for this sysfs ID
			fmt.Printf("  No match for sysfsID: %s\n", sysfsID)
		}
	}

	// Debug: Print matched HIDraw devices
	fmt.Println("Matched HIDraw devices:", matches)

	return matches, nil
}

// decodeReport decodes a keyboard report into human-readable characters
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

// allZero checks if all bytes in a slice are zero
func allZero(slice []byte) bool {
	for _, b := range slice {
		if b != 0 {
			return false
		}
	}
	return true
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
		return ""
	}
	return decodeReport(report)
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

			// Debug: Print matched devices
			fmt.Println("Matched devices:", matches)

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
