package attract

import (
	"bufio"
	"fmt"
	"math"
	"math/rand"
	"net"
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
	BOOT_FOLDER   = MUSIC_FOLDER + "/boot"
	INI_FILE      = MUSIC_FOLDER + "/bgm.ini"
	LOG_FILE      = "/tmp/bgm.log"
	SOCKET_FILE   = "/tmp/bgm.sock"
	MIDI_PORT     = "128:0"
	HISTORY_RATIO = 0.2
)

var CONFIG_DEFAULTS = map[string]interface{}{
	"playback":      "random",
	"playlist":      nil,
	"startup":       true,
	"playincore":    false,
	"corebootdelay": 0,
	"menuvolume":    -1,
	"defaultvolume": -1,
	"debug":         false,
}

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

// ---------- Config ----------

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

// ---------- Logging ----------

func logMsg(msg string, always bool) {
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

// ---------- MiSTer Volume ----------

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

// ---------- Player ----------

type Player struct {
	History []string
	Playing string

	Playlist *string
	Playback string

	mu       sync.Mutex
	cmd      *exec.Cmd
	stopLoop chan struct{}
	endWG    sync.WaitGroup
}

// valid extensions
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

func getLoopAmount(name string) int {
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

func (p *Player) playFile(cmd ...string) {
	c := exec.Command(cmd[0], cmd[1:]...)
	stdout, _ := c.StdoutPipe()
	c.Stderr = c.Stdout

	p.mu.Lock()
	p.cmd = c
	p.mu.Unlock()

	_ = c.Start()
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		logMsg(scanner.Text(), false)
	}
	c.Wait()

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
	loops := getLoopAmount(track)
	logMsg("Now playing: "+track, true)

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

		// bail early if stop was requested mid-track
		select {
		case <-p.stopLoop:
			return
		default:
		}
	}
	p.Playing = ""
}

func (p *Player) StartLoop() {
	p.mu.Lock()
	if p.stopLoop != nil {
		p.mu.Unlock()
		return
	}
	p.stopLoop = make(chan struct{})
	p.endWG.Add(1)
	p.mu.Unlock()

	go func() {
		defer p.endWG.Done()
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

				// Before starting track, recheck stop
				select {
				case <-p.stopLoop:
					return
				default:
					p.play(track) // blocking until done or killed
				}
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
	cmd := p.cmd
	p.cmd = nil
	p.mu.Unlock()

	// fade out and kill current process
	if cmd != nil && cmd.Process != nil {
		misterFadeOut(7, 0, 8, 100*time.Millisecond)
		_ = cmd.Process.Kill()
		cmd.Wait()
		misterVolume(7)
	}

	p.endWG.Wait()
}

// ---------- Remote socket ----------

func StartRemote(p *Player) {
	if _, err := os.Stat(SOCKET_FILE); err == nil {
		os.Remove(SOCKET_FILE)
	}
	ln, err := net.Listen("unix", SOCKET_FILE)
	if err != nil {
		logMsg(fmt.Sprintf("Socket listen error: %v", err), true)
		return
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				continue
			}
			buf := make([]byte, 1024)
			n, _ := conn.Read(buf)
			cmd := strings.TrimSpace(string(buf[:n]))
			switch cmd {
			case "stop":
				p.StopLoop()
			case "play":
				p.StartLoop()
			case "skip":
				p.StopLoop()
				p.StartLoop()
			}
			conn.Close()
		}
	}()
}
