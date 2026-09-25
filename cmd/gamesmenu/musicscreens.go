package main

import (
	"fmt"
	"os"

	gc "github.com/rthornton128/goncurses"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/mister"
	"github.com/synrais/SAM-GO/pkg/music"
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
			fmt.Sprintf("%-22s %s", "Start on MiSTer boot:", onOffText(cfg.Startup.Music)),
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
		case 5:
			cfg.Startup.Music = !cfg.Startup.Music
			err = saveStartup(stdscr, cfg)
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

// startupScreen is Options -> Attract Mode -> On startup: what the MiSTer
// starts when it boots. One choice, since the games menu and attract mode
// would both want the screen.
func startupScreen(stdscr *gc.Window, cfg *config.Config) {
	start := labelOption{"On MiSTer boot", []string{"Nothing", "Games menu", "Attract mode"}, 0}
	start.set(cfg.Startup.Start)
	runOptionsScreen(stdscr, cfg, optionsScreen{
		title:     "On Startup",
		noPreview: true,
		options:   func() []*labelOption { return []*labelOption{&start} },
		save: func() error {
			cfg.Startup.Start = start.value()
			return saveStartup(stdscr, cfg)
		},
	})
}

// saveStartup saves [Startup] and updates user-startup.sh to match.
func saveStartup(stdscr *gc.Window, cfg *config.Config) error {
	if err := config.SaveValues(cfg.Path, "Startup", [][2]string{
		{"Start", cfg.Startup.Start}, {"Music", boolText(cfg.Startup.Music)},
	}); err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := mister.UpdateStartup(cfg, exe); err != nil {
		return err
	}
	if cfg.Startup.Start == "Attract mode" && mister.OldSAMInStartup() {
		message(stdscr, "The old MiSTer_SAM also starts from user-startup.sh.\nRemove its line, or both will run at boot.")
	}
	return nil
}
