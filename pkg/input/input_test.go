package input

import (
	"encoding/binary"
	"encoding/hex"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func desc(t *testing.T, s string) []byte {
	b, err := hex.DecodeString(strings.ReplaceAll(s, " ", ""))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Real-world descriptors.
const (
	bootKeyboard = "05 01 09 06 A1 01 05 07 19 E0 29 E7 15 00 25 01 75 01 95 08 81 02 95 01 75 08 81 01 95 05 75 01 05 08 19 01 29 05 91 02 95 01 75 03 91 01 95 06 75 08 15 00 25 65 05 07 19 00 29 65 81 00 C0"
	idKeyboard   = "05 01 09 06 A1 01 85 01 05 07 19 E0 29 E7 15 00 25 01 75 01 95 08 81 02 95 01 75 08 81 01 95 06 75 08 15 00 25 65 05 07 19 00 29 65 81 00 C0" +
		"05 0C 09 01 A1 01 85 02 15 00 26 FF 03 19 00 2A FF 03 75 10 95 01 81 00 C0"
	nkroKeyboard = "05 01 09 06 A1 01 85 01 05 07 19 E0 29 E7 15 00 25 01 75 01 95 08 81 02 19 00 29 77 95 78 81 02 C0"
	bootMouse    = "05 01 09 02 A1 01 09 01 A1 00 05 09 19 01 29 03 15 00 25 01 95 03 75 01 81 02 95 01 75 05 81 01 05 01 09 30 09 31 09 38 15 81 25 7F 75 08 95 03 81 06 C0 C0"
	comboRecv    = "05 01 09 06 A1 01 85 01 05 07 19 E0 29 E7 15 00 25 01 75 01 95 08 81 02 95 01 75 08 81 01 95 06 75 08 15 00 25 65 19 00 29 65 81 00 C0" +
		"05 01 09 02 A1 01 85 02 09 01 A1 00 05 09 19 01 29 05 15 00 25 01 95 05 75 01 81 02 95 01 75 03 81 01 05 01 16 01 F8 26 FF 07 75 0C 95 02 09 30 09 31 81 06 15 81 25 7F 75 08 95 01 09 38 81 06 C0 C0"
	gamepad = "05 01 09 05 A1 01 15 00 25 01 35 00 45 01 75 01 95 0E 05 09 19 01 29 0E 81 02 95 02 81 01 05 01 25 07 46 3B 01 75 04 95 01 65 14 09 39 81 42 65 00 95 01 81 01 26 FF 00 46 FF 00 09 30 09 31 09 32 09 35 75 08 95 04 81 02 C0"
)

func TestClassify(t *testing.T) {
	for _, c := range []struct {
		name       string
		d          string
		kbd, mouse bool
	}{
		{"boot keyboard", bootKeyboard, true, false},
		{"keyboard with report IDs", idKeyboard, true, false},
		{"NKRO keyboard", nkroKeyboard, true, false},
		{"boot mouse", bootMouse, false, true},
		{"combo receiver", comboRecv, true, true},
		{"gamepad", gamepad, false, false},
	} {
		l := parseHIDDescriptor(desc(t, c.d))
		if l.isKeyboard != c.kbd || l.isMouse != c.mouse {
			t.Errorf("%s: keyboard=%v mouse=%v, want %v %v", c.name, l.isKeyboard, l.isMouse, c.kbd, c.mouse)
		}
	}
}

// feed runs reports through a device and returns the event names.
func feed(t *testing.T, d string, kbd, mouse bool, reports ...string) []string {
	dev := &hidDevice{name: "test", layout: parseHIDDescriptor(desc(t, d)), kbd: kbd, mouse: mouse,
		keysDown: map[int]map[uint16]bool{}, btnsDown: map[int]bool{}}
	out := make(chan Event, 100)
	now := time.Now()
	for _, r := range reports {
		dev.handle(desc(t, r), out, now)
		now = now.Add(10 * time.Millisecond)
	}
	close(out)
	var names []string
	for ev := range out {
		names = append(names, ev.Name)
	}
	return names
}

func TestKeyboards(t *testing.T) {
	check := func(name string, got []string, want ...string) {
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %v, want %v", name, got, want)
		}
	}
	// a pressed, a held + b pressed, released, then left shift + backtick
	check("boot", feed(t, bootKeyboard, true, false,
		"00 00 04 00 00 00 00 00", "00 00 04 05 00 00 00 00", "00 00 00 00 00 00 00 00",
		"02 00 35 00 00 00 00 00"),
		"a", "b", "leftshift", "`")
	// same keys behind report ID 1; a consumer (ID 2) report is ignored
	check("report IDs", feed(t, idKeyboard, true, false,
		"01 00 00 04 00 00 00 00 00", "02 E9 00", "01 00 00 04 28 00 00 00 00"),
		"a", "enter")
	// too many keys: nothing reported, and state kept
	check("rollover", feed(t, bootKeyboard, true, false,
		"00 00 04 00 00 00 00 00", "00 00 01 01 01 01 01 01", "00 00 04 00 00 00 00 00"),
		"a")
	// NKRO bitmap: bit 4 = a, bit 0x50 = left arrow, bit 0x4F = right arrow
	nk := make([]byte, 1+1+15)
	nk[0] = 1
	set := func(u int) { nk[2+u/8] |= 1 << (u % 8) }
	set(0x04)
	r1 := hex.EncodeToString(nk)
	set(0x50)
	set(0x4F)
	r2 := hex.EncodeToString(nk)
	check("NKRO", feed(t, nkroKeyboard, true, false, r1, r2), "a", "right", "left")
}

func TestMice(t *testing.T) {
	check := func(name string, got []string, want ...string) {
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %v, want %v", name, got, want)
		}
	}
	// left click, held while moving (no repeat), right click, wheel up,
	// then small moves adding up to a swipe left
	check("boot mouse", feed(t, bootMouse, false, true,
		"01 00 00 00", "01 05 05 00", "00 00 00 00", "02 00 00 00", "00 00 00 01",
		"00 E2 00 00", "00 E2 00 00", "00 E2 00 00"),
		"left", "right", "wheelup", "swipeleft")
	// combo: keyboard 'a' on ID 1, mouse on ID 2 with 12-bit X/Y (Y = +80: down)
	check("combo", feed(t, comboRecv, true, true,
		"01 00 00 04 00 00 00 00 00", "02 01 00 00 00 00", "02 00 00 00 05 00"),
		"a", "left", "swipedown")
	// the keyboard setting off: only the mouse is decoded
	check("combo, mouse only", feed(t, comboRecv, false, true,
		"01 00 00 04 00 00 00 00 00", "02 04 00 00 00 00"),
		"middle")
}

