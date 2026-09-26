package input

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/synrais/SAMenu/pkg/assets"
	"github.com/synrais/SAMenu/pkg/input/virtualinput"
)

// Controllers
//
// MiSTer holds controllers exclusively while a core runs, so a joystick
// device that stays open never receives events. But each time one is
// opened, the kernel reports its *live* state (every button and axis), so
// the detector reopens each controller every poll and compares that
// state with the last one.

const (
	jsPollEvery   = 25 * time.Millisecond
	jsRescanEvery = 2 * time.Second
	axisThreshold = 20000 // of 32767: how far an axis must move to count

	absHat0X = 0x10 // first d-pad ("hat") axis code; hats run to 0x17
	absHat3Y = 0x17
)

// -------- SDL DB parsing ----------

func le16(x int) int {
	return ((x & 0xFF) << 8) | ((x >> 8) & 0xFF)
}

func makeGUID(bus, vid, pid, version int) string {
	if vid == 0 || pid == 0 {
		return ""
	}
	return fmt.Sprintf("%02x000000%04x0000%04x0000%04x0000",
		bus&0xFF, le16(vid), le16(pid), le16(version))
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
	content := assets.GameControllerDB

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		if e := parseMappingLine(scanner.Text()); e != nil {
			entries = append(entries, e)
		}
	}
	return entries
}

func chooseMapping(entries []*mappingEntry, bus, vid, pid, ver int) (map[string]string, string) {
	attempts := []struct {
		guid string
		msg  string
	}{
		{makeGUID(bus, vid, pid, ver), "Exact match"},
		{makeGUID(0, vid, pid, ver), "Close match (ignore bustype)"},
		{makeGUID(0, vid, pid, 0), "Close match (ignore bustype & version)"},
	}

	for _, a := range attempts {
		if a.guid == "" {
			continue
		}
		g := strings.ToLower(a.guid)
		for _, e := range entries {
			if strings.ToLower(e.guid) == g && e.platform == "Linux" {
				return e.mapping, e.name
			}
		}
	}
	return nil, ""
}

// -------- Device layout ----------

// Joystick ioctls (see linux/joystick.h).
const (
	jsIOCGAXES    = 0x80016a11
	jsIOCGBUTTONS = 0x80016a12
	jsIOCGAXMAP   = 0x80406a32 // 64 bytes: the ABS code of each axis
)

func ioctlBytes(fd int, req uintptr, buf []byte) error {
	_, _, e := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), req, uintptr(unsafe.Pointer(&buf[0])))
	if e != 0 {
		return e
	}
	return nil
}

// jsLayout names a controller's buttons and axis directions.
type jsLayout struct {
	btnNames []string    // per button
	axNames  [][2]string // per axis: [0] negative side, [1] positive side
}

func isHat(code byte) bool { return code >= absHat0X && code <= absHat3Y }

// hatName gives d-pad names to an unmapped hat axis: hat 0 is "dpup" etc,
// further hats "hat1up" etc.
func hatName(code byte, positive bool) string {
	n := int(code-absHat0X) / 2
	isY := (code-absHat0X)%2 == 1
	dir := map[[2]bool]string{
		{false, false}: "left", {false, true}: "right",
		{true, false}: "up", {true, true}: "down",
	}[[2]bool{isY, positive}]
	if n == 0 {
		return "dp" + dir
	}
	return fmt.Sprintf("hat%d%s", n, dir)
}

