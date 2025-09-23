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

// -----------------------------
// Static Detector
// -----------------------------

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
			frames := 0
			start := time.Now()

			handledBlack := false
			handledStatic := false
			currCfg := baseCfg

			// sample buffers
			maxSamples := (2048 / defaultStep) * (2048 / defaultStep)
			prevRGB := make([]uint32, maxSamples)
			currRGB := make([]uint32, maxSamples)

			for {
				frames++
				uptime := time.Since(start).Seconds()

				// --- capture ---
				width := int(res.Map[12])<<8 | int(res.Map[13])
				height := int(res.Map[14])<<8 | int(res.Map[15])
				line := int(res.Map[16])<<8 | int(res.Map[17])

				if width == 0 || height == 0 || line == 0 {
					time.Sleep(time.Second / targetFPS)
					continue
				}

				// sample pixels
				idx := 0
				step := defaultStep
				var sumR, sumG, sumB int64
				counts := make(map[uint32]int)
				stuck := 0

				for y := 0; y < height; y += step {
					for x := 0; x < width; x += step {
						if idx >= maxSamples {
							break
						}
						offset := y*line + x*3
						if offset+2 >= len(res.Map) {
							continue
						}
						rv := int(res.Map[offset])
						gv := int(res.Map[offset+1])
						bv := int(res.Map[offset+2])
						val := uint32(rv)<<16 | uint32(gv)<<8 | uint32(bv)

						currRGB[idx] = val
						if currRGB[idx] == prevRGB[idx] {
							stuck++
						}
						prevRGB[idx] = val

						sumR += int64(rv)
						sumG += int64(gv)
						sumB += int64(bv)
						counts[val]++
						idx++
					}
				}

				samples := idx
				if samples == 0 {
					time.Sleep(time.Second / targetFPS)
					continue
				}

				// averages
				avgR := int(sumR / int64(samples))
				avgG := int(sumG / int64(samples))
				avgB := int(sumB / int64(samples))
				avgHex := rgbToHex(avgR, avgG, avgB)
				avgName := nearestColorName(avgR, avgG, avgB)

				// dominant
				var domVal uint32
				maxCount := 0
				for k, v := range counts {
					if v > maxCount {
						domVal = k
						maxCount = v
					}
				}
				domR := int((domVal >> 16) & 0xFF)
				domG := int((domVal >> 8) & 0xFF)
				domB := int(domVal & 0xFF)
				dominantHex := rgbToHex(domR, domG, domB)
				dominantName := nearestColorName(domR, domG, domB)

				// static detection
				if stuck > samples/2 {
					if staticScreenRun == 0 {
						staticStartTime = uptime
					}
					staticScreenRun = uptime - staticStartTime
				} else {
					staticScreenRun = 0
					staticStartTime = 0
				}

				cleanGame := filepath.Base(LastPlayedPath)

				// check thresholds
				if uptime > currCfg.Grace {
					// Black screen
					if avgHex == "#000000" && staticScreenRun > currCfg.BlackThreshold && !handledBlack {
						if currCfg.WriteBlackList {
							addToFile(LastPlayedSystem.Id, cleanGame, "_blacklist.txt")
						}
						if currCfg.SkipBlack {
							fmt.Printf("[StaticDetector] Auto-skip (black screen)\n")
							Next(cfg, r) // timer reset handled there
						}
						handledBlack = true
					}

					// Static screen
					if avgHex != "#000000" && staticScreenRun > currCfg.StaticThreshold && !handledStatic {
						if currCfg.WriteStaticList {
							entry := fmt.Sprintf("<%.0f> %s", staticStartTime, cleanGame)
							addToFile(LastPlayedSystem.Id, entry, "_staticlist.txt")
						}
						if currCfg.SkipStatic {
							fmt.Printf("[StaticDetector] Auto-skip (static screen)\n")
							Next(cfg, r) // timer reset handled there
						}
						handledStatic = true
					}
				}

				// emit event
				ev := StaticEvent{
					Uptime:       uptime,
					Frames:       frames,
					StaticScreen: staticScreenRun,
					StuckPixels:  stuck,
					Samples:      samples,
					Width:        width,
					Height:       height,
					DominantHex:  dominantHex,
					DominantName: dominantName,
					AverageHex:   avgHex,
					AverageName:  avgName,
					Game:         cleanGame,
				}

				select {
				case streamCh <- ev:
				default:
				}

				time.Sleep(time.Second / targetFPS)
			}
		}()
	})

	return streamCh
}
