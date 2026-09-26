package input

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/synrais/SAMenu/pkg/assets"
)

// -------------------------
// Key names
// -------------------------

// keyNames maps keyboard-page usages to SAMenu.ini names ("a", "enter",
// "pageup", "`"). Built from keyboardscancodes.txt; only the keyboard
// page (up to 0xA4) is used from it, since its higher entries are media
// keys from a different page that clash with codes like Left Alt.
var keyNames = map[uint16]string{
	0xE0: "leftctrl", 0xE1: "leftshift", 0xE2: "leftalt", 0xE3: "leftgui",
	0xE4: "rightctrl", 0xE5: "rightshift", 0xE6: "rightalt", 0xE7: "rightgui",
}

func init() {
	re := regexp.MustCompile(`0x([0-9A-Fa-f]+):\s*{\"([^\"]+)\"`)
	for _, m := range re.FindAllStringSubmatch(assets.KeyboardScanCodes, -1) {
		v, err := strconv.ParseUint(m[1], 16, 16)
		if err != nil || v > 0xA4 {
			continue
		}
		keyNames[uint16(v)] = cleanName(m[2])
	}
}

func keyName(usage uint16) string {
	if n, ok := keyNames[usage]; ok {
		return n
	}
	return fmt.Sprintf("key0x%02x", usage)
}

// -------------------------
// Decoding
// -------------------------

// pressedKeys returns the keyboard usages held down in one report. ok is
// false for a "too many keys" report, which says nothing about which
// keys are held.
func pressedKeys(l *hidLayout, id int, data []byte) (keys []uint16, ok bool) {
	for i := range l.Fields {
		f := &l.Fields[i]
		if f.ReportID != id || f.Page != pageKeyboard {
			continue
		}
		for n := 0; n < f.Count; n++ {
			v, good := fieldValue(data, f, n)
			if !good {
				break
			}
			if f.Variable {
				if v != 0 {
					if u, has := f.usageAt(n); has {
						keys = append(keys, u)
					}
				}
				continue
			}
			// Array: the value picks a usage.
			idx := int(v - f.LogMin)
			var u uint16
			switch {
			case len(f.Usages) > 0:
				if idx < 0 || idx >= len(f.Usages) {
					continue
				}
				u = f.Usages[idx]
			case f.usageRange:
				u = uint16(int(f.UsageMin) + idx)
			default:
				continue
			}
			if u == 0x01 { // ErrorRollOver: too many keys held
				return nil, false
			}
			if u > 0x03 { // 0 = none, 2-3 = other error codes
				keys = append(keys, u)
			}
		}
	}
	return keys, true
}

// mouseReport is what one mouse report says.
type mouseReport struct {
	Buttons    []int // 1 = left, 2 = right, 3 = middle, ...
	DX, DY     int
	Wheel      int
	HasPointer bool // the report had mouse data at all
}

func decodeMouse(l *hidLayout, id int, data []byte) mouseReport {
	var r mouseReport
	for i := range l.Fields {
		f := &l.Fields[i]
		if f.ReportID != id || f.AppUsage != pageGenericDesktop<<16|usageMouse {
			continue
		}
		for n := 0; n < f.Count; n++ {
			v, good := fieldValue(data, f, n)
			if !good {
				break
			}
			u, has := f.usageAt(n)
			if !f.Variable || !has {
				continue
			}
			switch {
			case f.Page == pageButton:
				r.HasPointer = true
				if v != 0 {
					r.Buttons = append(r.Buttons, int(u))
				}
			case f.Page == pageGenericDesktop && f.Relative:
				r.HasPointer = true
				switch u {
				case usageX:
					r.DX += int(v)
				case usageY:
					r.DY += int(v)
				case usageWheel:
					r.Wheel += int(v)
				}
			}
		}
	}
	return r
}

func mouseButtonName(b int) string {
	switch b {
	case 1:
		return "left"
	case 2:
		return "right"
	case 3:
		return "middle"
	}
	return fmt.Sprintf("button%d", b)
}