// buildLayout names buttons and axes from an SDL mapping (which may be
// nil), with raw names for anything the mapping doesn't cover.
//
// SDL numbers buttons the same way the joystick device does. It numbers
// axes the same way too, except that it leaves the hats (d-pads) out and
// refers to them as h0.1 etc, so its axis numbers are translated here.
func buildLayout(nButtons int, axisCodes []byte, mapping map[string]string) jsLayout {
	l := jsLayout{
		btnNames: make([]string, nButtons),
		axNames:  make([][2]string, len(axisCodes)),
	}
	var nonHat []int // SDL axis number -> joystick axis number
	for i, c := range axisCodes {
		if !isHat(c) {
			nonHat = append(nonHat, i)
		}
	}
	findAxis := func(code byte) int {
		for i, c := range axisCodes {
			if c == code {
				return i
			}
		}
		return -1
	}

	// Sorted, so the result doesn't depend on map order.
	friendly := make([]string, 0, len(mapping))
	for k := range mapping {
		friendly = append(friendly, k)
	}
	sort.Strings(friendly)

	for _, name := range friendly {
		raw := mapping[name]
		invert := strings.HasSuffix(raw, "~")
		raw = strings.TrimSuffix(raw, "~")
		half := 0 // -1 / +1 for "-a0" / "+a0"
		if strings.HasPrefix(raw, "+") {
			half, raw = 1, raw[1:]
		} else if strings.HasPrefix(raw, "-") {
			half, raw = -1, raw[1:]
		}
		// An output like "+leftx" maps only half of an axis.
		out := strings.TrimLeft(name, "+-")

		switch {
		case strings.HasPrefix(raw, "b"):
			if n, err := strconv.Atoi(raw[1:]); err == nil && n >= 0 && n < nButtons {
				l.btnNames[n] = out
			}
		case strings.HasPrefix(raw, "a"):
			k, err := strconv.Atoi(raw[1:])
			if err != nil || k < 0 || k >= len(nonHat) {
				continue
			}
			ax := nonHat[k]
			neg, pos := 0, 1
			if invert {
				neg, pos = 1, 0
			}
			switch half {
			case 0:
				l.axNames[ax][neg] = out + "-"
				l.axNames[ax][pos] = out + "+"
			case -1:
				l.axNames[ax][neg] = out
			case 1:
				l.axNames[ax][pos] = out
			}
		case strings.HasPrefix(raw, "h"):
			parts := strings.SplitN(raw[1:], ".", 2)
			if len(parts) != 2 {
				continue
			}
			hat, err1 := strconv.Atoi(parts[0])
			mask, err2 := strconv.Atoi(parts[1])
			if err1 != nil || err2 != nil || hat < 0 || hat > 3 {
				continue
			}
			x := findAxis(byte(absHat0X + 2*hat))
			y := findAxis(byte(absHat0X + 2*hat + 1))
			switch {
			case mask&1 != 0 && y >= 0: // up
				l.axNames[y][0] = out
			case mask&2 != 0 && x >= 0: // right
				l.axNames[x][1] = out
			case mask&4 != 0 && y >= 0: // down
				l.axNames[y][1] = out
			case mask&8 != 0 && x >= 0: // left
				l.axNames[x][0] = out
			}
		}
	}

	// Raw names for whatever is left.
	for i := range l.btnNames {
		if l.btnNames[i] == "" {
			l.btnNames[i] = fmt.Sprintf("btn%d", i)
		}
	}
	for i, c := range axisCodes {
		for side := 0; side < 2; side++ {
			if l.axNames[i][side] != "" {
				continue
			}
			if isHat(c) {
				l.axNames[i][side] = hatName(c, side == 1)
			} else if side == 0 {
				l.axNames[i][side] = fmt.Sprintf("axis%d-", i)
			} else {
				l.axNames[i][side] = fmt.Sprintf("axis%d+", i)
			}
		}
	}
	return l
}

// -------- Devices ----------

type jsDevice struct {
	path   string
	name   string
	layout jsLayout

	btn   []bool // last state
	axis  []int8 // last side of each axis: -1, 0, +1
	rest  []int8 // side an axis rests on (triggers often rest at -32767), never reported
	codes []byte // ABS code of each axis
	have  bool   // a first state has been read
}

