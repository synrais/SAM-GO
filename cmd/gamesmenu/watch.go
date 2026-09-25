package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/synrais/SAM-GO/pkg/attract"
	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/games"
)

// -------------------------
// Detector status viewer (gamesmenu.sh -watch)
// -------------------------

const (
	watchEvery = 100 * time.Millisecond
	staleAfter = 3 * time.Second // no update for this long = detector stopped
)

// watchDetector shows the static detector's status file, redrawn in place
// (no screen clearing, so no flashing) until Ctrl+C.
func watchDetector() {
	fmt.Print("\033[?25l\033[2J") // hide cursor, clear once
	restore := func() { fmt.Print("\033[?25h\n") }

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	ticker := time.NewTicker(watchEvery)
	defer ticker.Stop()

	last := ""
	var iniChecked time.Time
	iniNote := ""

	// When attract mode isn't running, -watch runs its own view-only
	// detector (never lists or skips) on whatever is on screen, so any
	// game can be checked while tuning. It hands back to attract mode's
	// detector as soon as attract mode starts.
	var own *attract.Detector
	ownCore := ""
	var lastTry time.Time
	startErr := ""
	defer func() {
		if own != nil {
			own.Stop()
		}
	}()

	for {
		screen := ""
		// Re-check SAM.ini at most every 2 seconds.
		if time.Since(iniChecked) > 2*time.Second {
			iniChecked = time.Now()
			iniNote = detectorIniNote()
		}
		switch {
		case attract.Running():
			if own != nil {
				own.Stop()
				own = nil
			}
			if iniNote != "" {
				// Switched off in SAM.ini: say so, never switch it on.
				screen = "Attract mode is running, but its static detector is switched off.\n\n" + iniNote
				break
			}
			info, err := os.Stat(attract.StatusFile)
			switch {
			case err != nil:
				screen = "Attract mode is running; waiting for its static detector..."
			case time.Since(info.ModTime()) > staleAfter:
				data, _ := os.ReadFile(attract.StatusFile)
				screen = fmt.Sprintf("The static detector has stopped (last update %s).\n\nLast status:\n\n%s",
					info.ModTime().Format("15:04:05"), data)
			default:
				data, _ := os.ReadFile(attract.StatusFile)
				screen = string(data)
			}
		default:
			core := currentCore()
			if (own == nil && time.Since(lastTry) > 2*time.Second) || (own != nil && core != ownCore) {
				lastTry = time.Now()
				if own != nil {
					own.Stop()
				}
				own, ownCore = nil, core
				if cfg, err := config.Load(); err == nil {
					own, err = attract.StartViewOnlyDetector(cfg, coreSystem(core))
					startErr = ""
					if err != nil {
						startErr = "Couldn't start the static detector: " + err.Error()
					}
				}
			}
			header := "VIEW ONLY: attract mode isn't running, so nothing is skipped or listed.\n"
			if core != "" {
				header += "Core: " + core
				if id := coreSystem(core); id != "" {
					header += " (using [StaticDetector." + id + "] settings, if any)"
				}
				header += "\n"
			}
			if startErr != "" {
				screen = startErr
			}
			if screen == "" {
				data, _ := os.ReadFile(attract.StatusFile)
				screen = header + "\n" + string(data)
			}
		}
		screen += "\n(Ctrl+C to stop watching)"

		if screen != last {
			draw(screen)
			last = screen
		}

		select {
		case <-stop:
			restore()
			return
		case <-ticker.C:
		}
	}
}

// draw writes the screen from the top-left corner over the previous one,
// clearing only the leftover end of each line and anything below.
func draw(screen string) {
	var b strings.Builder
	b.WriteString("\033[H")
	for _, line := range strings.Split(strings.TrimRight(screen, "\n"), "\n") {
		b.WriteString(line)
		b.WriteString("\033[K\n")
	}
	b.WriteString("\033[J")
	fmt.Print(b.String())
}

// detectorIniNote explains that the detector is switched off in SAM.ini,
// or returns "" when it's on.
func detectorIniNote() string {
	cfg, err := config.Load()
	if err != nil {
		return "Couldn't read SAM.ini: " + err.Error()
	}
	if !cfg.Attract.UseStaticDetector {
		return "It's switched off in " + cfg.Path + ":\n" +
			"  [Attract] UseStaticDetector = false\n" +
			"(or Options -> Attract Mode -> Detector & list settings)"
	}
	return ""
}

// currentCore is the core MiSTer has loaded (from /tmp/CORENAME).
func currentCore() string {
	b, _ := os.ReadFile("/tmp/CORENAME")
	return strings.TrimSpace(string(b))
}

// coreSystem finds the system ID for a core name, e.g. "NES" -> "NES",
// "ATARI800" -> "Atari800", or "" if none matches.
func coreSystem(core string) string {
	if core == "" {
		return ""
	}
	for _, s := range games.Systems {
		if strings.EqualFold(filepath.Base(s.Rbf), core) {
			return s.Id
		}
	}
	return ""
}