// -------------------------
// Per-device state
// -------------------------

const (
	swipeDistance = 60                     // mouse counts to count as a swipe
	swipeReset    = 300 * time.Millisecond // pause that starts a new swipe
)

type hidDevice struct {
	path   string // /dev/hidrawN
	name   string
	fd     int
	layout *hidLayout
	kbd    bool // decode keyboard data
	mouse  bool // decode mouse data

	keysDown map[int]map[uint16]bool // report ID -> held keys
	btnsDown map[int]bool
	accX     int
	accY     int
	lastMove time.Time
}

// handle decodes one report and sends an Event for each new press.
func (d *hidDevice) handle(report []byte, out chan<- Event, now time.Time) {
	id, data := splitReport(d.layout, report)

	if d.kbd {
		if keys, ok := pressedKeys(d.layout, id, data); ok {
			held := map[uint16]bool{}
			hasKeyField := false
			for i := range d.layout.Fields {
				f := &d.layout.Fields[i]
				if f.ReportID == id && f.Page == pageKeyboard {
					hasKeyField = true
					break
				}
			}
			if hasKeyField {
				prev := d.keysDown[id]
				for _, k := range keys {
					held[k] = true
					if !prev[k] {
						send(out, Event{Kind: "keyboard", Device: d.name, Name: keyName(k)})
					}
				}
				d.keysDown[id] = held
			}
		}
	}

	if d.mouse {
		r := decodeMouse(d.layout, id, data)
		if !r.HasPointer {
			return
		}
		held := map[int]bool{}
		for _, b := range r.Buttons {
			held[b] = true
			if !d.btnsDown[b] {
				send(out, Event{Kind: "mouse", Device: d.name, Name: mouseButtonName(b)})
			}
		}
		d.btnsDown = held

		if r.Wheel > 0 {
			send(out, Event{Kind: "mouse", Device: d.name, Name: "wheelup"})
		} else if r.Wheel < 0 {
			send(out, Event{Kind: "mouse", Device: d.name, Name: "wheeldown"})
		}

		if r.DX != 0 || r.DY != 0 {
			if now.Sub(d.lastMove) > swipeReset {
				d.accX, d.accY = 0, 0
			}
			d.lastMove = now
			d.accX += r.DX
			d.accY += r.DY
			name := ""
			switch { // HID mice: +X is right, +Y is down
			case d.accX <= -swipeDistance:
				name = "swipeleft"
			case d.accX >= swipeDistance:
				name = "swiperight"
			case d.accY <= -swipeDistance:
				name = "swipeup"
			case d.accY >= swipeDistance:
				name = "swipedown"
			}
			if name != "" {
				send(out, Event{Kind: "mouse", Device: d.name, Name: name})
				d.accX, d.accY = 0, 0
			}
		}
	}
}

// -------------------------
// Watching hidraw devices
// -------------------------

// hidName reads a hidraw device's name from sysfs.
func hidName(sysDir string) string {
	b, err := os.ReadFile(filepath.Join(sysDir, "device", "uevent"))
	if err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(line, "HID_NAME=") {
				return strings.TrimSpace(strings.TrimPrefix(line, "HID_NAME="))
			}
		}
	}
	return filepath.Base(sysDir)
}

