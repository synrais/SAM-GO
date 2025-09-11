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
	"syscall"
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

	// Parse the scan codes from the file
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
			devnode := fmt.Sprintf("/dev/%s", filepath.Base(filepath.Dir(realpath)))
			matches = append(matches, fmt.Sprintf("%s → %s", devnode, name))
			// Debug: Show sysfsID matching result
			fmt.Printf("Match found! %s → %s\n", devnode, name)
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

func main() {
	// Initialize the scan codes
	if err := loadScanCodes(); err != nil {
		fmt.Println(err)
		return
	}

	// Start monitoring keyboards
	out := StreamKeyboards()
	for event := range out {
		fmt.Println(event)
	}
}