// jsInfo reads a controller's name and USB/Bluetooth IDs from sysfs.
func jsInfo(path string) (name string, bus, vid, pid, ver int) {
	sysdir := filepath.Join("/sys/class/input", filepath.Base(path), "device")
	readHex := func(f string) int {
		b, err := os.ReadFile(filepath.Join(sysdir, "id", f))
		if err != nil {
			return 0
		}
		v, _ := strconv.ParseInt(strings.TrimSpace(string(b)), 16, 32)
		return int(v)
	}
	if b, err := os.ReadFile(filepath.Join(sysdir, "name")); err == nil {
		name = strings.TrimSpace(string(b))
	}
	return name, readHex("bustype"), readHex("vendor"), readHex("product"), readHex("version")
}

func openJoystick(path string, sdl []*mappingEntry) (*jsDevice, error) {
	name, bus, vid, pid, ver := jsInfo(path)
	if name == virtualinput.DeviceName {
		return nil, errSelf
	}
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer unix.Close(fd)

	var nAx, nBtn [1]byte
	axMap := make([]byte, 64)
	if err := ioctlBytes(fd, jsIOCGAXES, nAx[:]); err != nil {
		return nil, err
	}
	if err := ioctlBytes(fd, jsIOCGBUTTONS, nBtn[:]); err != nil {
		return nil, err
	}
	if err := ioctlBytes(fd, jsIOCGAXMAP, axMap); err != nil {
		return nil, err
	}
	axes := int(nAx[0])
	if axes > len(axMap) {
		axes = len(axMap)
	}

	var mapping map[string]string
	mapName := ""
	if makeGUID(bus, vid, pid, ver) != "" {
		mapping, mapName = chooseMapping(sdl, bus, vid, pid, ver)
	}
	d := &jsDevice{
		path:   path,
		name:   name,
		layout: buildLayout(int(nBtn[0]), axMap[:axes], mapping),
		btn:    make([]bool, nBtn[0]),
		axis:   make([]int8, axes),
		rest:   make([]int8, axes),
		codes:  append([]byte(nil), axMap[:axes]...),
	}
	if mapName != "" {
		logf("+ joystick: %s (%s), %d buttons, %d axes, named from SDL entry %q",
			name, filepath.Base(path), nBtn[0], axes, mapName)
	} else {
		logf("+ joystick: %s (%s), %d buttons, %d axes, no SDL entry (raw names like btn0, axis0+)",
			name, filepath.Base(path), nBtn[0], axes)
	}
	return d, nil
}

var errSelf = fmt.Errorf("SAMenu's own virtual device")

// poll reads the controller's live state and sends an Event for each
// button newly pressed and each axis newly pushed to one side.
func (d *jsDevice) poll(out chan<- Event, buf []byte) error {
	fd, err := unix.Open(d.path, unix.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		return err
	}
	btn := make([]bool, len(d.btn))
	copy(btn, d.btn)
	axis := make([]int8, len(d.axis))
	copy(axis, d.axis)

	for {
		n, err := unix.Read(fd, buf)
		if err != nil || n < 8 {
			break
		}
		for off := 0; off+8 <= n; off += 8 {
			val := int16(uint16(buf[off+4]) | uint16(buf[off+5])<<8)
			typ := buf[off+6] &^ 0x80 // drop the "initial state" flag
			num := int(buf[off+7])
			switch {
			case typ == 1 && num < len(btn):
				btn[num] = val != 0
			case typ == 2 && num < len(axis):
				switch {
				case val <= -axisThreshold:
					axis[num] = -1
				case val >= axisThreshold:
					axis[num] = 1
				default:
					axis[num] = 0
				}
			}
		}
	}
	_ = unix.Close(fd)

	if d.have {
		for i, down := range btn {
			if down && !d.btn[i] {
				send(out, Event{Kind: "joystick", Device: d.name, Name: d.layout.btnNames[i]})
			}
		}
		for i, side := range axis {
			if side != 0 && side != d.axis[i] && side != d.rest[i] {
				name := d.layout.axNames[i][0]
				if side > 0 {
					name = d.layout.axNames[i][1]
				}
				code := byte(0xFF)
				if i < len(d.codes) {
					code = d.codes[i]
				}
				send(out, Event{Kind: "joystick", Device: d.name, Name: name, Hint: axisHint(name, code, side > 0)})
			}
		}
	}
	if !d.have {
		// An axis already at one end on the first read is resting there
		// (a trigger), so that end isn't a press. D-pads never rest at an
		// end, so a d-pad held while a pad is plugged in still counts.
		for i, side := range axis {
			if i < len(d.rest) && (i >= len(d.codes) || !isHat(d.codes[i])) {
				d.rest[i] = side
			}
		}
	}
	d.btn, d.axis, d.have = btn, axis, true
	return nil
}

