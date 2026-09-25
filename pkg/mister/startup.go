package mister

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/synrais/SAM-GO/pkg/config"
)

// MiSTer startup
//
// MiSTer runs /media/fat/linux/user-startup.sh at boot. The games menu
// keeps its startup lines in one marked block there, and only ever
// changes or removes that block, so other tools' lines stay untouched.

const (
	startupBegin = "# BEGIN gamesmenu (set in the games menu: Options)"
	startupEnd   = "# END gamesmenu"
)

// UpdateStartup rewrites the games menu's block in user-startup.sh from
// [Startup] (exe is the games menu itself). No block is left when nothing
// is set to start.
func UpdateStartup(cfg *config.Config, exe string) error {
	data, err := os.ReadFile(config.StartupFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	text := string(data)
	if text == "" {
		text = "#!/bin/sh\n"
	}

	// Remove our old block.
	var kept []string
	inBlock := false
	for _, line := range strings.Split(text, "\n") {
		switch {
		case strings.HasPrefix(line, "# BEGIN gamesmenu"):
			inBlock = true
		case inBlock && line == startupEnd:
			inBlock = false
		case !inBlock:
			kept = append(kept, line)
		}
	}
	text = strings.TrimRight(strings.Join(kept, "\n"), "\n") + "\n"

	var lines []string
	if cfg.Startup.Music {
		lines = append(lines, fmt.Sprintf("%q -music start > /dev/null 2>&1 &", exe))
	}
	switch cfg.Startup.Start {
	case "Games menu":
		lines = append(lines, fmt.Sprintf("%q -boot menu > /dev/null 2>&1 &", exe))
	case "Attract mode":
		lines = append(lines, fmt.Sprintf("%q -boot attract > /dev/null 2>&1 &", exe))
	}
	if len(lines) > 0 {
		text += "\n" + startupBegin + "\n" + strings.Join(lines, "\n") + "\n" + startupEnd + "\n"
	}

	tmp := config.StartupFile + ".tmp"
	if err := os.WriteFile(tmp, []byte(text), 0755); err != nil {
		return err
	}
	return os.Rename(tmp, config.StartupFile)
}

// OldSAMInStartup reports whether the old MiSTer_SAM still starts from
// user-startup.sh, which would fight a startup attract mode.
func OldSAMInStartup() bool {
	data, err := os.ReadFile(config.StartupFile)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		l := strings.TrimSpace(line)
		if !strings.HasPrefix(l, "#") && strings.Contains(l, "MiSTer_SAM") {
			return true
		}
	}
	return false
}

// WaitForMiSTer waits (up to 2 minutes) until MiSTer's main program is
// running with the menu core loaded, as at the end of a boot.
func WaitForMiSTer() error {
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(config.CmdInterface); err == nil && MainRunning() && IsMenuRunning() {
			time.Sleep(3 * time.Second) // let it settle
			return nil
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("MiSTer didn't finish starting")
}