// watchHID finds keyboards and mice among the hidraw devices, follows
// them being plugged in and out, and sends their presses to out.
func watchHID(out chan<- Event, wantKbd, wantMouse bool) {
	devices := map[string]*hidDevice{} // /dev path -> open device
	skipped := map[string]bool{}       // /dev path -> not a keyboard/mouse

	closeDev := func(d *hidDevice) {
		_ = unix.Close(d.fd)
		delete(devices, d.path)
		logf("- %s (%s)", d.name, filepath.Base(d.path))
	}

	rescan := func() {
		dirs, _ := filepath.Glob("/sys/class/hidraw/hidraw*")
		present := map[string]bool{}
		for _, dir := range dirs {
			path := "/dev/" + filepath.Base(dir)
			present[path] = true
			if devices[path] != nil || skipped[path] {
				continue
			}
			desc, err := os.ReadFile(filepath.Join(dir, "device", "report_descriptor"))
			if err != nil {
				continue // not ready yet; try again on the next rescan
			}
			layout := parseHIDDescriptor(desc)
			kbd := wantKbd && layout.isKeyboard
			mouse := wantMouse && layout.isMouse
			if !kbd && !mouse {
				skipped[path] = true
				continue
			}
			fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NONBLOCK, 0)
			if err != nil {
				continue // node not ready yet
			}
			d := &hidDevice{
				path: path, name: hidName(dir), fd: fd, layout: layout,
				kbd: kbd, mouse: mouse,
				keysDown: map[int]map[uint16]bool{}, btnsDown: map[int]bool{},
			}
			devices[path] = d
			kind := "keyboard"
			if kbd && mouse {
				kind = "keyboard + mouse"
			} else if mouse {
				kind = "mouse"
			}
			logf("+ %s: %s (%s)", kind, d.name, filepath.Base(path))
		}
		for path, d := range devices {
			if !present[path] {
				closeDev(d)
			}
		}
		for path := range skipped {
			if !present[path] {
				delete(skipped, path) // a new device may reuse the name
			}
		}
	}

	inFd, err := unix.InotifyInit1(unix.IN_NONBLOCK | unix.IN_CLOEXEC)
	if err == nil {
		defer unix.Close(inFd)
		if _, err := unix.InotifyAddWatch(inFd, "/dev", unix.IN_CREATE|unix.IN_DELETE); err != nil {
			logf("can't watch /dev for new devices: %v", err)
		}
	} else {
		logf("can't watch for new devices: %v", err)
		inFd = -1
	}

	rescan()
	lastScan := time.Now()
	buf := make([]byte, 512)
	evBuf := make([]byte, 4096)

	for {
		var pfds []unix.PollFd
		var order []*hidDevice
		for _, d := range devices {
			pfds = append(pfds, unix.PollFd{Fd: int32(d.fd), Events: unix.POLLIN})
			order = append(order, d)
		}
		if inFd >= 0 {
			pfds = append(pfds, unix.PollFd{Fd: int32(inFd), Events: unix.POLLIN})
		}

		// Wake at least every 2s, so a device whose node wasn't ready
		// on its first scan is picked up without another /dev event.
		n, err := unix.Poll(pfds, 2000)
		if err != nil && err != unix.EINTR {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		now := time.Now()
		needScan := now.Sub(lastScan) > 2*time.Second

		if n > 0 {
			for i, d := range order {
				re := pfds[i].Revents
				if re&(unix.POLLHUP|unix.POLLERR|unix.POLLNVAL) != 0 {
					closeDev(d)
					continue
				}
				if re&unix.POLLIN == 0 {
					continue
				}
				for { // drain every waiting report
					m, err := unix.Read(d.fd, buf)
					if err != nil {
						if err != unix.EAGAIN {
							closeDev(d)
						}
						break
					}
					if m <= 0 {
						break
					}
					d.handle(buf[:m], out, now)
				}
			}
			if inFd >= 0 && pfds[len(pfds)-1].Revents&unix.POLLIN != 0 {
				m, _ := unix.Read(inFd, evBuf)
				for off := 0; off+unix.SizeofInotifyEvent <= m; {
					raw := (*unix.InotifyEvent)(unsafe.Pointer(&evBuf[off]))
					nameEnd := off + unix.SizeofInotifyEvent + int(raw.Len)
					if nameEnd > m {
						break
					}
					name := strings.TrimRight(string(evBuf[off+unix.SizeofInotifyEvent:nameEnd]), "\x00")
					if strings.HasPrefix(name, "hidraw") {
						needScan = true
					}
					off = nameEnd
				}
			}
		}
		if needScan {
			rescan()
			lastScan = now
		}
	}
}
