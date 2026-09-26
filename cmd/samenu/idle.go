package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/synrais/SAMenu/pkg/attract"
	"github.com/synrais/SAMenu/pkg/config"
	"github.com/synrais/SAMenu/pkg/input"
	"github.com/synrais/SAMenu/pkg/mister"
	"github.com/synrais/SAMenu/pkg/video"
)

// -------------------------
// Start When Idle
// -------------------------
//
// [Startup] Attract mode "When idle": the idle watcher ("SAMenu -idlewatch") runs in
// the background (started at boot, and when the setting is switched on)
// and starts attract mode once nothing has been pressed for that many
// minutes (IdleTime), like a screensaver, counting in the MiSTer menu,
// in games, or both (IdleWhere). After you choose to play one of attract
// mode's games, this is what starts it again, with the same history.
// It stays quiet while attract mode runs and while a video plays.

const idlePidFile = "/tmp/SAMenu_idle.pid"

func idleWatcherRunning() bool {
	b, err := os.ReadFile(idlePidFile)
	if err != nil {
		return false
	}
	c, err := os.ReadFile("/proc/" + strings.TrimSpace(string(b)) + "/cmdline")
	return err == nil && strings.Contains(string(c), "-idlewatch")
}

// ensureIdleWatcher starts or stops the idle watcher to match SAMenu.ini.
func ensureIdleWatcher(cfg *config.Config) {
	running := idleWatcherRunning()
	switch {
	case cfg.IdleWatch() && !running:
		exe, err := os.Executable()
		if err != nil {
			return
		}
		cmd := exec.Command(exe, "-idlewatch")
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		if cmd.Start() == nil {
			_ = cmd.Process.Release()
		}
	case !cfg.IdleWatch() && running:
		if b, err := os.ReadFile(idlePidFile); err == nil {
			var pid int
			fmt.Sscan(strings.TrimSpace(string(b)), &pid)
			if pid > 1 {
				_ = syscall.Kill(pid, syscall.SIGTERM)
			}
		}
		_ = os.Remove(idlePidFile)
	}
}

// runIdleWatcher is the idle watcher process.
func runIdleWatcher() {
	if idleWatcherRunning() {
		return
	}
	_ = os.WriteFile(idlePidFile, []byte(fmt.Sprint(os.Getpid())), 0644)
	defer os.Remove(idlePidFile)

	events := input.Start(input.Options{Keyboard: true, Mouse: true, Joystick: true, Quiet: true})
	cfg := mustConfig()
	lastInput, lastLoad := time.Now(), time.Now()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-events:
			lastInput = time.Now()
		case now := <-tick.C:
			if now.Sub(lastLoad) > 10*time.Second { // pick up setting changes
				if c, err := config.Load(); err == nil {
					cfg = c
				}
				lastLoad = now
			}
			if !cfg.IdleWatch() {
				return // switched off
			}
			running := attract.Running()
			mister.UndoStaleMute(running)
			inMenu := mister.IsMenuRunning()
			counts := cfg.Startup.IdleWhere == config.IdleBoth ||
				(cfg.Startup.IdleWhere == config.IdleMenu && inMenu) ||
				(cfg.Startup.IdleWhere == config.IdleGames && !inMenu)
			_, busy := mister.Busy() // e.g. update_all: never start over a script
			if running || busy || video.Playing() || !counts {
				lastInput = now // not counting: attract mode, a script, a video, or not a place set in Where
				continue
			}
			if now.Sub(lastInput) >= time.Duration(cfg.Startup.IdleTime)*time.Minute {
				_ = startAttractInBackground()
				lastInput = now
			}
		}
	}
}
