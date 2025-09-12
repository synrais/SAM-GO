package staticdetector

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	scalerBaseAddr = 0x20000000
	scalerBufSize  = 2048 * 3 * 1024

	defaultStep = 8
	targetFPS   = 30

	activityFile = "/tmp/.SAM_tmp/SAM_Screen_Activity"
	logFile      = "/tmp/.SAM_tmp/SAM_Screen_Activity.log"
	coreNameFile = "/tmp/CORENAME"
	gameNameFile = "/tmp/Now_Playing.txt"
	logMaxLines  = 50000

	listDir         = "/media/fat/Scripts/.MiSTer_SAM/SAM_Gamelists"
	blackThreshold  = 30.0
	staticThreshold = 30.0
)

// NamedColor represents a well known color and its RGB components.
type NamedColor struct {
	Name    string
	R, G, B int
}

var colors = []NamedColor{
	{"Black", 0, 0, 0},
	{"White", 255, 255, 255},
	{"Red", 255, 0, 0},
	{"Green", 0, 255, 0},
	{"Blue", 0, 0, 255},
	{"Magenta", 255, 0, 255},
	{"Cyan", 0, 255, 255},
	{"Yellow", 255, 255, 0},
}

func nearestColorName(r, g, b int) string {
	best := 0
	bestDist := int64(1<<63 - 1)
	for i, c := range colors {
		dr := int64(r - c.R)
		dg := int64(g - c.G)
		db := int64(b - c.B)
		d := dr*dr + dg*dg + db*db
		if d < bestDist {
			bestDist = d
			best = i
		}
	}
	return colors[best].Name
}

func rgbToHex(r, g, b int) string {
	return fmt.Sprintf("#%02X%02X%02X", r&0xFF, g&0xFF, b&0xFF)
}

func readTmpTxt(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return "Unknown"
	}
	s := strings.TrimSpace(string(b))
	if s == "" {
		return "Unknown"
	}
	return s
}

type resolution struct {
	Header int
	Width  int
	Height int
	Line   int
	Map    []byte
}

func newResolution() (*resolution, error) {
	fd, err := unix.Open("/dev/mem", unix.O_RDONLY|unix.O_SYNC, 0)
	if err != nil {
		return nil, err
	}
	data, err := unix.Mmap(fd, scalerBaseAddr, scalerBufSize, unix.PROT_READ, unix.MAP_SHARED)
	_ = unix.Close(fd)
	if err != nil {
		return nil, err
	}
	return &resolution{Map: data}, nil
}

func (r *resolution) Close() {
	if r.Map != nil {
		_ = unix.Munmap(r.Map)
		r.Map = nil
	}
}

func nowSeconds() float64 {
	return float64(time.Now().UnixNano()) / 1e9
}

// List helpers

func extractSystem(game string) string {
	open := strings.LastIndex(game, "(")
	close := strings.LastIndex(game, ")")
	var system string
	if open >= 0 && close > open {
		system = game[open+1 : close]
	} else {
		system = "unknown"
	}
	system = strings.ReplaceAll(system, " ", "_")
	system = strings.ToLower(system)
	return system
}

func stripSystemFromGame(game string) string {
	clean := game
	if lp := strings.LastIndex(clean, "("); lp >= 0 {
		if rp := strings.LastIndex(clean, ")"); rp >= 0 && rp > lp {
			if lp > 0 {
				clean = strings.TrimSpace(clean[:lp-1])
			}
		}
	}
	return clean
}

func isEntryInFile(path, game string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if scanner.Text() == game {
			return true
		}
	}
	return false
}