// ---- joysticks ----

func TestJoystickLayout(t *testing.T) {
	// 8BitDo-style pad: X, Y, Z, RX, RY, RZ, HAT0X, HAT0Y
	axes := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x10, 0x11}
	mapping := map[string]string{
		"a": "b0", "b": "b1", "start": "b7",
		"leftx": "a0", "lefty": "a1", "lefttrigger": "a2", "righttrigger": "+a5",
		"rightx": "a3~",
		"dpup":   "h0.1", "dpright": "h0.2", "dpdown": "h0.4", "dpleft": "h0.8",
	}
	l := buildLayout(10, axes, mapping)
	want := map[string]string{
		"btn0": l.btnNames[0], "btn7": l.btnNames[7], "btn9": l.btnNames[9],
		"ax0-": l.axNames[0][0], "ax0+": l.axNames[0][1],
		"ax2+": l.axNames[2][1], "ax3-": l.axNames[3][0], "ax5+": l.axNames[5][1],
		"ax5-": l.axNames[5][0], "hatX-": l.axNames[6][0], "hatY+": l.axNames[7][1],
	}
	exp := map[string]string{
		"btn0": "a", "btn7": "start", "btn9": "btn9",
		"ax0-": "leftx-", "ax0+": "leftx+", "ax2+": "lefttrigger+",
		"ax3-": "rightx+", // inverted
		"ax5+": "righttrigger", "ax5-": "axis5-", "hatX-": "dpleft", "hatY+": "dpdown",
	}
	if !reflect.DeepEqual(want, exp) {
		t.Errorf("layout:\n got  %v\n want %v", want, exp)
	}

	// SDL numbers axes without the hats: its a2 is the 4th axis here.
	l = buildLayout(2, []byte{0x00, 0x01, 0x10, 0x11, 0x02}, map[string]string{"lefttrigger": "a2"})
	if l.axNames[4][1] != "lefttrigger+" || l.axNames[2][0] != "dpleft" {
		t.Errorf("hat-skipping: got %v", l.axNames)
	}

	// Unknown pad: raw names, hats still named as a d-pad.
	l = buildLayout(2, []byte{0x00, 0x10, 0x11}, nil)
	if l.btnNames[1] != "btn1" || l.axNames[0][1] != "axis0+" || l.axNames[1][1] != "dpright" || l.axNames[2][0] != "dpup" {
		t.Errorf("unknown pad: %v %v", l.btnNames, l.axNames)
	}
}

