// Package video plays video files from /media/fat/video (or a playlist
// folder inside it) with MPlayer, on the MiSTer's Linux screen.
//
// Videos need the MiSTer menu core loaded (only it shows the Linux screen).
// The screen is set to 640x480 for playback and MiSTer's scaler fills the
// TV with it. Everything is decoded by the MiSTer's ARM processor, so SD
// video (around 480p) plays well; 720p and up, or HEVC, is too heavy.
//
// While a video plays, its PID is kept in PidFile so the music player can
// pause.
package video

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/synrais/SAMenu/pkg/assets"
	"github.com/synrais/SAMenu/pkg/config"
	"github.com/synrais/SAMenu/pkg/mister"
)

const (
	Folder    = "/media/fat/video"
	PidFile   = "/tmp/SAMenu_video.pid" // MPlayer's PID while a video plays
	playerDir = "/tmp/SAMenu_mplayer"

	// The background player ("SAMenu -videod"), for -video commands.
	CommandPipe   = "/tmp/SAMenu_video.cmd"
	StatusFile    = "/tmp/SAMenu_video.status"
	DaemonPidFile = "/tmp/SAMenu_videod.pid"

	// Screen size for playback. 480p video plays at its own size (no
	// scaling work for the processor) and the TV output is upscaled.
	screenW, screenH = 640, 480
)

// extensions MPlayer can play here.
var extensions = []string{
	".mp4", ".m4v", ".mov", ".mkv", ".webm", ".avi", ".mpg", ".mpeg",
	".ts", ".wmv", ".asf", ".ogv", ".flv",
}

// IsVideo reports whether a file name has a video extension.
func IsVideo(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	for _, e := range extensions {
		if ext == e {
			return true
		}
	}
	return false
}

// Playlists lists the folders inside the video folder (each is a
// playlist); "" is the video folder itself, "All" everything.
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

// Videos lists the video files for a playlist, sorted.
func Videos(playlist string) []string {
	var files []string
	if strings.EqualFold(playlist, "All") {
		_ = filepath.WalkDir(Folder, func(p string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() && IsVideo(p) {
				files = append(files, p)
			}
			return nil
		})
	} else {
		files = InFolder(filepath.Join(Folder, playlist))
	}
	sort.Strings(files)
	return files
}