// watchJoysticks polls every controller and follows them being plugged
// in and out.
func watchJoysticks(out chan<- Event) {
	sdl := loadSDLDB()
	devices := map[string]*jsDevice{}
	ignored := map[string]bool{} // SAMenu's own virtual pad

	rescan := func() {
		paths, _ := filepath.Glob("/dev/input/js*")
		present := map[string]bool{}
		for _, p := range paths {
			present[p] = true
			if devices[p] != nil || ignored[p] {
				continue
			}
			d, err := openJoystick(p, sdl)
			if err == errSelf {
				ignored[p] = true
				continue
			}
			if err == nil {
				devices[p] = d
			}
		}
		for p, d := range devices {
			if !present[p] {
				logf("- %s (%s)", d.name, filepath.Base(p))
				delete(devices, p)
			}
		}
		for p := range ignored {
			if !present[p] {
				delete(ignored, p)
			}
		}
	}

	// Plug-in events arrive on their own goroutine, which only raises a
	// flag: the device list itself is only ever touched by this loop.
	hotplug := make(chan struct{}, 1)
	if inFd, err := unix.InotifyInit1(unix.IN_CLOEXEC); err == nil {
		if _, err := unix.InotifyAddWatch(inFd, "/dev/input", unix.IN_CREATE|unix.IN_DELETE); err == nil {
			go func() {
				buf := make([]byte, 4096)
				for {
					if n, err := unix.Read(inFd, buf); err != nil || n <= 0 {
						time.Sleep(time.Second)
						continue
					}
					select {
					case hotplug <- struct{}{}:
					default:
					}
				}
			}()
		}
	}

	rescan()
	lastScan := time.Now()
	buf := make([]byte, 8*(256+64))
	tick := time.NewTicker(jsPollEvery)
	defer tick.Stop()

	for range tick.C {
		scan := time.Since(lastScan) > jsRescanEvery
		select {
		case <-hotplug:
			scan = true
		default:
		}
		if scan {
			rescan()
			lastScan = time.Now()
		}
		for p, d := range devices {
			if err := d.poll(out, buf); err != nil {
				logf("- %s (%s)", d.name, filepath.Base(p))
				delete(devices, p)
			}
		}
	}
}

// axisHint describes an axis name in plain words: "lefty-" is "left stick
// up". Raw names on unknown pads ("axis1+") are described from the axis
// type where that's reliable (X/Y and RX/RY); negative Y is up.
func axisHint(name string, code byte, positive bool) string {
	named := map[string]string{
		"leftx-": "left stick left", "leftx+": "left stick right",
		"lefty-": "left stick up", "lefty+": "left stick down",
		"rightx-": "right stick left", "rightx+": "right stick right",
		"righty-": "right stick up", "righty+": "right stick down",
		"lefttrigger+": "left trigger", "righttrigger+": "right trigger",
	}
	if h, ok := named[name]; ok {
		return h
	}
	if !strings.HasPrefix(name, "axis") {
		return ""
	}
	dirs := map[byte][2]string{
		0x00: {"left", "right"},                         // X
		0x01: {"up", "down"},                            // Y
		0x03: {"right stick left", "right stick right"}, // RX
		0x04: {"right stick up", "right stick down"},    // RY
	}
	if d, ok := dirs[code]; ok {
		if positive {
			return d[1]
		}
		return d[0]
	}
	return ""
}
