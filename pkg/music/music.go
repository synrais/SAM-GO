// Package music is the background music player: it plays tracks from
// /media/fat/music (or a playlist folder inside it) through mpg123 (MP3)
// or ogg123 (OGG), one after another, and pauses while a game core is
// loaded ([Music] PauseInGames), since MiSTer mixes Linux audio into
// every core's sound. It runs as its own process ("gamesmenu -musicd"),
// controlled through a command pipe like attract mode.
package music

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/synrais/SAM-GO/pkg/config"
)

const (
	Folder      = "/media/fat/music"
	CommandPipe = "/tmp/gamesmenu_music.cmd"
	StatusFile  = "/tmp/gamesmenu_music.status"
	PidFile     = "/tmp/gamesmenu_music.pid"
)

// Playlists lists the folders inside the music folder (each is a
// playlist); "" is the music folder itself, "All" everything.
func Playlists() []string {
	out := []string{"All", ""}
	entries, _ := os.ReadDir(Folder)
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out
}

// Tracks lists the playable files for a playlist.
func Tracks(playlist string) []string {
	var tracks []string
	add := func(path string) {
		l := strings.ToLower(path)
		if strings.HasSuffix(l, ".mp3") || strings.HasSuffix(l, ".ogg") {
			tracks = append(tracks, path)
		}
	}
	switch playlist {
	case "All", "all":
		_ = filepath.WalkDir(Folder, func(p string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				add(p)
			}
			return nil
		})
	default:
		dir := Folder
		if playlist != "" {
			dir = filepath.Join(Folder, playlist)
		}
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			if !e.IsDir() {
				add(filepath.Join(dir, e.Name()))
			}
		}
	}
	sort.Strings(tracks)
	return tracks
}

// ----- controlling a running player -----

// Running reports whether the player is running.
func Running() bool {
	b, err := os.ReadFile(PidFile)
	if err != nil {
		return false
	}
	c, err := os.ReadFile("/proc/" + strings.TrimSpace(string(b)) + "/cmdline")
	return err == nil && strings.Contains(string(c), "-musicd")
}

// Send sends a command ("next", "stop") to the running player.
func Send(cmd string) error {
	f, err := os.OpenFile(CommandPipe, os.O_WRONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return fmt.Errorf("the music player isn't running")
	}
	defer f.Close()
	_, err = f.WriteString(cmd + "\n")
	return err
}

// Start runs the player in the background (exe is gamesmenu itself).
func Start(exe string) error {
	if Running() {
		return nil
	}
	cmd := exec.Command(exe, "-musicd")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	_ = cmd.Process.Release()
	for i := 0; i < 20 && !Running(); i++ {
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}

// Stop stops the running player.
func Stop() {
	if Send("stop") != nil {
		return
	}
	for i := 0; i < 20 && Running(); i++ {
		time.Sleep(100 * time.Millisecond)
	}
}

// Status says what the player is doing.
func Status() string {
	if !Running() {
		return "Stopped"
	}
	b, _ := os.ReadFile(StatusFile)
	if s := strings.TrimSpace(string(b)); s != "" {
		return s
	}
	return "Playing"
}

// ----- the player process -----

// Run is the player process: play tracks until told to stop.
func Run(cfg *config.Config) {
	_ = os.Remove(CommandPipe)
	if err := syscall.Mkfifo(CommandPipe, 0666); err != nil {
		return
	}
	pipe, err := os.OpenFile(CommandPipe, os.O_RDWR, 0)
	if err != nil {
		return
	}
	_ = os.WriteFile(PidFile, []byte(fmt.Sprint(os.Getpid())), 0644)
	defer func() {
		_ = os.Remove(CommandPipe)
		_ = os.Remove(PidFile)
		_ = os.Remove(StatusFile)
	}()

	cmds := make(chan string, 4)
	go func() {
		sc := bufio.NewScanner(pipe)
		for sc.Scan() {
			cmds <- strings.ToLower(strings.TrimSpace(sc.Text()))
		}
	}()

	status := func(s string) { _ = os.WriteFile(StatusFile, []byte(s+"\n"), 0644) }
	order := 0
	for {
		// Settings can change while it runs (menu Options).
		if c, err := config.Load(); err == nil {
			cfg = c
		}
		tracks := Tracks(cfg.Music.Playlist)
		if len(tracks) == 0 {
			status("No music found in " + filepath.Join(Folder, cfg.Music.Playlist))
			select {
			case c := <-cmds:
				if c == "stop" {
					return
				}
			case <-time.After(10 * time.Second):
			}
			continue
		}
		var track string
		if strings.EqualFold(cfg.Music.Playback, "In order") {
			track = tracks[order%len(tracks)]
			order++
		} else {
			track = tracks[rand.Intn(len(tracks))]
		}
		if stop := play(cfg, track, cmds, status); stop {
			return
		}
	}
}

// play plays one track; it reports true when told to stop.
func play(cfg *config.Config, track string, cmds <-chan string, status func(string)) bool {
	var c *exec.Cmd
	if strings.HasSuffix(strings.ToLower(track), ".ogg") {
		c = exec.Command("ogg123", "-q", track)
	} else {
		c = exec.Command("mpg123", "-q", "--no-control", track)
	}
	if err := c.Start(); err != nil {
		status("Can't play " + filepath.Base(track) + ": " + err.Error())
		time.Sleep(2 * time.Second)
		return false
	}
	name := strings.TrimSuffix(filepath.Base(track), filepath.Ext(track))
	done := make(chan struct{})
	go func() { _ = c.Wait(); close(done) }()

	paused := false
	status("Playing: " + name)
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-done:
			return false
		case cmd := <-cmds:
			switch cmd {
			case "next":
				_ = c.Process.Signal(syscall.SIGCONT)
				_ = c.Process.Kill()
				<-done
				return false
			case "stop":
				_ = c.Process.Signal(syscall.SIGCONT)
				_ = c.Process.Kill()
				<-done
				return true
			}
		case <-tick.C:
			// Pause while a game core is loaded (its own sound plays).
			inGame := cfg.Music.PauseInGames && !menuLoaded()
			if inGame && !paused {
				_ = c.Process.Signal(syscall.SIGSTOP)
				paused = true
				status("Paused while a game plays: " + name)
			} else if !inGame && paused {
				_ = c.Process.Signal(syscall.SIGCONT)
				paused = false
				status("Playing: " + name)
			}
		}
	}
}

// menuLoaded reports whether the MiSTer menu core is loaded.
func menuLoaded() bool {
	b, err := os.ReadFile("/tmp/CORENAME")
	return err != nil || strings.TrimSpace(string(b)) == config.MenuCore
}
