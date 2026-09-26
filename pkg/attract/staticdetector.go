package attract

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/unix"

	"github.com/synrais/SAMenu/pkg/config"
	"github.com/synrais/SAMenu/pkg/games"
)

// Static screen detector
//
// Watches the MiSTer's video output while attract mode plays a game. After
// a grace period, a screen that stays black too long can put the game on
// its system's Blacklist, and a screen that stays unchanged too long can
// put it on the Staticlist (with the time it went static). Either can also
// tell attract mode to skip to the next game. Settings: [StaticDetector]
// in SAMenu.ini, with [StaticDetector.X] overrides for a system or group.

const (
	scalerBaseAddr = 0x20000000
	scalerBufSize  = 2048 * 3 * 1024

	defaultStep = 8
	targetFPS   = 30
)

// nearestColorName names a colour for the status display. Black, white
// and grey are judged per channel, not by overall brightness, because
// brightness barely counts blue: dark blue (the Amstrad's #00026B) would
// otherwise read as Black, and pure yellow as White. Everything else is
// named by its hue, so dark and washed-out shades still get the right name.
func nearestColorName(r, g, b int) string {
	hi, lo := r, r
	for _, c := range []int{g, b} {
		if c > hi {
			hi = c
		}
		if c < lo {
			lo = c
		}
	}
	switch {
	case hi < 40:
		return "Black"
	case lo > 200:
		return "White"
	case (hi-lo)*5 < hi: // under 20% saturation
		return "Grey"
	}

	// Hue in degrees (0 = red, 120 = green, 240 = blue).
	d := float64(hi - lo)
	var hue float64
	switch hi {
	case r:
		hue = 60 * float64(g-b) / d
	case g:
		hue = 60*float64(b-r)/d + 120
	default:
		hue = 60*float64(r-g)/d + 240
	}
	if hue < 0 {
		hue += 360
	}
	names := []string{"Red", "Yellow", "Green", "Cyan", "Blue", "Magenta"}
	return names[int((hue+30)/60)%6]
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

// Motion tracking
//
// The screen is split into a grid of cells, and each cell remembers when
// it last changed. The screen counts as moving only if enough different
// cells changed recently. How much of the screen changes doesn't matter,
// only how spread out the change is: a flashing cursor or a blinking
// "PRESS START" keeps changing the same few cells, so it counts as static,
// while even a tiny Pac-Man ghost or Pong ball crosses many cells in a
// few seconds, so it counts as moving.
const (
	gridCols = 16
	gridRows = 12
)

type motionTracker struct {
	lastChange [gridCols * gridRows]float64 // seconds; <0 = not yet
}

func (m *motionTracker) reset() {
	for i := range m.lastChange {
		m.lastChange[i] = -1
	}
}

// cellOf returns the grid cell for a pixel on a width x height screen.
func cellOf(x, y, width, height int) uint16 {
	return uint16((y*gridRows/height)*gridCols + x*gridCols/width)
}

func (m *motionTracker) mark(cell uint16, now float64) {
	m.lastChange[cell] = now
}

// spread returns how many cells changed within the last window seconds.
func (m *motionTracker) spread(now, window float64) int {
	n := 0
	for _, t := range m.lastChange {
		if t >= 0 && now-t <= window {
			n++
		}
	}
	return n
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
	// The colours as numbers: turned into text (hex and a name) only when
	// the status is written, not on every frame.
	DomR, DomG, DomB int
	AvgR, AvgG, AvgB int
	System, Title    string
	ChangedPct       float64 // share of sampled pixels that changed, in %
	Spread           int     // cells that changed within SpreadTime
}

// StatusFile always holds the detector's latest state (updated 10 times a
// second, in RAM), so it can be watched from a PC, e.g.
//
//	ssh -t root@mister "watch -n 0.1 cat /tmp/SAMenu_detector"
const StatusFile = config.TempFolder + "/SAMenu_detector"

// statusEvery is how often StatusFile is rewritten.
const statusEvery = 100 * time.Millisecond

// Detector runs the static screen detector in the background.
type Detector struct {
	cfg *config.Config
	res *resolution

	mu         sync.Mutex
	systemID   string
	title      string
	gen        int // bumped on every SetGame
	latest     StaticEvent
	lastAction string // e.g. "Added to SNES blacklist, skipping"
	lastStatus time.Time

	skip chan int // generation of the game to skip

	// viewOnly: just measure and show (SAMenu -watch outside attract
	// mode). Nothing is added to the lists and nothing is skipped.
	viewOnly bool
	stop     chan struct{}
}

// StartDetector maps the video output and starts watching it.
func StartDetector(cfg *config.Config) (*Detector, error) {
	res, err := newResolution()
	if err != nil {
		return nil, fmt.Errorf("static detector: %w", err)
	}
	d := &Detector{cfg: cfg, res: res, skip: make(chan int, 1), stop: make(chan struct{})}
	go d.run()
	return d, nil
}

// StartViewOnlyDetector starts a detector that only measures and shows
// its status, for "SAMenu -watch" when attract mode isn't running:
// outside attract mode nobody says which game is on, so it never adds to
// the lists and never asks for a skip. systemID picks up that system's
// [StaticDetector.X] settings.
func StartViewOnlyDetector(cfg *config.Config, systemID string) (*Detector, error) {
	res, err := newResolution()
	if err != nil {
		return nil, fmt.Errorf("static detector: %w", err)
	}
	d := &Detector{cfg: cfg, res: res, skip: make(chan int, 1), stop: make(chan struct{}), viewOnly: true}
	d.SetGame(systemID, "whatever is playing")
	go d.run()
	return d, nil
}

// Stop ends the detector.
func (d *Detector) Stop() {
	select {
	case <-d.stop:
	default:
		close(d.stop)
	}
}

// SetGame tells the detector which game has just started. It returns the
// game's generation, which Skip reports back.
func (d *Detector) SetGame(systemID, title string) int {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.systemID, d.title = systemID, title
	d.gen++
	return d.gen
}

// Skip delivers the generation of a game the detector wants skipped.
// Compare it with SetGame's result to ignore stale requests.
func (d *Detector) Skip() <-chan int { return d.skip }

// writeStatus refreshes StatusFile, at most every statusEvery.
func (d *Detector) writeStatus(cfg config.StaticDetectorConfig, force bool) {
	d.mu.Lock()
	if !force && time.Since(d.lastStatus) < statusEvery {
		d.mu.Unlock()
		return
	}
	d.lastStatus = time.Now()
	ev, action, title := d.latest, d.lastAction, d.title
	d.mu.Unlock()

	var b strings.Builder
	fmt.Fprintf(&b, "SAMenu static detector - updated %s\n\n", time.Now().Format("15:04:05.0"))
	if title == "" {
		b.WriteString("Waiting for a game...\n")
	} else {
		fmt.Fprintf(&b, "Game:          [%s] %s\n", ev.System, ev.Title)
		fmt.Fprintf(&b, "Playing for:   %.1fs (grace %.0fs)\n", ev.Uptime, cfg.Grace)
		fmt.Fprintf(&b, "Unchanged for: %.1fs (black limit %.0fs, static limit %.0fs)\n",
			ev.StaticScreen, cfg.BlackThreshold, cfg.StaticThreshold)
		fmt.Fprintf(&b, "Moving area:   %d/%d cells in the last %.0fs (moving at %d+)\n",
			ev.Spread, gridCols*gridRows, cfg.SpreadTime, cfg.MinSpread)
		fmt.Fprintf(&b, "Changed:       %.2f%% of screen this frame\n", ev.ChangedPct)
		fmt.Fprintf(&b, "Stuck pixels:  %d/%d\n", ev.StuckPixels, ev.Samples)
		fmt.Fprintf(&b, "Resolution:    %dx%d\n", ev.Width, ev.Height)
		fmt.Fprintf(&b, "Colours:       dominant %s %s, average %s %s\n",
			rgbToHex(ev.DomR, ev.DomG, ev.DomB), nearestColorName(ev.DomR, ev.DomG, ev.DomB),
			rgbToHex(ev.AvgR, ev.AvgG, ev.AvgB), nearestColorName(ev.AvgR, ev.AvgG, ev.AvgB))
	}
	if action != "" {
		fmt.Fprintf(&b, "\nLast action:   %s\n", action)
	}

	tmp := StatusFile + ".tmp"
	if os.WriteFile(tmp, []byte(b.String()), 0644) == nil {
		_ = os.Rename(tmp, StatusFile)
	}
}

func (d *Detector) setAction(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println("[Detector] " + msg)
	d.mu.Lock()
	d.lastAction = time.Now().Format("15:04:05") + "  " + msg
	d.mu.Unlock()
}

func (d *Detector) current() (string, string, int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.systemID, d.title, d.gen
}

func (d *Detector) requestSkip(gen int) {
	select {
	case <-d.skip: // drop an older request
	default:
	}
	select {
	case d.skip <- gen:
	default:
	}
}

// sampleOffsets shifts the sampling grid a little each frame, so over four
// frames the detector checks four spots in every 8x8 block instead of one.
// Each frame is compared with the last frame that used the same offset, so
// the same pixels are always compared. Two spots sit on "even" pixels and
// two on "odd" ones, so fine patterns like checkerboards and dithering
// can't hide from the grid.
var sampleOffsets = [][2]int{{0, 0}, {4, 4}, {5, 0}, {1, 4}}

func (d *Detector) run() {
	defer d.res.Close()

	var (
		staticScreenRun, staticStartTime float64
		sampleFrames                     int
		lastFrameTime                    = time.Now()
		titleStartTime                   = time.Now()
		handledBlack, handledStatic      bool
		lastGen                          = -1
		currCfg                          config.StaticDetectorConfig
		frameNo                          int
	)

	// Samples in screen order, one saved frame per grid offset.
	var prevRGB [4][]uint32
	var prevSize [4][2]int // width, height each saved frame was taken at
	var havePrev [4]bool
	currRGB := make([]uint32, 0, 1024)
	currCell := make([]uint16, 0, 1024) // grid cell of each sample
	var motion motionTracker
	motion.reset()
	sorted := make([]uint32, 0, 1024) // copy for the dominant colour

	for {
		select {
		case <-d.stop:
			return
		default:
		}
		t1 := time.Now()

		systemID, title, gen := d.current()
		if gen != lastGen {
			lastGen = gen
			staticScreenRun, staticStartTime, sampleFrames = 0, 0, 0
			handledBlack, handledStatic = false, false
			havePrev = [4]bool{}
			motion.reset()
			lastFrameTime, titleStartTime = time.Now(), time.Now()
			currCfg = detectorConfigFor(d.cfg, systemID)
		}
		if systemID == "" {
			d.writeStatus(currCfg, false)
			time.Sleep(time.Second / targetFPS)
			continue
		}

		res := d.res
		res.Header = int(res.Map[2])<<8 | int(res.Map[3])
		res.Width = int(res.Map[6])<<8 | int(res.Map[7])
		res.Height = int(res.Map[8])<<8 | int(res.Map[9])
		res.Line = int(res.Map[10])<<8 | int(res.Map[11])

		valid := !(res.Width < 64 || res.Width > 2048 ||
			res.Height < 64 || res.Height > 2048 ||
			res.Line < res.Width*3 || res.Line > 2048*4) &&
			// the whole frame must fit in the mapped memory: a large or
			// corrupt header would otherwise read past it and crash
			res.Header+res.Height*res.Line <= len(res.Map)

		var sumR, sumG, sumB int
		currRGB = currRGB[:0]
		currCell = currCell[:0]
		slot := frameNo % len(sampleOffsets)
		ox, oy := sampleOffsets[slot][0], sampleOffsets[slot][1]
		frameNo++

		if !valid {
			currRGB = append(currRGB, 0)
		} else {
			for y := oy; y < res.Height; y += defaultStep {
				row := res.Map[res.Header+y*res.Line:]
				for x := ox; x < res.Width; x += defaultStep {
					off := x * 3
					if off+2 < res.Line {
						r := row[off]
						g := row[off+1]
						b := row[off+2]
						currRGB = append(currRGB, uint32(r)<<16|uint32(g)<<8|uint32(b))
						currCell = append(currCell, cellOf(x, y, res.Width, res.Height))
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

		// Dominant colour: sort a copy, so currRGB keeps screen order for
		// the frame-to-frame comparison below.
		sorted = append(sorted[:0], currRGB...)
		sort.Sort(rgbList(sorted)) // a typed sort: much cheaper than sort.Slice
		bestCount := 0
		currCount := 1
		bestVal := sorted[0]
		for i := 1; i <= len(sorted); i++ {
			if i < len(sorted) && sorted[i] == sorted[i-1] {
				currCount++
			} else {
				if currCount > bestCount {
					bestCount = currCount
					bestVal = sorted[i-1]
				}
				currCount = 1
			}
		}
		domR := int((bestVal >> 16) & 0xFF)
		domG := int((bestVal >> 8) & 0xFF)
		domB := int(bestVal & 0xFF)

		// Compare the same screen positions: this frame against the last
		// frame that used the same grid offset, marking each grid cell
		// that has a changed sample.
		frameTime := time.Now()
		now := frameTime.Sub(titleStartTime).Seconds()
		size := [2]int{res.Width, res.Height}
		stuckPixels := 0
		changedPct := 0.0
		spread := 0
		if havePrev[slot] {
			prev := prevRGB[slot]
			sameSize := prevSize[slot] == size && len(prev) == len(currRGB)
			if sameSize {
				for i := range currRGB {
					if currRGB[i] == prev[i] {
						stuckPixels++
					} else if i < len(currCell) {
						motion.mark(currCell[i], now)
					}
				}
				changedPct = float64(len(currRGB)-stuckPixels) * 100 / float64(len(currRGB))
			} else {
				// Resolution changed: the old cells mean nothing now.
				motion.reset()
			}
			spread = motion.spread(now, currCfg.SpreadTime)

			// Moving only if the change is spread over enough of the
			// screen, not stuck in one spot.
			minSpread := currCfg.MinSpread
			if minSpread < 1 {
				minSpread = 1
			}
			moving := !sameSize || spread >= minSpread
			if !moving {
				if staticScreenRun == 0 {
					// The screen only drops below MinSpread once its last
					// real movement is SpreadTime old, so that's when it
					// actually went still: count from there.
					back := currCfg.SpreadTime
					if back > now {
						back = now
					}
					staticStartTime = now - back
					staticScreenRun = back
				}
				staticScreenRun += frameTime.Sub(lastFrameTime).Seconds()
			} else {
				staticScreenRun = 0
				staticStartTime = 0
				handledStatic = false
				handledBlack = false
			}
		}
		prevRGB[slot] = append(prevRGB[slot][:0], currRGB...)
		prevSize[slot] = size
		havePrev[slot] = true
		lastFrameTime = frameTime

		uptime := frameTime.Sub(titleStartTime).Seconds()
		black := avgR|avgG|avgB == 0

		event := StaticEvent{
			Uptime:       uptime,
			Frames:       sampleFrames,
			StaticScreen: staticScreenRun,
			StuckPixels:  stuckPixels,
			Samples:      len(currRGB),
			Width:        res.Width,
			Height:       res.Height,
			DomR:         domR, DomG: domG, DomB: domB,
			AvgR: avgR, AvgG: avgG, AvgB: avgB,
			System: systemID, Title: title,
			ChangedPct: changedPct,
			Spread:     spread,
		}

		if uptime > currCfg.Grace {
			if black && staticScreenRun > currCfg.BlackThreshold && !handledBlack {
				msg := fmt.Sprintf("Black screen on %q", title)
				switch {
				case d.viewOnly:
					msg = "Black screen (view only: attract mode would" + wouldText(currCfg.WriteBlackList, "blacklist", currCfg.SkipBlack) + ")"
				case currCfg.WriteBlackList && AddToList(Blacklist, systemID, title) == nil:
					msg += ", added to " + systemID + " blacklist"
				}
				if currCfg.SkipBlack && !d.viewOnly {
					msg += ", skipping"
				}
				d.setAction("%s", msg) // report first, so it prints before the next launch
				if currCfg.SkipBlack && !d.viewOnly {
					d.requestSkip(gen)
				}
				handledBlack = true
			}
			if !black && staticScreenRun > currCfg.StaticThreshold && !handledStatic {
				msg := fmt.Sprintf("Static screen on %q from %.0fs", title, staticStartTime)
				switch {
				case d.viewOnly:
					msg = fmt.Sprintf("Static screen from %.0fs (view only: attract mode would", staticStartTime) +
						wouldText(currCfg.WriteStaticList, "staticlist", currCfg.SkipStatic) + ")"
				case currCfg.WriteStaticList && AddStatic(systemID, title, staticStartTime) == nil:
					msg += ", added to " + systemID + " staticlist"
				}
				if currCfg.SkipStatic && !d.viewOnly {
					msg += ", skipping"
				}
				d.setAction("%s", msg) // report first, so it prints before the next launch
				if currCfg.SkipStatic && !d.viewOnly {
					d.requestSkip(gen)
				}
				handledStatic = true
			}
		}

		d.mu.Lock()
		d.latest = event
		d.mu.Unlock()
		d.writeStatus(currCfg, false)

		elapsed := time.Since(t1)
		if frameDur := time.Second / targetFPS; elapsed < frameDur {
			time.Sleep(frameDur - elapsed)
		}
	}
}

// detectorConfigFor applies [StaticDetector.X] overrides for a system on
// top of [StaticDetector]: group sections first, then the system's own.
func detectorConfigFor(cfg *config.Config, systemID string) config.StaticDetectorConfig {
	out := cfg.StaticDetector
	id := strings.ToLower(systemID)

	var groups []string
	for name := range cfg.StaticDetectorOverrides {
		if name == id {
			continue
		}
		if ids, _ := games.ResolveSystems([]string{name}); ids[id] {
			groups = append(groups, name)
		}
	}
	sort.Strings(groups)
	for _, name := range groups {
		applyDetectorOverride(&out, cfg.StaticDetectorOverrides[name])
	}
	if own, ok := cfg.StaticDetectorOverrides[id]; ok {
		applyDetectorOverride(&out, own)
	}
	return out
}

func applyDetectorOverride(c *config.StaticDetectorConfig, keys map[string]string) {
	num := func(key string, dst *float64) {
		if v, ok := keys[key]; ok {
			if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
				*dst = f
			}
		}
	}
	flag := func(key string, dst *bool) {
		if v, ok := keys[key]; ok {
			if b, err := strconv.ParseBool(strings.TrimSpace(v)); err == nil {
				*dst = b
			}
		}
	}
	num("blackthreshold", &c.BlackThreshold)
	num("staticthreshold", &c.StaticThreshold)
	num("grace", &c.Grace)
	num("spreadtime", &c.SpreadTime)
	if v, ok := keys["minspread"]; ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			c.MinSpread = n
		}
	}
	flag("skipblack", &c.SkipBlack)
	flag("writeblacklist", &c.WriteBlackList)
	flag("skipstatic", &c.SkipStatic)
	flag("writestaticlist", &c.WriteStaticList)
}

// wouldText describes what attract mode would do, for view-only reports.
func wouldText(list bool, listName string, skip bool) string {
	var parts []string
	if list {
		parts = append(parts, "add it to the "+listName)
	}
	if skip {
		parts = append(parts, "skip it")
	}
	if len(parts) == 0 {
		return " do nothing"
	}
	return " " + strings.Join(parts, " and ")
}

// rgbList sorts colour samples (sort.Interface, for the detector's
// dominant colour: faster than sort.Slice on every frame).
type rgbList []uint32

func (l rgbList) Len() int           { return len(l) }
func (l rgbList) Less(i, j int) bool { return l[i] < l[j] }
func (l rgbList) Swap(i, j int)      { l[i], l[j] = l[j], l[i] }
