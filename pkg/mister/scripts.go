package mister

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/synrais/SAMenu/pkg/config"
	"github.com/synrais/SAMenu/pkg/input/virtualinput"
)

type Script struct {
	Name     string `json:"name"`
	Filename string `json:"filename"`
	Path     string `json:"path"`
}

func IsMenuRunning() bool {
	activeCore, err := GetActiveCoreName()
	if err != nil {
		return false
	}
	return activeCore == config.MenuCore
}

// RunOnScreen runs a program on the MiSTer's own screen (the Linux
// console), the way the Scripts menu does, but started from outside: it
// presses F9 on a virtual keyboard, which is MiSTer's key for showing the
// console. Only works while the MiSTer menu core is loaded. It returns
// when the program exits. Unlike RunScript it doesn't press F12 afterwards
// (SAMenu leaves the console itself).
// OpenMenuLog records each step of opening SAMenu on the TV.
const OpenMenuLog = "/tmp/SAMenu_openmenu.log"

// stepLog prints a step and appends it to OpenMenuLog.
func stepLog(format string, args ...interface{}) {
	line := time.Now().Format("15:04:05.000") + " " + fmt.Sprintf(format, args...)
	fmt.Println(line)
	if f, err := os.OpenFile(OpenMenuLog, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
		fmt.Fprintln(f, line)
		f.Close()
	}
}

func activeTty() string {
	b, err := os.ReadFile("/sys/devices/virtual/tty/tty0/active")
	if err != nil {
		return "unknown (" + err.Error() + ")"
	}
	return strings.TrimSpace(string(b))
}

func RunOnScreen(exe string, args ...string) error {
	core, _ := GetActiveCoreName()
	stepLog("core loaded: %q, active console: %s", core, activeTty())
	if !IsMenuRunning() {
		return fmt.Errorf("the MiSTer menu core isn't loaded")
	}
	kbd, err := virtualinput.NewKeyboard(40 * time.Millisecond)
	if err != nil {
		return fmt.Errorf("failed to create virtual keyboard: %w", err)
	}
	defer kbd.Close()
	stepLog("created SAMenu's virtual keyboard")
	time.Sleep(time.Second) // MiSTer needs a moment to find a new device

	if out, err := exec.Command("chvt", "3").CombinedOutput(); err != nil {
		return fmt.Errorf("chvt 3: %v %s", err, out)
	}
	stepLog("switched to console 3, now: %s; pressing F9", activeTty())
	f9, ok := virtualinput.ToKeyboardCode("f9")
	if !ok {
		return fmt.Errorf("no F9 key in the virtual keyboard map")
	}
	onConsole := false
	for i := 0; i < 60 && !onConsole; i++ { // up to about 3 seconds
		_ = kbd.Press(f9)
		time.Sleep(50 * time.Millisecond)
		if b, err := os.ReadFile("/sys/devices/virtual/tty/tty0/active"); err == nil &&
			strings.TrimSpace(string(b)) == "tty1" {
			onConsole = true
		}
	}
	if !onConsole {
		return fmt.Errorf("couldn't show the console with F9 (console is %s, wanted tty1)", activeTty())
	}
	stepLog("F9 worked, console: %s", activeTty())
	if out, err := exec.Command("chvt", "2").CombinedOutput(); err != nil {
		return fmt.Errorf("chvt 2: %v %s", err, out)
	}
	stepLog("switched to console 2 (%s), starting SAMenu there", activeTty())
	quoted := make([]string, 0, len(args)+1)
	for _, a := range append([]string{exe}, args...) {
		quoted = append(quoted, "'"+strings.ReplaceAll(a, "'", `'\''`)+"'")
	}
	launcher := "#!/bin/bash\nexport LC_ALL=en_US.UTF-8\nexport HOME=/root\nexport LESSKEY=/media/fat/linux/lesskey\ncd " +
		filepath.Dir(exe) + "\n" + strings.Join(quoted, " ") + "\n"
	if err := os.WriteFile("/tmp/script", []byte(launcher), 0755); err != nil {
		return err
	}
	out, err := exec.Command("/sbin/agetty", "-a", "root", "-l", "/tmp/script", "--nohostname", "-L", "tty2", "linux").CombinedOutput()
	stepLog("SAMenu on console 2 ended: err=%v %s", err, strings.TrimSpace(string(out)))
	return err
}

// OpenGamesMenu closes whatever game is running (loading the MiSTer menu
// core) and opens SAMenu on the TV, straight into its Search
// screen if search is true.
func OpenGamesMenu(search bool) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	_ = os.Remove(OpenMenuLog)
	stepLog("opening SAMenu on the TV (search=%v)", search)
	if !IsMenuRunning() {
		stepLog("loading the MiSTer menu core")
		if err := LaunchMenu(); err != nil {
			return err
		}
		deadline := time.Now().Add(20 * time.Second)
		for !IsMenuRunning() || !MainRunning() {
			if time.Now().After(deadline) {
				return fmt.Errorf("the MiSTer menu didn't load")
			}
			time.Sleep(250 * time.Millisecond)
		}
		stepLog("menu core loaded")
		time.Sleep(2 * time.Second) // let the menu core settle
	}
	var args []string
	if search {
		args = append(args, "-search")
	}
	return RunOnScreen(exe, args...)
}
