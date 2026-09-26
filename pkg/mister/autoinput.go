package mister

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bendahl/uinput"

	"github.com/synrais/SAMenu/pkg/config"
	"github.com/synrais/SAMenu/pkg/games"
	"github.com/synrais/SAMenu/pkg/input/virtualinput"
)

// BIOS skip
//
// Presses buttons on SAMenu's virtual gamepad (or keys on its virtual
// keyboard), e.g. to get past a BIOS screen after a game loads.
//
// MiSTer treats the virtual pad like a newly plugged-in controller. It has
// no saved mapping, so MiSTer uses its built-in default map: East = A,
// South = B, North = X, West = Y, shoulders L/R, Select, Start, Home = OSD,
// and the hat for the d-pad. Loading a core restarts MiSTer's main
// program, which clears every player slot, and the first controller to
// press a button becomes player 1: so the virtual pad is player 1 unless a
// real controller is pressed first. It's removed once the sequence is
// done, freeing the slot again.

// padButtons are the pad names, in MiSTer's default map.
var padButtons = map[string]int{
	"a": uinput.ButtonEast, "b": uinput.ButtonSouth,
	"x": uinput.ButtonNorth, "y": uinput.ButtonWest,
	"l": uinput.ButtonBumperLeft, "r": uinput.ButtonBumperRight,
	"select": uinput.ButtonSelect, "start": uinput.ButtonStart,
	"home": uinput.ButtonMode,
}

var padHat = map[string]uinput.HatDirection{
	"up": uinput.HatUp, "down": uinput.HatDown, "left": uinput.HatLeft, "right": uinput.HatRight,
}

// Step is one item of a sequence: a pause, a pad button, a d-pad direction
// or a keyboard key.
type Step struct {
	pause  time.Duration
	button int
	hat    uinput.HatDirection
	key    int
	text   string
}

// ParseSequence reads a sequence like "10, start, 1, a, key:enter": a
// number is a pause in seconds, key:<name> a keyboard key, anything else a
// pad button or d-pad direction.
func ParseSequence(s string) ([]Step, error) {
	var steps []Step
	for _, raw := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' }) {
		t := strings.ToLower(strings.TrimSpace(raw))
		if t == "" {
			continue
		}
		if secs, err := strconv.ParseFloat(t, 64); err == nil {
			steps = append(steps, Step{pause: time.Duration(secs * float64(time.Second)), text: t + "s"})
			continue
		}
		if name, ok := strings.CutPrefix(t, "key:"); ok {
			code, found := virtualinput.KeyboardMap["{"+name+"}"]
			if !found {
				code, found = virtualinput.KeyboardMap[name]
			}
			if !found {
				return nil, fmt.Errorf("unknown key %q", name)
			}
			steps = append(steps, Step{key: code, text: t})
			continue
		}
		if b, ok := padButtons[t]; ok {
			steps = append(steps, Step{button: b, text: t})
			continue
		}
		if h, ok := padHat[t]; ok {
			steps = append(steps, Step{hat: h, text: t})
			continue
		}
		return nil, fmt.Errorf("unknown button %q (pad: a b x y l r select start home up down left right; keyboard: key:<name>)", t)
	}
	return steps, nil
}

// VirtualDevices are the virtual pad and keyboard a sequence needs.
type VirtualDevices struct {
	pad    *virtualinput.Gamepad
	kbd    *virtualinput.Keyboard
	closed bool
}

// OpenDevices creates the virtual devices a sequence uses. Create them
// before launching the game, so MiSTer finds them when it starts the core.
func OpenDevices(steps []Step) (*VirtualDevices, error) {
	d := &VirtualDevices{}
	for _, s := range steps {
		if (s.button != 0 || s.hat != 0) && d.pad == nil {
			p, err := virtualinput.NewGamepad(80 * time.Millisecond)
			if err != nil {
				d.Close()
				return nil, err
			}
			d.pad = &p
		}
		if s.key != 0 && d.kbd == nil {
			k, err := virtualinput.NewKeyboard(80 * time.Millisecond)
			if err != nil {
				d.Close()
				return nil, err
			}
			d.kbd = &k
		}
	}
	return d, nil
}