// InFolder lists the video files directly inside a folder, sorted.
func InFolder(dir string) []string {
	var files []string
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !e.IsDir() && IsVideo(e.Name()) {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(files)
	return files
}

// Playing reports whether a video is playing right now.
func Playing() bool {
	b, err := os.ReadFile(PidFile)
	if err != nil {
		return false
	}
	c, err := os.ReadFile("/proc/" + strings.TrimSpace(string(b)) + "/cmdline")
	return err == nil && strings.Contains(string(c), "mplayer")
}

// ----- playing -----

// player unpacks MPlayer (once per boot) and returns its path.
func player() (string, error) {
	bin := filepath.Join(playerDir, "mplayer")
	if info, err := os.Stat(bin); err == nil && info.Size() > 0 {
		return bin, nil
	}
	if err := os.MkdirAll(playerDir, 0755); err != nil {
		return "", err
	}
	if err := assets.ExtractZip(assets.MPlayerZip, playerDir); err != nil {
		return "", fmt.Errorf("unpacking MPlayer: %w", err)
	}
	return bin, nil
}

// command builds the MPlayer command for some files. With keys, MPlayer
// reads its keyboard controls from the console (a controller works too,
// as MiSTer turns its buttons into keys).
func command(files []string, shuffle, keys bool) (*exec.Cmd, error) {
	bin, err := player()
	if err != nil {
		return nil, err
	}
	args := []string{"-n", "-20", bin,
		"-really-quiet",
		"-vo", "fbdev2",
		"-zoom", "-xy", strconv.Itoa(screenW), // fit the width, keep the shape
		"-framedrop", // drop frames rather than fall behind the sound
	}
	if keys {
		if conf, err := writeKeys(); err == nil {
			args = append(args, "-input", "conf="+conf)
		}
	}
	args = append(args, syncArgs(true)...)
	if !keys {
		args = append(args, "-noconsolecontrols")
	}
	if shuffle {
		args = append(args, "-shuffle")
	}
	args = append(args, files...)
	cmd := exec.Command("nice", args...)
	cmd.Env = append(os.Environ(), "LD_LIBRARY_PATH="+playerDir)
	return cmd, nil
}

// toConsole writes to the MiSTer's own screen: stdout when this program is
// on it (SAMenu), otherwise the console device (attract mode in the
// background, SSH), so escape codes never end up in a log or SSH window.
func toConsole(s string) {
	if mister.OnConsole() {
		fmt.Print(s)
		return
	}
	if f, err := os.OpenFile("/dev/tty1", os.O_WRONLY, 0); err == nil {
		_, _ = f.WriteString(s)
		f.Close()
	}
}

// prepareScreen sets the screen up for a video (640x480, cleared, no
// cursor) and returns a function that puts the previous size back.
func prepareScreen() (restore func()) {
	prevW, prevH := mister.FbSize()
	_ = mister.SetFbSize(screenW, screenH)
	toConsole("\033[2J\033[H\033[?25l")
	return func() {
		toConsole("\033[2J\033[H\033[?25h")
		if prevW > 0 && prevH > 0 && (prevW != screenW || prevH != screenH) {
			_ = mister.SetFbSize(prevW, prevH)
		}
	}
}

func markPlaying(pid int) { _ = os.WriteFile(PidFile, []byte(strconv.Itoa(pid)), 0644) }
func markStopped()        { _ = os.Remove(PidFile) }

// PlayOnScreen plays files one after another in the foreground, from a
// program on the MiSTer's own screen (SAMenu), and returns when
// they're done or the user quits. MPlayer's keys work while it plays; with
// a controller: A = next video, B = stop, X = pause, d-pad left/right =
// back/forward 10 seconds, up/down = 1 minute.
//
// The screen's previous size is put back afterwards.
func PlayOnScreen(files []string, shuffle bool) error {
	if len(files) == 0 {
		return fmt.Errorf("no videos to play")
	}
	cmd, err := command(files, shuffle, true)
	if err != nil {
		return err
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, quietStderr()

	restore := prepareScreen()
	defer restore()
	if err := cmd.Start(); err != nil {
		return err
	}
	markPlaying(cmd.Process.Pid)
	defer markStopped()
	_ = cmd.Wait() // quitting early isn't an error
	return nil
}

// PlayStream plays a video piped into this program's input (e.g. from a
// PC: cat movie.mkv | ssh root@mister "SAMenu.sh -video play -"), in
// the foreground, until it ends or is stopped (-video stop). Streams can't
// seek, so MP4s need their index at the start ("faststart").
func PlayStream() error {
	bin, err := player()
	if err != nil {
		return err
	}
	cmd := exec.Command("nice", "-n", "-20", bin,
		"-really-quiet", "-vo", "fbdev2", "-zoom", "-xy", strconv.Itoa(screenW), "-framedrop",
		"-noconsolecontrols",
		"-cache", "8192", "-cache-min", "10", // buffer the network a little
	)
	cmd.Args = append(cmd.Args, syncArgs(false)...) // a stream can't be indexed
	cmd.Args = append(cmd.Args, "-")
	cmd.Env = append(os.Environ(), "LD_LIBRARY_PATH="+playerDir)
	cmd.Stdin = os.Stdin
	cmd.Stderr = quietStderr()

	EnsureMenuCore()
	restore := prepareScreen()
	defer restore()
	if err := cmd.Start(); err != nil {
		return err
	}
	markPlaying(cmd.Process.Pid)
	defer markStopped()
	_ = cmd.Wait()
	return nil
}

// Background is a video playing without keys (attract mode).
type Background struct {
	cmd  *exec.Cmd
	Done <-chan struct{} // closed when the video ends
}

// StartBackground plays one video with no console controls; attract mode
// stops it with Stop.
func StartBackground(file string) (*Background, error) {
	cmd, err := command([]string{file}, false, false)
	if err != nil {
		return nil, err
	}
	devnull, _ := os.Open(os.DevNull)
	cmd.Stdin = devnull

	restore := prepareScreen()
	if err := cmd.Start(); err != nil {
		restore()
		return nil, err
	}
	markPlaying(cmd.Process.Pid)
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		markStopped()
		restore()
		if devnull != nil {
			devnull.Close()
		}
		close(done)
	}()
	return &Background{cmd: cmd, Done: done}, nil
}

// ----- the background player (-video commands) -----

// EnsureMenuCore loads the MiSTer menu core if a game core is running, as
// only the menu core shows the Linux screen, and waits for it.
func EnsureMenuCore() {
	if menuCoreLoaded() {
		return
	}
	_ = mister.LaunchMenu()
	for i := 0; i < 50 && !menuCoreLoaded(); i++ {
		time.Sleep(100 * time.Millisecond)
	}
	time.Sleep(time.Second) // let the menu core settle
}

func menuCoreLoaded() bool {
	b, err := os.ReadFile(config.CoreNameFile)
	return err == nil && strings.TrimSpace(string(b)) == config.MenuCore
}

// Running reports whether the background player is running.
func Running() bool {
	b, err := os.ReadFile(DaemonPidFile)
	if err != nil {
		return false
	}
	c, err := os.ReadFile("/proc/" + strings.TrimSpace(string(b)) + "/cmdline")
	return err == nil && strings.Contains(string(c), "-videod")
}

// Send sends a command ("next", "stop") to the background player.
func Send(cmd string) error {
	f, err := os.OpenFile(CommandPipe, os.O_WRONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return fmt.Errorf("no videos are playing")
	}
	defer f.Close()
	_, err = f.WriteString(cmd + "\n")
	return err
}

// Start runs the background player (exe is SAMenu itself) for a target:
// "" for the [Video] playlist, or a file, a folder, or a playlist name.
// A player already running is stopped first.
func Start(exe, target string) error {
	Stop()
	cmd := exec.Command(exe, "-videod", target)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	_ = cmd.Process.Release()
	for i := 0; i < 30 && !Running(); i++ {
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}

// Stop stops whatever video is playing: the background player, or a
// stream or menu playback (by MPlayer's PID).
func Stop() {
	if Send("stop") == nil {
		// It may still be loading the menu core before it notices.
		for i := 0; i < 100 && Running(); i++ {
			time.Sleep(100 * time.Millisecond)
		}
		return
	}
	if b, err := os.ReadFile(PidFile); err == nil && Playing() {
		if pid, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil {
			_ = syscall.Kill(pid, syscall.SIGTERM)
		}
	}
}

// Status says what's playing.
func Status() string {
	if Running() {
		if b, _ := os.ReadFile(StatusFile); strings.TrimSpace(string(b)) != "" {
			return strings.TrimSpace(string(b))
		}
		return "Playing"
	}
	if Playing() {
		return "Playing (from SAMenu, attract mode or a stream)"
	}
	return "Stopped"
}

// Resolve turns a -video target into the videos to play, and whether to
// shuffle them: "" = the [Video] playlist and playback, a file, a folder
// (a path, or a name inside the video folder, "All" for everything).
func Resolve(cfg *config.Config, target string) ([]string, bool, error) {
	shuffle := strings.EqualFold(cfg.Video.Playback, "Random")
	if target == "" {
		vids := Videos(cfg.Video.Playlist)
		if len(vids) == 0 {
			return nil, false, fmt.Errorf("no videos in %s", filepath.Join(Folder, cfg.Video.Playlist))
		}
		return vids, shuffle, nil
	}
	if strings.EqualFold(target, "All") {
		return Videos("All"), shuffle, nil
	}
	for _, p := range []string{target, filepath.Join(Folder, target)} {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			if !IsVideo(p) {
				return nil, false, fmt.Errorf("%s isn't a video file MPlayer plays here", p)
			}
			return []string{p}, false, nil
		}
		vids := InFolder(p)
		if len(vids) == 0 {
			return nil, false, fmt.Errorf("no videos in %s", p)
		}
		return vids, shuffle, nil
	}
	return nil, false, fmt.Errorf("no video file or folder called %q (looked in %s too)", target, Folder)
}

// Run is the background player process: play the target's videos once
// through, then leave the MiSTer on its menu.
func Run(cfg *config.Config, target string) {
	status := func(s string) { _ = os.WriteFile(StatusFile, []byte(s+"\n"), 0644) }
	vids, shuffle, err := Resolve(cfg, target)
	if err != nil {
		return // -video play checks the target before starting this
	}
	if shuffle {
		rand.Seed(time.Now().UnixNano())
		rand.Shuffle(len(vids), func(i, j int) { vids[i], vids[j] = vids[j], vids[i] })
	}

	_ = os.Remove(CommandPipe)
	if err := syscall.Mkfifo(CommandPipe, 0666); err != nil {
		return
	}
	pipe, err := os.OpenFile(CommandPipe, os.O_RDWR, 0)
	if err != nil {
		return
	}
	_ = os.WriteFile(DaemonPidFile, []byte(fmt.Sprint(os.Getpid())), 0644)
	defer func() {
		_ = os.Remove(CommandPipe)
		_ = os.Remove(DaemonPidFile)
		_ = os.Remove(StatusFile)
	}()
	cmds := make(chan string, 4)
	go func() {
		sc := bufio.NewScanner(pipe)
		for sc.Scan() {
			cmds <- strings.ToLower(strings.TrimSpace(sc.Text()))
		}
	}()

	status("Starting...")
	EnsureMenuCore()
	for i, file := range vids {
		// A stop sent while the menu core loaded, or between videos.
		select {
		case c := <-cmds:
			if c == "stop" {
				return
			}
		default:
		}
		status(fmt.Sprintf("Playing %d/%d: %s", i+1, len(vids), filepath.Base(file)))
		bg, err := StartBackground(file)
		if err != nil {
			status("Couldn't play " + filepath.Base(file) + ": " + err.Error())
			time.Sleep(2 * time.Second)
			continue
		}
		select {
		case <-bg.Done:
		case c := <-cmds:
			bg.Stop()
			if c == "stop" {
				return
			}
			// "next" (or anything else): on to the next video
		}
	}
}

// Stop ends a background video and waits for it.
func (b *Background) Stop() {
	select {
	case <-b.Done:
		return
	default:
	}
	_ = b.cmd.Process.Signal(syscall.SIGTERM)
	<-b.Done
}

// syncArgs are the MPlayer sync settings from [Video] (Options -> Video
// Player). files is false for a piped-in stream, which can't be indexed.
func syncArgs(files bool) []string {
	cfg, err := config.Load()
	if err != nil {
		return []string{"-autosync", "30"}
	}
	v := cfg.Video
	var args []string
	if v.AutoSync {
		args = append(args, "-autosync", "30")
	}
	if v.CorrectPts {
		args = append(args, "-correct-pts")
	}
	if v.Mp3Seek {
		args = append(args, "-hr-mp3-seek")
	}
	if v.AviIndex && files {
		args = append(args, "-idx")
	}
	return args
}

// quietStderr passes MPlayer's error output to the screen, minus the Linux
// loader's warnings about MiSTer's ALSA library ("... no version
// information available (required by mplayer)"): harmless, since the
// library works, just without the version tags MPlayer was built with.
func quietStderr() io.Writer { return &lineFilter{out: os.Stderr} }

type lineFilter struct {
	out     io.Writer
	partial []byte
}

func (f *lineFilter) Write(p []byte) (int, error) {
	f.partial = append(f.partial, p...)
	for {
		i := bytes.IndexByte(f.partial, '\n')
		if i < 0 {
			break
		}
		line := f.partial[:i+1]
		if !bytes.Contains(line, []byte("information available")) {
			if _, err := f.out.Write(line); err != nil {
				return len(p), err
			}
		}
		f.partial = f.partial[i+1:]
	}
	return len(p), nil
}

// keyBindings are the player's controls: MPlayer's own defaults, except
// that Enter and Esc both exit. A controller reaches MPlayer as keys
// (MiSTer sends d-pad = arrows, A = Enter, B = Esc, X = Space, L/R = Page
// Up/Down), so on a controller:
//
//	d-pad left/right  back / forward 10 seconds
//	d-pad up/down     forward / back 1 minute
//	L / R             forward / back 10 minutes
//	X (Space)         pause
//	A or B            exit (circle is A or B, depending on the pad)
//
// The list is also in SAMenu.ini's [Video] section.
const keyBindings = `LEFT seek -10
RIGHT seek 10
UP seek 60
DOWN seek -60
PGUP seek 600
PGDWN seek -600
SPACE pause
ENTER quit
ESC quit
q quit
`

// writeKeys writes the key bindings next to the player, for -input conf=.
func writeKeys() (string, error) {
	path := filepath.Join(playerDir, "input.conf")
	if b, err := os.ReadFile(path); err == nil && string(b) == keyBindings {
		return path, nil
	}
	if err := os.MkdirAll(playerDir, 0755); err != nil {
		return "", err
	}
	return path, os.WriteFile(path, []byte(keyBindings), 0644)
}
