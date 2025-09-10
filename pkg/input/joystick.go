package input

import (
	"bufio"
	"fmt"
	"strings"
	"time"
	"golang.org/x/sys/unix"
	"github.com/synrais/SAM-GO/pkg/assets"
	"os"
	"path/filepath"
	"strconv"
	"unsafe"
	"sort"
)

const (
	jsEventSize     = 8
	jsReadFrequency = 50 * time.Millisecond
)

// JoystickEvent is a snapshot of a joystick's state.
type JoystickEvent struct {
	Timestamp int64
	Device    string
	Buttons   map[string]string // friendly button name -> "P"/"R"
	Axes      map[string]int16  // friendly axis name -> value
}

// -------- SDL DB parsing ----------

func le16(x int) int {
	return ((x & 0xFF) << 8) | ((x >> 8) & 0xFF)
}

func makeGUID(vid, pid, version int) string {
	if vid == 0 || pid == 0 {
		return ""
	}
	return fmt.Sprintf("03000000%04x0000%04x0000%04x0000",
		le16(vid), le16(pid), le16(version))
}

type mappingEntry struct {
	guid     string
	name     string
	platform string
	mapping  map[string]string
}

func parseMappingLine(line string) *mappingEntry {
	parts := strings.Split(strings.TrimSpace(line), ",")
	if len(parts) < 3 {
		return nil
	}
	guid := strings.ToLower(parts[0])
	name := parts[1]
	items := parts[2:]
	mapping := map[string]string{}
	var platform string
	for _, item := range items {
		if !strings.Contains(item, ":") {
			continue
		}
		kv := strings.SplitN(item, ":", 2)
		k, v := kv[0], kv[1]
		if k == "platform" {
			platform = v
		} else {
			mapping[k] = v
		}
	}
	return &mappingEntry{guid: guid, name: name, platform: platform, mapping: mapping}
}

// Use embedded DB content instead of file
func loadSDLDB() []*mappingEntry {
	var entries []*mappingEntry
	// Access the embedded game controller DB content
	content := assets.GameControllerDB // This is the embedded content from assets

	// Read the content line by line
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		if e := parseMappingLine(scanner.Text()); e != nil {
			entries = append(entries, e)
		}
	}
	return entries
}

func chooseMapping(entries []*mappingEntry, guid string) map[string]string {
	for _, e := range entries {
		if e.guid == strings.ToLower(guid) && e.platform == "Linux" {
			fmt.Printf("  -> SDL DB: Matched to '%s'\n", e.name)
			return e.mapping
		}
	}
	fmt.Printf("  -> No SDL match for GUID: %s\n", guid)
	return map[string]string{}
}

func invertMapping(mapping map[string]string) (map[int]string, map[int]string) {
	btnmap := map[int]string{}
	axmap := map[int]string{}
	for friendly, raw := range mapping {
		if strings.HasPrefix(raw, "b") {
			if n, err := strconv.Atoi(raw[1:]); err == nil {
				btnmap[n] = friendly
			}
		} else if strings.HasPrefix(raw, "a") || strings.HasPrefix(raw, "+a") || strings.HasPrefix(raw, "-a") {
			nstr := strings.TrimLeft(raw, "+a-")
			if n, err := strconv.Atoi(nstr); err == nil {
				axmap[n] = friendly
			}
		}
	}
	return btnmap, axmap
}

// -------- Device handling ----------

type JoystickDevice struct {
	Path    string
	Name    string
	GUID    string
	FD      int
	Buttons map[int]int16
	Axes    map[int]int16
	btnmap  map[int]string
	axmap   map[int]string
}

func getJSMetadata(path string) (string, int, int, int) {
	base := filepath.Base(path)
	sysdir := filepath.Join("/sys/class/input", base, "device")
	readHex := func(fname string) int {
		b, err := os.ReadFile(filepath.Join(sysdir, fname))
		if err != nil {
			return 0
		}
		v, _ := strconv.ParseInt(strings.TrimSpace(string(b)), 16, 32)
		return int(v)
	}
	name := stringMust(os.ReadFile(filepath.Join(sysdir, "name")))

	vid := readHex("id/vendor")
	pid := readHex("id/product")
	ver := readHex("id/version")
	return name, vid, pid, ver
}

func stringMust(b []byte, _ error) string {
	if b == nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func openJoystickDevice(path string, sdlmap []*mappingEntry) (*JoystickDevice, error) {
	name, vid, pid, ver := getJSMetadata(path)
	guid := makeGUID(vid, pid, ver)
	mapping := map[string]string{}
	if guid != "" {
		mapping = chooseMapping(sdlmap, guid)
	}
	btnmap, axmap := invertMapping(mapping)

	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}

	fmt.Printf("[+] Opened %s (%s, GUID=%s)\n", path, name, guid)

	return &JoystickDevice{
		Path:    path,
		Name:    name,
		GUID:    guid,
		FD:      fd,
		Buttons: make(map[int]int16),
		Axes:    make(map[int]int16),
		btnmap:  btnmap,
		axmap:   axmap,
	}, nil
}