// jsState writes what the kernel returns when a joystick is opened: an
// initial-state event for every button, then every axis.
func jsState(t *testing.T, path string, buttons []int16, axes []int16) {
	var b []byte
	ev := func(val int16, typ, num byte) {
		e := make([]byte, 8)
		binary.LittleEndian.PutUint16(e[4:], uint16(val))
		e[6], e[7] = typ|0x80, num
		b = append(b, e...)
	}
	for i, v := range buttons {
		ev(v, 1, byte(i))
	}
	for i, v := range axes {
		ev(v, 2, byte(i))
	}
	if err := os.WriteFile(path, b, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestJoystickPoll(t *testing.T) {
	path := filepath.Join(t.TempDir(), "js0")
	axes := []byte{0x00, 0x01, 0x02, 0x10, 0x11} // X, Y, trigger, hat
	d := &jsDevice{path: path, name: "pad",
		layout: buildLayout(3, axes, map[string]string{"a": "b0", "leftx": "a0", "lefttrigger": "a2"}),
		btn:    make([]bool, 3), axis: make([]int8, 5), rest: make([]int8, 5), codes: axes}
	out := make(chan Event, 100)
	buf := make([]byte, 1024)
	step := func(btn []int16, ax []int16) {
		jsState(t, path, btn, ax)
		if err := d.poll(out, buf); err != nil {
			t.Fatal(err)
		}
	}
	rest := int16(-32767)
	step([]int16{0, 1, 0}, []int16{0, 0, rest, 0, 0})          // first read: btn1 already held, trigger at rest
	step([]int16{1, 1, 0}, []int16{0, 0, rest, 0, 0})          // a pressed (btn1 still held: no repeat)
	step([]int16{1, 1, 1}, []int16{-30000, 0, rest, 0, 0})     // btn2 + stick left
	step([]int16{1, 1, 1}, []int16{-32000, 0, rest, 0, 0})     // stick still left: nothing
	step([]int16{0, 0, 0}, []int16{0, 0, 32767, -32767, 0})    // release all, trigger pulled, d-pad left
	step([]int16{0, 0, 0}, []int16{0, 0, rest, -32767, 32767}) // d-pad left + down (only down is new)
	close(out)
	var got []string
	for ev := range out {
		got = append(got, ev.Name)
	}
	want := []string{"a", "btn2", "leftx-", "lefttrigger+", "dpleft", "dpdown"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v\nwant %v", got, want)
	}

	// Unplugged: poll reports it.
	os.Remove(path)
	if err := d.poll(make(chan Event, 1), buf); err == nil {
		t.Error("expected an error for a missing device")
	}
}

func TestAxisHints(t *testing.T) {
	for _, c := range []struct {
		name string
		code byte
		pos  bool
		want string
	}{
		{"lefty-", 1, false, "left stick up"},
		{"leftx+", 0, true, "left stick right"},
		{"righttrigger+", 5, true, "right trigger"},
		{"axis0-", 0x00, false, "left"},
		{"axis1+", 0x01, true, "down"},
		{"axis4-", 0x04, false, "right stick up"},
		{"axis2+", 0x02, true, ""}, // Z: could be anything
		{"dpleft", 0x10, false, ""},
	} {
		if got := axisHint(c.name, c.code, c.pos); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
	ev := Event{Kind: "joystick", Device: "Twin USB Joystick", Name: "lefty-", Hint: "left stick up"}
	if ev.String() != "joystick (Twin USB Joystick): lefty- (left stick up)" {
		t.Errorf("String: %s", ev)
	}
}
