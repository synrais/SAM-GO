package main

import (
	"fmt"
	"os"

	gc "github.com/rthornton128/goncurses"

	"github.com/synrais/SAMenu/pkg/config"
	"github.com/synrais/SAMenu/pkg/mister"
	"github.com/synrais/SAMenu/pkg/music"
)

// -------------------------
// Music Player and Startup
// -------------------------

func onOffText(b bool) string {
	if b {
		return "On"
	}
	return "Off"
}

// playlistName shows a playlist setting ("" is the music folder itself).
func playlistName(p string) string {
	if p == "" {
		return "Music folder"
	}
	return p
}

// musicScreen is Options -> Music Player: start or stop the player, skip a
// track, and its settings ([Music], and [Startup] Music).
func musicScreen(stdscr *gc.Window, cfg *config.Config) {
	m := &cfg.Music
	selected := 0
	for {
		playing := music.Running()
		toggle := "Start music"
		if playing {
			toggle = "Stop music"
		}
		items := []string{
			fmt.Sprintf("%-22s (%s)", toggle, fitText(music.Status(), optionsWidth-32)),
			"Next track",
			fmt.Sprintf("%-22s %s", "Playback:", m.Playback),
			fmt.Sprintf("%-22s %s", "Playlist:", playlistName(m.Playlist)),
			fmt.Sprintf("%-22s %s", "Pause during games:", onOffText(m.PauseInGames)),
		}
		sel, ok := optionsList(stdscr, "Music Player", items, selected)
		if !ok {
			return
		}
		selected = sel
		var err error
		switch sel {
		case 0:
			if playing {
				music.Stop()
			} else {
				var exe string
				if exe, err = os.Executable(); err == nil {
					err = music.Start(exe)
				}
			}
		case 1:
			if music.Send("next") != nil {
				message(stdscr, "The music player isn't running.")
			}
			continue
		case 2:
			if m.Playback == "Random" {
				m.Playback = "In order"
			} else {
				m.Playback = "Random"
			}
		case 3:
			lists := music.Playlists()
			next := 0
			for i, p := range lists {
				if p == m.Playlist {
					next = (i + 1) % len(lists)
				}
			}
			m.Playlist = lists[next]
		case 4:
			m.PauseInGames = !m.PauseInGames
		}
		if sel >= 2 && sel <= 4 {
			err = config.SaveValues(cfg.Path, "Music", [][2]string{
				{"Playback", m.Playback}, {"Playlist", m.Playlist}, {"PauseInGames", boolText(m.PauseInGames)},
			})
		}
		if err != nil {
			message(stdscr, fmt.Sprintf("Couldn't do that: %v", err))
		}
	}
}

// startupScreen is Options -> Startup: what happens when the MiSTer boots,
// and (for attract mode "When idle") from then on.
func startupScreen(stdscr *gc.Window, cfg *config.Config) {
	st := &cfg.Startup
	start := labelOption{"On boot", []string{"Nothing", "SAMenu", "Attract mode"}, 0}
	start.set(st.Start)
	when := labelOption{"Attract starts", []string{"Instantly", "After a delay", "When idle"}, 0}
	when.set(st.AttractWhen)

	delays := []string{"30 sec", "1 min", "2 min", "5 min"}
	delaySecs := []int{30, 60, 120, 300}
	delay := labelOption{"Delay", delays, 1}
	for i, s := range delaySecs {
		if s == st.AttractDelay {
			delay.index = i
		}
	}
	press := labelOption{"A press during it", []string{"Restarts the countdown", "Cancels it", "Is ignored"}, 0}
	press.set(st.AttractPress)

	idleChoicesOn := idleChoices[1:] // no "Off": When idle needs a time
	idle := labelOption{"Idle time", append([]string(nil), idleChoicesOn...), 2}
	idle.set(fmt.Sprintf("%d min", st.IdleTime))
	where := labelOption{"Where", []string{config.IdleMenu, config.IdleGames, config.IdleBoth}, 0}
	where.set(st.IdleWhere)
	musicOn := onOffOption("Music on boot", st.Music)

	runOptionsScreen(stdscr, cfg, optionsScreen{
		title:     "Startup",
		noPreview: true,
		options: func() []*labelOption {
			opts := []*labelOption{&start}
			if start.value() == "Attract mode" {
				opts = append(opts, &when)
				switch when.value() {
				case "After a delay":
					opts = append(opts, &delay, &press)
				case "When idle":
					opts = append(opts, &idle, &where)
				}
			}
			return append(opts, &musicOn)
		},
		save: func() error {
			st.Start, st.Music, st.AttractWhen = start.value(), musicOn.isOn(), when.value()
			st.AttractDelay, st.AttractPress = delaySecs[delay.index], press.value()
			st.IdleTime, st.IdleWhere = idleMinutes(idle), where.value()
			return saveStartup(stdscr, cfg)
		},
	})
}

// saveStartup saves [Startup] and updates user-startup.sh to match.
func saveStartup(stdscr *gc.Window, cfg *config.Config) error {
	if err := config.SaveValues(cfg.Path, "Startup", [][2]string{
		{"Start", cfg.Startup.Start}, {"Music", boolText(cfg.Startup.Music)},
		{"AttractWhen", cfg.Startup.AttractWhen},
		{"AttractDelay", fmt.Sprint(cfg.Startup.AttractDelay)}, {"AttractPress", cfg.Startup.AttractPress},
		{"IdleTime", fmt.Sprint(cfg.Startup.IdleTime)}, {"IdleWhere", cfg.Startup.IdleWhere},
	}); err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	ensureIdleWatcher(cfg)
	if err := mister.UpdateStartup(cfg, exe); err != nil {
		return err
	}
	if cfg.Startup.Start == "Attract mode" && mister.OldSAMInStartup() {
		message(stdscr, "The old MiSTer_SAM also starts from user-startup.sh.\nRemove its line, or both will run at boot.")
	}
	return nil
}
