package attract

import (
	"bufio"
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"gopkg.in/ini.v1"
)

const (
	MUSIC_FOLDER  = "/media/fat/music"
	BOOT_FOLDER   = MUSIC_FOLDER + "/boot"
	INI_FILE      = MUSIC_FOLDER + "/bgm.ini"
	LOG_FILE      = "/tmp/bgm.log"
	HISTORY_RATIO = 0.2
	MIDI_PORT     = "128:0"
)

type Config struct {
	Playback      string
	Playlist      string
	Startup       bool
	PlayInCore    bool
	CoreBootDelay float64
	MenuVolume    int
	DefaultVolume int
	Debug         bool
}

// --- Config handling ---
func writeDefaultIni() {
	_ = os.MkdirAll(MUSIC_FOLDER, 0755)
	f, _ := os.Create(INI_FILE)
	defer f.Close()
	f.WriteString(`[bgm]
playback = random
playlist = none
startup = yes
playincore = no
corebootdelay = 0
menuvolume = -1
defaultvolume = -1
debug = no
`)
}

func GetConfig() Config {
	if _, err := os.Stat(INI_FILE); os.IsNotExist(err) {
		writeDefaultIni()
	}

	cfg, _ := ini.Load(INI_FILE)
	section := cfg.Section("bgm")

	playback := section.Key("playback").MustString("random")
	playlist := section.Key("playlist").MustString("")
	if playlist == "none" {
		playlist = ""
	}
	startup := section.Key("startup").MustBool(true)
	playincore := section.Key("playincore").MustBool(false)
	corebootdelay := section.Key("corebootdelay").MustFloat64(0)
	menuvol := section.Key("menuvolume").MustInt(-1)
	defvol := section.Key("defaultvolume").MustInt(-1)
	debug := section.Key("debug").MustBool(false)

	return Config{
		Playback:      playback,
		Playlist:      playlist,
		Startup:       startup,
		PlayInCore:    playincore,
		CoreBootDelay: corebootdelay,
		MenuVolume:    menuvol,
		DefaultVolume: defvol,
		Debug:         debug,
	}
}

// --- Logging ---
func LogMsg(msg string, always bool) {
	cfg := GetConfig()
	if msg == "" {
		return
	}
	if always || cfg.Debug {
		fmt.Println(msg)
	}
	if cfg.Debug {
		f, _ := os.OpenFile(LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		defer f.Close()
		f.WriteString(fmt.Sprintf("[%s] %s\n", time.Now().Format(time.RFC3339), msg))
	}
}

// --- File helpers ---
func IsValidFile(name string) bool {
	l := strings.ToLower(name)
	if strings.HasSuffix(l, ".mp3") ||
		strings.HasSuffix(l, ".ogg") ||
		strings.HasSuffix(l, ".wav") ||
		strings.HasSuffix(l, ".mid") ||
		strings.HasSuffix(l, ".pls") {
		return true
	}
	matched, _ := regexp.MatchString(`\.(vgm|vgz|vgm\.gz)$`, l)
	return matched
}

func GetLoopAmount(name string) int {
	base := filepath.Base(name)
	re := regexp.MustCompile(`^X(\d\d)_`)
	match := re.FindStringSubmatch(base)
	if len(match) == 2 {
		var n int
		fmt.Sscanf(match[1], "%d", &n)
		return n
	}
	return 1
}

// --- Player ---
type Player struct {
	mu       sync.Mutex
	History  []string
	Playing  string
	Playlist string
	Playback string
	cmd      *exec.Cmd
	stop     chan struct{}
}

// history management
func (p *Player) addHistory(track string, total int) {
	hsize := int(math.Floor(float64(total) * HISTORY_RATIO))
	if hsize < 1 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.History) >= hsize {
		p.History = p.History[1:]
	}
	p.History = append(p.History, track)
}