// Close removes the virtual devices, freeing their player slots.
func (d *VirtualDevices) Close() {
	if d == nil || d.closed {
		return
	}
	d.closed = true
	if d.pad != nil {
		_ = d.pad.Close()
	}
	if d.kbd != nil {
		_ = d.kbd.Close()
	}
}

// Run performs the steps in order, telling log about each one.
func (d *VirtualDevices) Run(steps []Step, log func(string)) error {
	for _, s := range steps {
		var err error
		switch {
		case s.pause > 0:
			time.Sleep(s.pause)
			continue
		case s.button != 0:
			err = d.pad.Press(s.button)
		case s.hat != 0:
			if err = d.pad.Device.HatPress(s.hat); err == nil {
				time.Sleep(d.pad.Delay)
				err = d.pad.Device.HatRelease(s.hat)
			}
		case s.key != 0:
			err = d.kbd.Press(s.key)
		}
		if err != nil {
			return fmt.Errorf("pressing %s: %w", s.text, err)
		}
		if log != nil {
			log("pressed " + s.text)
		}
	}
	return nil
}

// AutoInputFrom says where launches come from, for the [BiosSkip]
// switches: "menu" (the default) or "attract" (set by attract mode).
var AutoInputFrom = "menu"

const autoInputPid = "/tmp/SAMenu_autoinput.pid"

// startAutoInput runs the system's [BiosSkip.<system>] sequence, if it
// has one and auto input is on for where this launch came from. It runs
// as its own background process ("SAMenu -autoinput <sequence>"), so it
// keeps going after SAMenu exits, and any earlier one still
// waiting is stopped first (so a quick "next" can't press into the wrong
// game). The virtual pad is created before the game launches, the way
// MiSTer finds it most reliably.
func startAutoInput(cfg *config.Config, system games.System) {
	on := cfg.AutoInput.Menu
	if AutoInputFrom == "attract" {
		on = cfg.AutoInput.Attract
	}
	stopAutoInput()
	seq := cfg.AutoInput.Sequences[strings.ToLower(system.Id)]
	if !on || seq == "" {
		return
	}
	if _, err := ParseSequence(seq); err != nil {
		fmt.Printf("[BiosSkip] %s: %v\n", system.Id, err)
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	cmd := exec.Command(exe, "-autoinput", seq)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		fmt.Printf("[BiosSkip] couldn't start: %v\n", err)
		return
	}
	_ = os.WriteFile(autoInputPid, []byte(strconv.Itoa(cmd.Process.Pid)), 0644)
	_ = cmd.Process.Release()
	time.Sleep(300 * time.Millisecond) // let it create the virtual pad
}

// stopAutoInput stops an auto input run that's still waiting.
func stopAutoInput() {
	b, err := os.ReadFile(autoInputPid)
	if err != nil {
		return
	}
	_ = os.Remove(autoInputPid)
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || pid <= 1 {
		return
	}
	// Only if it's really one of ours (the PID may have been reused).
	if cmdline, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid)); err == nil &&
		strings.Contains(string(cmdline), "-autoinput") {
		_ = syscall.Kill(pid, syscall.SIGTERM)
	}
}

// RunAutoInput is the background process: create the devices, run the
// sequence, remove the devices.
func RunAutoInput(seq string) {
	steps, err := ParseSequence(seq)
	if err != nil {
		return
	}
	dev, err := OpenDevices(steps)
	if err != nil {
		return
	}
	defer dev.Close()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	done := make(chan struct{})
	go func() { _ = dev.Run(steps, nil); close(done) }()
	select {
	case <-done:
	case <-sig:
	}
	_ = os.Remove(autoInputPid)
}
