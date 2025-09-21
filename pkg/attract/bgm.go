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
	"time"

	"gopkg.in/ini.v1"
)

const (
	MUSIC_FOLDER  = "/media/fat/music"
	INI_FILE      = MUSIC_FOLDER + "/bgm.ini"
	LOG_FILE      = "/tmp/bgm.log"
	MIDI_PORT     = "128:0"
	HISTORY_RATIO = 0.2
)

type Config struct {
	Playback      string
	Playlist      *string
	Startup       bool
	PlayInCore    bool
	CoreBootDelay float64
	MenuVolume    int
	DefaultVolume int
	Debug         bool
}

// ---------------- Config ----------------

func writeDefaultIni() {
	os.MkdirAll(MUSIC_FOLDER, 0755)
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
	playlist := section.Key("playlist").MustString("none")
	if playlist == "none" {
		playlist = ""
	}
	startup := section.Key("startup").MustBool(true)
	playincore := section.Key("playincore").MustBool(false)
	corebootdelay := section.Key("corebootdelay").MustFloat64(0)
	menuvol := section.Key("menuvolume").MustInt(-1)
	defvol := section.Key("defaultvolume").MustInt(-1)
	debug := section.Key("debug").MustBool(false)

	var playlistPtr *string
	if playlist != "" {
		playlistPtr = &playlist
	}

	return Config{
		Playback:      playback,
		Playlist:      playlistPtr,
		Startup:       startup,
		PlayInCore:    playincore,
		CoreBootDelay: corebootdelay,
		MenuVolume:    menuvol,
		DefaultVolume: defvol,
		Debug:         debug,
	}
}

// ---------------- Logging ----------------

func logMsg(msg string) {
	if msg == "" {
		return
	}
	fmt.Println(msg)
	f, _ := os.OpenFile(LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer f.Close()
	f.WriteString(fmt.Sprintf("[%s] %s\n", time.Now().Format(time.RFC3339), msg))
}

// ---------------- Volume ----------------

func misterVolume(level int) {
	if level < 0 {
		level = 0
	}
	if level > 7 {
		level = 7
	}
	_ = os.WriteFile("/dev/MiSTer_cmd", []byte(fmt.Sprintf("volume %d\n", level)), 0644)
}

func misterFadeOut(from, to, steps int, delay time.Duration) {
	if steps < 1 {
		steps = 1
	}
	stepSize := float64(from-to) / float64(steps)
	vol := float64(from)
	for i := 0; i < steps; i++ {
		misterVolume(int(math.Round(vol)))
		time.Sleep(delay)
		vol -= stepSize
	}
	misterVolume(to)
}

// ---------------- Player ----------------

type Player struct {
	History  []string
	Playing  string
	Playlist *string
	Playback string

	mu       sync.Mutex
	cmd      *exec.Cmd
	stopLoop chan struct{}
}

func isValidFile(name string) bool {
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

func getTracks(playlist *string) []string {
	var base string
	if playlist == nil || *playlist == "" {
		base = MUSIC_FOLDER
	} else if *playlist == "all" {
		base = MUSIC_FOLDER
	} else {
		base = filepath.Join(MUSIC_FOLDER, *playlist)
	}
	var tracks []string
	filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && isValidFile(info.Name()) && !strings.HasPrefix(info.Name(), "_") {
			tracks = append(tracks, path)
		}
		return nil
	})
	return tracks
}

func (p *Player) getRandomTrack() string {
	tracks := getTracks(p.Playlist)
	if len(tracks) == 0 {
		return ""
	}
	for {
		track := tracks[rand.Intn(len(tracks))]
		found := false
		for _, h := range p.History {
			if h == track {
				found = true
				break
			}
		}
		if !found {
			return track
		}
	}
}

func (p *Player) playFile(track string) {
	lower := strings.ToLower(track)
	var c *exec.Cmd
	switch {
	case strings.HasSuffix(lower, ".mp3"), strings.HasSuffix(lower, ".pls"):
		c = exec.Command("mpg123", "--no-control", track)
	case strings.HasSuffix(lower, ".ogg"):
		c = exec.Command("ogg123", track)
	case strings.HasSuffix(lower, ".wav"):
		c = exec.Command("aplay", track)
	case strings.HasSuffix(lower, ".mid"):
		c = exec.Command("aplaymidi", "--port="+MIDI_PORT, track)
	default:
		c = exec.Command("vgmplay", track)
	}

	p.mu.Lock()
	p.cmd = c
	p.mu.Unlock()

	stdout, _ := c.StdoutPipe()
	c.Stderr = c.Stdout
	_ = c.Start()

	scanner := bufio.NewScanner(stdout)
	go func() {
		for scanner.Scan() {
			logMsg(scanner.Text())
		}
	}()

	// This blocks until the process exits
	_ = c.Wait()

	p.mu.Lock()
	if p.cmd == c {
		p.cmd = nil
	}
	p.mu.Unlock()
}

func (p *Player) play(track string) {
	if !isValidFile(track) {
		return
	}
	p.Playing = track
	logMsg("Now playing: " + track)
	p.playFile(track)
	p.Playing = ""
}

func (p *Player) StartLoop() {
	p.mu.Lock()
	if p.stopLoop != nil {
		p.mu.Unlock()
		return // already running
	}
	p.stopLoop = make(chan struct{})
	p.mu.Unlock()

	go func() {
		for {
			select {
			case <-p.stopLoop:
				return
			default:
				track := p.getRandomTrack()
				if track == "" {
					time.Sleep(time.Second)
					continue
				}
				p.play(track) // blocks until finished or killed
			}
		}
	}()
}

func (p *Player) StopLoop() {
	p.mu.Lock()
	if p.stopLoop != nil {
		close(p.stopLoop)
		p.stopLoop = nil
	}
	if p.cmd != nil && p.cmd.Process != nil {
		// fade out before kill
		misterFadeOut(7, 0, 8, 100*time.Millisecond)
		_ = p.cmd.Process.Kill()
		p.cmd = nil
	}
	p.mu.Unlock()
	misterVolume(7) // restore max
}