// low-level runner
func (p *Player) playFile(cmd ...string) {
	c := exec.Command(cmd[0], cmd[1:]...)
	stdout, _ := c.StdoutPipe()
	c.Stderr = c.Stdout

	p.mu.Lock()
	p.cmd = c
	p.mu.Unlock()

	if err := c.Start(); err != nil {
		LogMsg(fmt.Sprintf("Failed to start player: %v", err), true)
		return
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		LogMsg(scanner.Text(), false)
	}
	_ = c.Wait()

	p.mu.Lock()
	if p.cmd == c {
		p.cmd = nil
	}
	p.mu.Unlock()
}

// play one track fully
func (p *Player) Play(track string) {
	if !IsValidFile(track) {
		return
	}

	// Ensure no other track is playing
	p.StopCurrent()

	p.mu.Lock()
	p.Playing = track
	p.mu.Unlock()

	tracks := GetTracks(p.Playlist)
	p.addHistory(track, len(tracks))

	loops := GetLoopAmount(track)
	LogMsg("Now playing: "+track, true)

	for loops > 0 {
		lower := strings.ToLower(track)
		switch {
		case strings.HasSuffix(lower, ".mp3"), strings.HasSuffix(lower, ".pls"):
			p.playFile("mpg123", "--no-control", track)
		case strings.HasSuffix(lower, ".ogg"):
			p.playFile("ogg123", track)
		case strings.HasSuffix(lower, ".wav"):
			p.playFile("aplay", track)
		case strings.HasSuffix(lower, ".mid"):
			p.playFile("aplaymidi", "--port="+MIDI_PORT, track)
		default:
			p.playFile("vgmplay", track)
		}
		loops--
	}

	p.mu.Lock()
	p.Playing = ""
	p.mu.Unlock()
}

// track picking
func GetTracks(playlist string) []string {
	var base string
	if playlist == "" || playlist == "all" {
		base = MUSIC_FOLDER
	} else {
		base = filepath.Join(MUSIC_FOLDER, playlist)
	}
	var tracks []string
	_ = filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && IsValidFile(info.Name()) && !strings.HasPrefix(info.Name(), "_") {
			tracks = append(tracks, path)
		}
		return nil
	})
	return tracks
}

func (p *Player) GetRandomTrack() string {
	tracks := GetTracks(p.Playlist)
	if len(tracks) == 0 {
		return ""
	}
	for {
		track := tracks[rand.Intn(len(tracks))]
		p.mu.Lock()
		found := false
		for _, h := range p.History {
			if h == track {
				found = true
				break
			}
		}
		p.mu.Unlock()
		if !found {
			return track
		}
	}
}

// --- Loop control ---
func (p *Player) StartLoop() {
	p.mu.Lock()
	if p.stop != nil {
		p.mu.Unlock()
		return // already running
	}
	p.stop = make(chan struct{})
	p.mu.Unlock()

	go func() {
		for {
			select {
			case <-p.stop:
				return
			default:
				track := p.GetRandomTrack()
				if track == "" {
					time.Sleep(time.Second)
					continue
				}

				done := make(chan struct{})
				go func() {
					p.Play(track)
					close(done)
				}()

				select {
				case <-p.stop:
					return
				case <-done:
				}
			}
		}
	}()
}

// Stop only current track, but keep loop logic
func (p *Player) StopCurrent() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cmd != nil && p.cmd.Process != nil {
		_ = p.cmd.Process.Signal(syscall.SIGTERM)
		time.Sleep(200 * time.Millisecond)
		_ = p.cmd.Process.Kill()
		p.cmd = nil
		p.Playing = ""
	}
}

func (p *Player) StopLoop() {
	p.mu.Lock()
	if p.stop != nil {
		close(p.stop)
		p.stop = nil
	}
	p.mu.Unlock()
	p.StopCurrent()
}

// --- New API: PlayNow ---
func (p *Player) PlayNow(track string) {
	// Stop loop if running
	p.StopLoop()

	// Play requested track immediately
	go p.Play(track)
}
