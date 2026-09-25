package mister

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/synrais/SAM-GO/pkg/config"
)

// Main program watchdog
//
// MiSTer's main program (the menu, the OSD, passing controllers to cores)
// can die on a bad file, e.g. the Atari 800 core crash. The core keeps
// running but nothing responds. MainRunning spots that, and RestartMain
// starts it again, the same as running this by hand:
//
//	cd /media/fat && ./MiSTer > /tmp/MiSTer.log 2>&1 &

// mainLog keeps the restarted program's messages (in RAM), so a crash can
// be looked into: tail -50 /tmp/MiSTer.log
const mainLog = "/tmp/MiSTer.log"

// MainRunning reports whether the MiSTer main program is running.
func MainRunning() bool {
	dirs, _ := filepath.Glob("/proc/[0-9]*/comm")
	for _, f := range dirs {
		if b, err := os.ReadFile(f); err == nil && strings.TrimSpace(string(b)) == "MiSTer" {
			return true
		}
	}
	return false
}

// RestartMain starts the MiSTer main program in its own session and waits
// for it to be ready to take commands.
func RestartMain() error {
	exe := filepath.Join(config.SdFolder, "MiSTer")
	logFile, err := os.OpenFile(mainLog, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer logFile.Close()

	cmd := exec.Command(exe)
	cmd.Dir = config.SdFolder
	cmd.Stdout, cmd.Stderr = logFile, logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	_ = cmd.Process.Release()

	// Give it time to start up and load the menu core before the next
	// launch command arrives.
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if MainRunning() {
			if _, err := os.Stat(config.CmdInterface); err == nil {
				time.Sleep(3 * time.Second)
				return nil
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("MiSTer main program didn't come back up")
}

// EnsureMain restarts the main program if it isn't running, and reports
// whether it had to.
func EnsureMain() (restarted bool, err error) {
	if MainRunning() {
		return false, nil
	}
	return true, RestartMain()
}
