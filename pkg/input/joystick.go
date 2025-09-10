package input

import (
	"fmt"
	"path/filepath"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	jsEventSize          = 8
	JS_READ_FREQUENCY    = 50 * time.Millisecond
	HOTPLUG_SCAN_INTERVAL = 2 * time.Second
	DB_FILE_BASENAME     = "gamecontrollerdb.txt" // placeholder for SDL map
)

// JoystickDevice holds state for a joystick.
type JoystickDevice struct {
	Path    string
	Name    string
	GUID    string
	Buttons map[int]int16
	Axes    map[int]int16
	FD      int
}

func getJsMetadata(path string) (string, int, int, int) {
	base := filepath.Base(path)
	sysdir := filepath.Join("/sys/class/input", base, "device")

	readHex := func(fname string) (int, error) {
		data, err := os.ReadFile(filepath.Join(sysdir, fname))
		if err != nil {
			return 0, err
		}
		var val int
		fmt.Sscanf(string(data), "%x", &val)
		return val, nil
	}

	nameBytes, err := os.ReadFile(filepath.Join(sysdir, "name"))
	if err != nil {
		return base, 0, 0, 0
	}
	name := string(nameBytes)

	vid, _ := readHex("id/vendor")
	pid, _ := readHex("id/product")
	ver, _ := readHex("id/version")
	return name, vid, pid, ver
}

func makeGuid(vid, pid, version int) string {
	if vid == 0 || pid == 0 {
		return ""
	}
	le16 := func(x int) int { return ((x & 0xFF) << 8) | ((x >> 8) & 0xFF) }
	return fmt.Sprintf("03000000%04x0000%04x0000%04x0000", le16(vid), le16(pid), le16(version))
}

func newJoystickDevice(path string) *JoystickDevice {
	name, vid, pid, ver := getJsMetadata(path)
	guid := makeGuid(vid, pid, ver)

	return &JoystickDevice{
		Path:    path,
		Name:    name,
		GUID:    guid,
		Buttons: make(map[int]int16),
		Axes:    make(map[int]int16),
		FD:      -1,
	}
}

func (j *JoystickDevice) open(announce bool) bool {
	fd, err := unix.Open(j.Path, unix.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		j.FD = -1
		return false
	}
	j.FD = fd
	if announce {
		fmt.Printf("Opened %s (%s, GUID=%s)\n", j.Path, j.Name, j.GUID)
	}
	return true
}

func (j *JoystickDevice) close() {
	if j.FD >= 0 {
		_ = unix.Close(j.FD)
		j.FD = -1
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
		t := *(*uint32)(unsafe.Pointer(&buf[0]))
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
		_ = t
	}
	return changed
}

func (j *JoystickDevice) printState() {
	btns := []string{}
	for i, v := range j.Buttons {
		state := "R"
		if v != 0 {
			state = "P"
		}
		btns = append(btns, fmt.Sprintf("Btn%d=%s", i, state))
	}
	axs := []string{}
	for i, v := range j.Axes {
		axs = append(axs, fmt.Sprintf("Axis%d=%d", i, v))
	}
	fmt.Printf("Changed: Buttons[%v] Axes[%v]\n", btns, axs)
}

// RunJoystickMonitor runs the joystick hotplug + read loop.
func RunJoystickMonitor() {
	devices := map[string]*JoystickDevice{}
	lastScan := time.Now()

	for {
		now := time.Now()
		if now.Sub(lastScan) > HOTPLUG_SCAN_INTERVAL {
			lastScan = now
			paths, _ := filepath.Glob("/dev/input/js*")
			for _, path := range paths {
				if _, ok := devices[path]; !ok {
					dev := newJoystickDevice(path)
					if dev.open(true) {
						devices[path] = dev
					}
				}
			}
			// remove vanished
			for path, dev := range devices {
				if _, err := os.Stat(path); os.IsNotExist(err) {
					dev.close()
					delete(devices, path)
				}
			}
		}

		for _, dev := range devices {
			if dev.FD < 0 {
				if _, err := os.Stat(dev.Path); err == nil {
					dev.open(false)
				}
				continue
			}
			if dev.readEvents() {
				dev.printState()
			}
			// ✅ always reopen quietly (like Python)
			dev.close()
			dev.open(false)
		}

		time.Sleep(JS_READ_FREQUENCY)
	}
}