func (j *JoystickDevice) close() {
	if j.FD >= 0 {
		unix.Close(j.FD)
		j.FD = -1
	}
}

func (j *JoystickDevice) reopen() {
	j.close()
	fd, err := unix.Open(j.Path, unix.O_RDONLY|unix.O_NONBLOCK, 0)
	if err == nil {
		j.FD = fd
	}
}

func (j *JoystickDevice) readEvents() bool {
	if j.FD < 0 {
		return false
	}
	changed := false
	for {
		buf := make([]byte, jsEventSize)
		n, err := unix.Read(j.FD, buf)
		if err != nil || n < jsEventSize {
			break
		}
		val := *(*int16)(unsafe.Pointer(&buf[4]))
		etype := buf[6] & 0x7F
		num := int(buf[7])

		switch etype {
		case 0x01: // button
			if j.Buttons[num] != val {
				j.Buttons[num] = val
				changed = true
			}
		case 0x02: // axis
			if j.Axes[num] != val {
				j.Axes[num] = val
				changed = true
			}
		}
	}
	return changed
}

// -------- Streaming monitor with inotify hotplug ----------

func StreamJoysticks() <-chan string {
	out := make(chan string, 100)
	sdlmap := loadSDLDB() // Use the embedded SDL DB content

	go func() {
		defer close(out)
		devices := map[string]*JoystickDevice{}

		// Function to rescan all /dev/input/js* devices
		rescan := func() {
			paths, _ := filepath.Glob("/dev/input/js*")

			// Add new ones
			for _, path := range paths {
				if _, ok := devices[path]; !ok {
					if dev, err := openJoystickDevice(path, sdlmap); err == nil {
						devices[path] = dev
					}
				}
			}

			// Remove vanished ones
			for path, dev := range devices {
				if _, err := os.Stat(path); os.IsNotExist(err) {
					fmt.Printf("[-] Lost %s (%s)\n", dev.Path, dev.Name)
					dev.close()
					delete(devices, path)
				}
			}
		}

		// Initial scan
		rescan()

		// Inotify setup
		inFd, err := unix.InotifyInit()
		if err != nil {
			fmt.Println("inotify init failed:", err)
			return
		}
		defer unix.Close(inFd)

		// Watch by-id symlinks (reliable for all input devices)
		_, err = unix.InotifyAddWatch(inFd, "/dev/input/by-id",
			unix.IN_CREATE|unix.IN_DELETE|unix.IN_MOVED_FROM|unix.IN_MOVED_TO)
		if err != nil {
			fmt.Println("inotify addwatch failed:", err)
			return
		}

		// Hotplug watcher
		go func() {
			buf := make([]byte, 4096)
			for {
				_, _ = unix.Read(inFd, buf) // block until something happens
				// Any event here = device list changed, so rescan
				rescan()
			}
		}()

		// Main loop (read events + reopen)
		for {
			for _, dev := range devices {
				if dev.FD < 0 {
					if _, err := os.Stat(dev.Path); err == nil {
						dev.reopen()
					}
					continue
				}
				if dev.readEvents() {
					// ---- Build button list in order ----
					btnKeys := make([]int, 0, len(dev.btnmap))
					for k := range dev.btnmap {
						btnKeys = append(btnKeys, k)
					}
					sort.Ints(btnKeys)

					btnParts := []string{}
					for _, k := range btnKeys {
						name := dev.btnmap[k]
						if name == "" {
							name = fmt.Sprintf("Btn%d", k)
						}
						state := "R"
						if v, ok := dev.Buttons[k]; ok && v != 0 {
							state = "P"
						}
						btnParts = append(btnParts, fmt.Sprintf("%s=%s", name, state))
					}

					// ---- Build axis list in order ----
					axKeys := make([]int, 0, len(dev.axmap))
					for k := range dev.axmap {
						axKeys = append(axKeys, k)
					}
					sort.Ints(axKeys)

					axParts := []string{}
					for _, k := range axKeys {
						name := dev.axmap[k]
						if name == "" {
							name = fmt.Sprintf("Axis%d", k)
						}
						val := int16(0)
						if v, ok := dev.Axes[k]; ok {
							val = v
						}
						axParts = append(axParts, fmt.Sprintf("%s=%d", name, val))
					}

					// ---- Final line identical to Python ----
					line := fmt.Sprintf("[%d ms] %s: Buttons[%s] Axes[%s] ",
						time.Now().UnixMilli(),
						filepath.Base(dev.Path),
						strings.Join(btnParts, ", "),
						strings.Join(axParts, ", "),
					)

					out <- line
				}
				// Always reopen quietly
				dev.reopen()
			}

			time.Sleep(jsReadFrequency)
		}
	}()

	return out
}
