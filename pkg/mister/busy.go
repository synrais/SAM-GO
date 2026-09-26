package mister

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Busy
//
// Something is running that attract mode mustn't load a core over, e.g.
// update_all: loading a core would end it half way. That's a script on
// the MiSTer's own screen (started from the Scripts menu, which runs it
// through /tmp/script on the Linux console), except SAMenu
// itself, or a known updater run over SSH. The idle watcher doesn't count while busy, attract mode waits
// before its next game, and the boot countdown waits.

// busyMarks are command-line parts that mean busy.
var busyMarks = []string{
	"/tmp/script", // anything from the Scripts menu, on the screen
	"update_all",  // update_all, however it was started
	"downloader",  // MiSTer's own updater
}

// Busy reports whether a script or updater is running, and what it is.
// SAMenu's own background processes (attract mode, the idle
// watcher, music, BIOS skip) don't count.
func Busy() (string, bool) {
	self := os.Getpid()
	procs, _ := filepath.Glob("/proc/[0-9]*/cmdline")
	for _, p := range procs {
		if strings.TrimPrefix(filepath.Dir(p), "/proc/") == strconv.Itoa(self) {
			continue
		}
		b, err := os.ReadFile(p)
		if err != nil || len(b) == 0 {
			continue
		}
		cmd := strings.ReplaceAll(strings.TrimRight(string(b), "\x00"), "\x00", " ")
		if strings.Contains(cmd, "SAMenu") && !strings.Contains(cmd, "/tmp/script") {
			continue // our own background processes
		}
		for _, m := range busyMarks {
			if !strings.Contains(cmd, m) {
				continue
			}
			// SAMenu itself also runs through /tmp/script (from
			// the Scripts menu, or opened on the TV); it isn't busy.
			if m == "/tmp/script" && scriptIsGamesMenu() {
				continue
			}
			return cmd, true
		}
	}
	return "", false
}

// scriptIsGamesMenu reports whether the script on the screen is the games
// menu: /tmp/script, MiSTer's launcher for it, names what it runs.
func scriptIsGamesMenu() bool {
	b, err := os.ReadFile("/tmp/script")
	return err == nil && strings.Contains(string(b), "SAMenu")
}
