package main

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
	"time"

	"gopkg.in/ini.v1"
)

const (
	MUSIC_FOLDER   = "/media/fat/music"
	BOOT_FOLDER    = MUSIC_FOLDER + "/boot"
	INI_FILE       = MUSIC_FOLDER + "/bgm.ini"
	LOG_FILE       = "/tmp/bgm.log"
	HISTORY_RATIO  = 0.2
	MIDI_PORT      = "128:0"
)

var CONFIG_DEFAULTS = map[string]interface{}{
	"playback":     "random",
	"playlist":     nil,
	"startup":      true,
	"playincore":   false,
	"corebootdelay": 0,
	"menuvolume":   -1,
	"defaultvolume": -1,
	"debug":        false,
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

func getConfig() Config {
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

func logMsg(msg string, always bool) {
	cfg := getConfig()
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

type Player struct {
	History  []string
	Playing  string
	Playlist *string
	Playback string
}

func (p *Player) addHistory(track string, total int) {
	hsize := int(math.Floor(float64(total) * HISTORY_RATIO))
	if hsize < 1 {
		return
	}
	if len(p.History) >= hsize {
		p.History = p.History[1:]
	}
	p.History = append(p.History, track)
}

func (p *Player) playFile(cmd ...string) {
	c := exec.Command(cmd[0], cmd[1:]...)
	stdout, _ := c.StdoutPipe()
	c.Stderr = c.Stdout
	_ = c.Start()

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		logMsg(scanner.Text(), false)
	}
	c.Wait()
}

func (p *Player) Play(track string) {
	if !isValidFile(track) {
		return
	}
	p.Playing = track
	p.addHistory(track, 100) // placeholder for total track count
	loops := getLoopAmount(track)
	logMsg("Now playing: "+track, true)

	for loops > 0 {
		if strings.HasSuffix(strings.ToLower(track), ".mp3") ||
			strings.HasSuffix(strings.ToLower(track), ".pls") {
			p.playFile("mpg123", "--no-control", track)
		} else if strings.HasSuffix(strings.ToLower(track), ".ogg") {
			p.playFile("ogg123", track)
		} else if strings.HasSuffix(strings.ToLower(track), ".wav") {
			p.playFile("aplay", track)
		} else if strings.HasSuffix(strings.ToLower(track), ".mid") {
			p.playFile("aplaymidi", "--port="+MIDI_PORT, track)
		} else {
			p.playFile("vgmplay", track)
		}
		loops--
	}

	p.Playing = ""
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
		// avoid repeats
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

func main() {
	rand.Seed(time.Now().UnixNano())
	cfg := getConfig()

	player := Player{
		Playlist: cfg.Playlist,
		Playback: cfg.Playback,
	}

	tracks := getTracks(player.Playlist)
	if len(tracks) == 0 {
		logMsg("No music files found in "+MUSIC_FOLDER, true)
		return
	}

	// Just play one random track for now
	track := player.getRandomTrack()
	if track != "" {
		player.Play(track)
	}
}
