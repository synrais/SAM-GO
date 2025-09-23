package attract

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/utils"
	"golang.org/x/sys/unix"
)

const (
	scalerBaseAddr = 0x20000000
	scalerBufSize  = 2048 * 3 * 1024

	defaultStep = 8
	targetFPS   = 30
)

// Singleton globals
var (
	streamOnce sync.Once
	streamCh   chan StaticEvent
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
	y := 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)

	// black / white cutoffs
	if y < 30 {
		return "Black"
	}
	if y > 220 {
		return "White"
	}

	// detect greys: small difference between channels
	if maxDiff(r, g, b) < 15 {
		return "Grey"
	}

	// else: closest chromatic color
	best := 0
	bestDist := int64(1<<63 - 1)
	for i, c := range colors {
		if c.Name == "Black" || c.Name == "White" {
			continue
		}
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

func maxDiff(r, g, b int) int {
	max := r
	if g > max {
		max = g
	}
	if b > max {
		max = b
	}
	min := r
	if g < min {
		min = g
	}
	if b < min {
		min = b
	}
	return max - min
}

func rgbToHex(r, g, b int) string {
	return fmt.Sprintf("#%02X%02X%02X", r&0xFF, g&0xFF, b&0xFF)
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

// ---- List helpers ----
func isEntryInFile(path, game string) bool {
	normName, _ := utils.NormalizeEntry(game)
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		name, _ := utils.NormalizeEntry(scanner.Text())
		if name == normName {
			return true
		}
	}
	return false
}

func addToFile(system, game, suffix string) {
	dir := config.FilterlistDir()
	_ = os.MkdirAll(dir, 0777)
	path := filepath.Join(dir, system+suffix)

	name, _ := utils.NormalizeEntry(game)
	entry := name

	if strings.Contains(suffix, "staticlist") {
		if strings.HasPrefix(game, "<") {
			if idx := strings.Index(game, ">"); idx > 1 {
				ts := game[:idx+1]
				entry = ts + name
			}
		}
	}

	if isEntryInFile(path, entry) {
		return
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return
	}
	defer f.Close()

	fmt.Fprintf(f, "%s\n", entry)
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
	DetectorSkip bool
}

func (e StaticEvent) String() string {
	return fmt.Sprintf("Uptime=%7.1f | Frames=%7d | StaticScreen=%7.1fs | "+
		"StuckPixels=%d/%d | Resolution=%4dx%-4d | "+
		"DominantRGB= %s %-7s | AverageRGB= %s %-7s | Game= %s",
		e.Uptime, e.Frames, e.StaticScreen,
		e.StuckPixels, e.Samples,
		e.Width, e.Height,
		e.DominantHex, e.DominantName,
		e.AverageHex, e.AverageName,
		e.Game)
}

// Stream launches the static screen detector and streams events (singleton).
func Stream(cfg *config.UserConfig, r *rand.Rand) <-chan StaticEvent {
	streamOnce.Do(func() {
		streamCh = make(chan StaticEvent, 1)

		baseCfg := cfg.StaticDetector
		overrides := cfg.StaticDetector.Systems

		go func() {
			defer close(streamCh)

			_ = os.MkdirAll("/tmp/.SAM_tmp", 0777)

			res, err := newResolution()
			if err != nil {
				fmt.Println("resolution init:", err)
				return
			}
			defer res.Close()

			staticScreenRun := 0.0
			staticStartTime := 0.0
			sampleFrames := 0
			lastFrameTime := time.Now()
			firstFrame := true

			lastGame := ""
			handledBlack := false
			handledStatic := false
			currCfg := baseCfg

			titleStartTime := time.Now()

			resetState := func(game string, sysId string) {
				lastGame = game
				staticScreenRun = 0
				staticStartTime = 0
				sampleFrames = 0
				firstFrame = true
				lastFrameTime = time.Now()
				handledBlack = false
				handledStatic = false
				titleStartTime = time.Now()

				currCfg = baseCfg
				sysName := strings.ToLower(sysId)
				if ov, ok := overrides[sysName]; ok {
					if ov.BlackThreshold != nil {
						currCfg.BlackThreshold = *ov.BlackThreshold
					}
					if ov.StaticThreshold != nil {
						currCfg.StaticThreshold = *ov.StaticThreshold
					}
					if ov.SkipBlack != nil {
						currCfg.SkipBlack = *ov.SkipBlack
					}
					if ov.WriteBlackList != nil {
						currCfg.WriteBlackList = *ov.WriteBlackList
					}
					if ov.SkipStatic != nil {
						currCfg.SkipStatic = *ov.SkipStatic
					}
					if ov.WriteStaticList != nil {
						currCfg.WriteStaticList = *ov.WriteStaticList
					}
					if ov.Grace != nil {
						currCfg.Grace = *ov.Grace
					}
				}
			}

			prevRGB := make([]uint32, 0, (2048/defaultStep)*(2048/defaultStep))
			currRGB := make([]uint32, 0, (2048/defaultStep)*(2048/defaultStep))

			for {
				t1 := time.Now()

				system, name := getLastPlayed()
				displayGame := fmt.Sprintf("[%s] %s", system.Id, name)
				cleanGame, _ := utils.NormalizeEntry(name)

				if displayGame != lastGame {
					resetState(displayGame, system.Id)
					continue
				}

				res.Header = int(res.Map[2])<<8 | int(res.Map[3])
				res.Width = int(res.Map[6])<<8 | int(res.Map[7])
				res.Height = int(res.Map[8])<<8 | int(res.Map[9])
				res.Line = int(res.Map[10])<<8 | int(res.Map[11])

				valid := !(res.Width < 64 || res.Width > 2048 ||
					res.Height < 64 || res.Height > 2048 ||
					res.Line < res.Width*3 || res.Line > 2048*4)

				var sumR, sumG, sumB int
				currRGB = currRGB[:0]

				if !valid {
					currRGB = append(currRGB, 0)
				} else {
					for y := 0; y < res.Height; y += defaultStep {
						row := res.Map[res.Header+y*res.Line:]
						for x := 0; x < res.Width; x += defaultStep {
							off := x * 3
							if off+2 < res.Line {
								r := row[off]
								g := row[off+1]
								b := row[off+2]
								currRGB = append(currRGB, uint32(r)<<16|uint32(g)<<8|uint32(b))
								sumR += int(r)
								sumG += int(g)
								sumB += int(b)
							}
						}
					}
				}

				if len(currRGB) == 0 {
					time.Sleep(time.Second / targetFPS)
					continue
				}
				sampleFrames++

				avgR := sumR / len(currRGB)
				avgG := sumG / len(currRGB)
				avgB := sumB / len(currRGB)

				sort.Slice(currRGB, func(i, j int) bool { return currRGB[i] < currRGB[j] })
				bestCount := 0
				currCount := 1
				bestVal := currRGB[0]
				for i := 1; i <= len(currRGB); i++ {
					if i < len(currRGB) && currRGB[i] == currRGB[i-1] {
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

				frameTime := time.Now()
				stuckPixels := 0
				if !firstFrame {
					changed := false
					minLen := len(prevRGB)
					if len(currRGB) < minLen {
						minLen = len(currRGB)
					}
					for i := 0; i < minLen; i++ {
						if currRGB[i] != prevRGB[i] {
							changed = true
						} else {
							stuckPixels++
						}
					}
					if !changed {
						if staticScreenRun == 0 {
							staticStartTime = frameTime.Sub(titleStartTime).Seconds()
						}
						staticScreenRun += frameTime.Sub(lastFrameTime).Seconds()
					} else {
						staticScreenRun = 0
						staticStartTime = 0
						handledStatic = false
						handledBlack = false
					}
				}
				prevRGB = append(prevRGB[:0], currRGB...)
				firstFrame = false
				lastFrameTime = frameTime

				uptime := frameTime.Sub(titleStartTime).Seconds()
				avgHex := rgbToHex(avgR, avgG, avgB)

				event := StaticEvent{
					Uptime:       uptime,
					Frames:       sampleFrames,
					StaticScreen: staticScreenRun,
					StuckPixels:  stuckPixels,
					Samples:      len(currRGB),
					Width:        res.Width,
					Height:       res.Height,
					DominantHex:  rgbToHex(domR, domG, domB),
					DominantName: nearestColorName(domR, domG, domB),
					AverageHex:   avgHex,
					AverageName:  nearestColorName(avgR, avgG, avgB),
					Game:         displayGame,
					DetectorSkip: false,
				}

				if uptime > currCfg.Grace {
					if avgHex == "#000000" &&
						staticScreenRun > currCfg.BlackThreshold &&
						!handledBlack {
						if currCfg.WriteBlackList {
							addToFile(system.Id, cleanGame, "_blacklist.txt")
						}
						if currCfg.SkipBlack {
							event.DetectorSkip = true
						}
						handledBlack = true
					}
					if avgHex != "#000000" &&
						staticScreenRun > currCfg.StaticThreshold &&
						!handledStatic {
						if currCfg.WriteStaticList {
							entry := fmt.Sprintf("<%.0f> %s", staticStartTime, cleanGame)
							addToFile(system.Id, entry, "_staticlist.txt")
						}
						if currCfg.SkipStatic {
							event.DetectorSkip = true
						}
						handledStatic = true
					}
				}

				streamCh <- event

				elapsed := time.Since(t1)
				frameDur := time.Second / targetFPS
				if elapsed < frameDur {
					time.Sleep(frameDur - elapsed)
				}
			}
		}()
	})

	return streamCh
}