func addToFile(system, game, suffix string) {
	_ = os.MkdirAll(listDir, 0777)
	path := filepath.Join(listDir, system+suffix)
	if isEntryInFile(path, game) {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s\n", game)
	fmt.Printf("\n[LIST] Added \"%s\" to %s%s\n", game, system, suffix)
}

// StaticEvent describes a snapshot of the static detector state.
type StaticEvent struct {
	Uptime       float64
	Frames       int
	StaticScreen float64
	StuckPixels  int
	Samples      int
	Width        int
	Height       int
	DominantHex  string
	DominantName string
	AverageHex   string
	AverageName  string
	Game         string
}

func (e StaticEvent) String() string {
	return fmt.Sprintf("Uptime=%7.1f | Frames=%7d | StaticScreen=%7.1fs | "+
		"StuckPixels=%5d/%-5d | Resolution=%4dx%-4d | "+
		"DominantRGB= %s %-7s | AverageRGB= %s %-7s | Game= %s",
		e.Uptime, e.Frames, e.StaticScreen,
		e.StuckPixels, e.Samples,
		e.Width, e.Height,
		e.DominantHex, e.DominantName,
		e.AverageHex, e.AverageName,
		e.Game)
}

// Stream launches the static screen detector and streams events.
func Stream() <-chan StaticEvent {
	out := make(chan StaticEvent, 1)

	go func() {
		defer close(out)

		_ = os.MkdirAll("/tmp/.SAM_tmp", 0777)

		res, err := newResolution()
		if err != nil {
			fmt.Println("resolution init:", err)
			return
		}
		defer res.Close()

		inFd, err := unix.InotifyInit1(unix.IN_NONBLOCK)
		if err != nil {
			fmt.Println("inotify init:", err)
			return
		}
		defer unix.Close(inFd)
		_, _ = unix.InotifyAddWatch(inFd, "/tmp", unix.IN_CREATE|unix.IN_MODIFY|unix.IN_ATTRIB)

		game := readTmpTxt(gameNameFile)

		uptimeStart := time.Now()
		staticScreenRun := 0.0
		staticStartTime := 0.0
		sampleFrames := 0
		lastFrameTime := time.Now()
		seenChange := false
		firstFrame := true

		maxSamples := (2048 / defaultStep) * (2048 / defaultStep)
		prevRGB := make([]uint32, maxSamples)
		currRGB := make([]uint32, maxSamples)

		logf, err := os.Create(logFile)
		if err != nil {
			logf = nil
		}
		if logf != nil {
			defer logf.Close()
		}
		logLines := 0

		alreadyBlacklisted := false
		alreadyStaticlisted := false

		for {
			t1 := time.Now()

			res.Header = int(res.Map[2])<<8 | int(res.Map[3])
			res.Width  = int(res.Map[6])<<8 | int(res.Map[7])
			res.Height = int(res.Map[8])<<8 | int(res.Map[9])
			res.Line   = int(res.Map[10])<<8 | int(res.Map[11])

			// Sanity check resolution before using it
			const (
				minWidth  = 128
				minHeight = 128
				maxWidth  = 2048
				maxHeight = 2048
			)
			if res.Width < minWidth || res.Width > maxWidth ||
			   res.Height < minHeight || res.Height > maxHeight ||
			   res.Line < res.Width*3 || res.Line > maxWidth*4 {
				// Skip this frame – prevents out-of-bounds crashes
				fmt.Printf("Invalid resolution skipped: %dx%d (line=%d)\n", res.Width, res.Height, res.Line)
				time.Sleep(time.Second / targetFPS)
				continue
			}

			buf := make([]byte, 4096)
			n, err := unix.Read(inFd, buf)
			if err == nil && n > 0 {
				offset := 0
				for offset < n {
					ev := (*unix.InotifyEvent)(unsafe.Pointer(&buf[offset]))
					nameBytes := buf[offset+unix.SizeofInotifyEvent : offset+unix.SizeofInotifyEvent+int(ev.Len)]
					name := strings.TrimRight(string(nameBytes), "\x00")
					if name == "CORENAME" {
						uptimeStart = time.Now()
						staticScreenRun = 0
						sampleFrames = 0
						seenChange = false
						lastFrameTime = time.Now()
						firstFrame = true
						alreadyBlacklisted = false
						alreadyStaticlisted = false
					} else if name == "Now_Playing.txt" {
						game = readTmpTxt(gameNameFile)
					}
					offset += unix.SizeofInotifyEvent + int(ev.Len)
				}
			}


			idx := 0
			var sumR, sumG, sumB int
			for y := 0; y < res.Height; y += defaultStep {
				row := res.Map[res.Header+y*res.Line:]
				for x := 0; x < res.Width; x += defaultStep {
					off := x * 3
					if off+2 < res.Line {
						r := row[off]
						g := row[off+1]
						b := row[off+2]
						currRGB[idx] = uint32(r)<<16 | uint32(g)<<8 | uint32(b)
						sumR += int(r)
						sumG += int(g)
						sumB += int(b)
						idx++
					}
				}
			}
			samples := idx
			if samples <= 0 {
				continue
			}
			sampleFrames++

			avgR := sumR / samples
			avgG := sumG / samples
			avgB := sumB / samples

			sort.Slice(currRGB[:samples], func(i, j int) bool { return currRGB[i] < currRGB[j] })
			bestCount := 0
			currCount := 1
			bestVal := currRGB[0]
			for i := 1; i <= samples; i++ {
				if i < samples && currRGB[i] == currRGB[i-1] {
					currCount++
				} else {
					if currCount > bestCount {
						bestCount = currCount
						bestVal = currRGB[i-1]
					}
					currCount = 1
				}
			}
			domR := int((bestVal >> 16) & 0xFF)
			domG := int((bestVal >> 8) & 0xFF)
			domB := int(bestVal & 0xFF)

			stuckPixels := 0
			frameTime := time.Now()
			if !firstFrame {
				changed := false
				for i := 0; i < samples; i++ {
					if currRGB[i] != prevRGB[i] {
						changed = true
						break
					}
				}
				if !changed {
					if staticScreenRun == 0 {
						staticStartTime = frameTime.Sub(uptimeStart).Seconds()
					}
					delta := frameTime.Sub(lastFrameTime).Seconds()
				    if delta > 0 {
						staticScreenRun += delta
					}
				} else {
					seenChange = true
					staticScreenRun = 0
				}
				for i := 0; i < samples; i++ {
					if currRGB[i] == prevRGB[i] {
						stuckPixels++
					}
				}
			}
			copy(prevRGB, currRGB[:samples])
			firstFrame = false
			lastFrameTime = frameTime

			uptime := frameTime.Sub(uptimeStart).Seconds()

			domHex := rgbToHex(domR, domG, domB)
			avgHex := rgbToHex(avgR, avgG, avgB)

			domName := nearestColorName(domR, domG, domB)
			avgName := nearestColorName(avgR, avgG, avgB)

			if avgHex == "#000000" && staticScreenRun > blackThreshold && !alreadyBlacklisted {
				system := extractSystem(game)
				clean := stripSystemFromGame(game)
				addToFile(system, clean, "_blacklist.txt")
				alreadyBlacklisted = true
			}
			if avgHex != "#000000" && staticScreenRun > staticThreshold && !alreadyStaticlisted {
				system := extractSystem(game)
				clean := stripSystemFromGame(game)
				entry := fmt.Sprintf("%s | StaticStart=%.1fs", clean, staticStartTime)
				addToFile(system, entry, "_staticlist.txt")
				alreadyStaticlisted = true
			}

			event := StaticEvent{
				Uptime:       uptime,
				Frames:       sampleFrames,
				StaticScreen: staticScreenRun,
				StuckPixels:  stuckPixels,
				Samples:      samples,
				Width:        res.Width,
				Height:       res.Height,
				DominantHex:  domHex,
				DominantName: domName,
				AverageHex:   avgHex,
				AverageName:  avgName,
				Game:         game,
			}
			out <- event

			type jsonRGB struct {
				Hex  string `json:"hex"`
				Name string `json:"name"`
			}
			type jsonStatic struct {
				Uptime       float64 `json:"uptime"`
				Frames       int     `json:"frames"`
				StaticScreen struct {
					Current float64 `json:"current_s"`
				} `json:"static_screen"`
				Resolution  string  `json:"resolution"`
				DominantRGB jsonRGB `json:"dominant_rgb"`
				AverageRGB  jsonRGB `json:"average_rgb"`
				Game        string  `json:"game"`
			}
			j := jsonStatic{
				Uptime:      uptime,
				Frames:      sampleFrames,
				Resolution:  fmt.Sprintf("%dx%d", res.Width, res.Height),
				DominantRGB: jsonRGB{Hex: domHex, Name: domName},
				AverageRGB:  jsonRGB{Hex: avgHex, Name: avgName},
				Game:        game,
			}
			j.StaticScreen.Current = staticScreenRun

			if af, err := os.Create(activityFile); err == nil {
				enc := json.NewEncoder(af)
				enc.SetEscapeHTML(false)
				_ = enc.Encode(j)
				af.Close()
			}

			if logf != nil {
				enc := json.NewEncoder(logf)
				enc.SetEscapeHTML(false)
				_ = enc.Encode(j)
				logLines++
				if logLines >= logMaxLines {
					logf.Close()
					logf, _ = os.Create(logFile)
					logLines = 0
				}
			}

			elapsed := time.Since(t1)
			frameDur := time.Second / targetFPS
			if elapsed < frameDur {
				time.Sleep(frameDur - elapsed)
			}
		}
	}()

	return out
}
