package staticdetector

import (
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

const (
	misterScalerBaseAddr   = 0x20000000
	misterScalerBufferSize = 2048 * 3 * 1024
)

type namedColor struct {
	name    string
	r, g, b uint8
}

var colorTable = []namedColor{
	{"Black", 0, 0, 0},
	{"White", 255, 255, 255},
	{"Red", 255, 0, 0},
	{"Green", 0, 255, 0},
	{"Blue", 0, 0, 255},
	{"Yellow", 255, 255, 0},
	{"Cyan", 0, 255, 255},
	{"Magenta", 255, 0, 255},
	{"Gray", 128, 128, 128},
	{"Orange", 255, 165, 0},
	{"Purple", 128, 0, 128},
	{"Pink", 255, 192, 203},
}

func nearestColorName(r, g, b uint8) string {
	bestIdx := 0
	bestDist := int64(math.MaxInt64)
	for i, c := range colorTable {
		dr := int64(int(r) - int(c.r))
		dg := int64(int(g) - int(c.g))
		db := int64(int(b) - int(c.b))
		dist := dr*dr + dg*dg + db*db
		if dist < bestDist {
			bestDist = dist
			bestIdx = i
		}
	}
	return colorTable[bestIdx].name
}

func readTmpFile(path string) string {
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

func readRomName() string {
	rom := readTmpFile("/tmp/SAM_Game.txt")
	if rom == "Unknown" || rom == "" {
		rom = readTmpFile("/tmp/ROM")
	}
	if rom == "Unknown" || rom == "" {
		rom = readTmpFile("/tmp/NAME")
	}
	if rom == "" {
		rom = "Unknown"
	}
	return rom
}

// Stream returns a channel that emits status lines with static detection info.
func Stream() <-chan string {
	out := make(chan string, 10)

	go func() {
		defer close(out)

		fd, err := unix.Open("/dev/mem", unix.O_RDONLY|unix.O_SYNC, 0)
		if err != nil {
			out <- fmt.Sprintf("open /dev/mem: %v", err)
			return
		}
		defer unix.Close(fd)

		data, err := unix.Mmap(fd, misterScalerBaseAddr, misterScalerBufferSize, unix.PROT_READ, unix.MAP_SHARED)
		if err != nil {
			out <- fmt.Sprintf("mmap: %v", err)
			return
		}
		defer unix.Munmap(data)

		var frameCount int
		lastCnt := -1
		start := time.Now()
		var lastHash uint32
		staticActive := false
		var staticStart, staticNow time.Time

		for {
			buffer := data
			if !(buffer[0] == 1 && buffer[1] == 1) {
				time.Sleep(2 * time.Millisecond)
				continue
			}

			flags := buffer[5]
			frameCnt := int((flags >> 5) & 0x07)
			if lastCnt == -1 {
				lastCnt = frameCnt
			} else if frameCnt != lastCnt {
				delta := (frameCnt - lastCnt) & 0x07
				if delta > 0 {
					frameCount += delta
				}
				lastCnt = frameCnt
			}

			now := time.Now()
			elapsed := now.Sub(start).Seconds()
			if elapsed < 0.1 {
				time.Sleep(2 * time.Millisecond)
				continue
			}
			fps := float64(frameCount) / elapsed
			frameCount = 0
			start = now

			hdrOffset := int(buffer[3])<<8 | int(buffer[4])
			width := int(buffer[6])<<8 | int(buffer[7])
			height := int(buffer[8])<<8 | int(buffer[9])
			stride := int(buffer[10])<<8 | int(buffer[11])
			outW := int(buffer[12])<<8 | int(buffer[13])
			outH := int(buffer[14])<<8 | int(buffer[15])
			_ = buffer[16] // format unused

			rom := readRomName()

			fb := buffer[hdrOffset:]
			var hash uint32
			sampleCount := 0
			var rSum, gSum, bSum int64

			for y := 0; y < height; y += 16 {
				row := fb[y*stride:]
				for x := 0; x < width; x += 16 {
					idx := x * 3
					if idx+2 >= len(row) {
						break
					}
					b := row[idx]
					r := row[idx+1]
					g := row[idx+2]
					hash = hash*131 + uint32(r) + (uint32(g) << 8) + (uint32(b) << 16)
					rSum += int64(r)
					gSum += int64(g)
					bSum += int64(b)
					sampleCount++
				}
			}

			if hash == lastHash {
				if !staticActive {
					staticActive = true
					staticStart = time.Now()
				}
				staticNow = time.Now()
			} else {
				staticActive = false
				lastHash = hash
			}

			var staticSeconds float64
			if staticActive {
				staticSeconds = staticNow.Sub(staticStart).Seconds()
			}

			var domR, domG, domB uint8
			if sampleCount > 0 {
				domR = uint8(rSum / int64(sampleCount))
				domG = uint8(gSum / int64(sampleCount))
				domB = uint8(bSum / int64(sampleCount))
			}

			hexColor := fmt.Sprintf("#%02X%02X%02X", domR, domG, domB)
			humanColor := nearestColorName(domR, domG, domB)

			output := "NO"
			if buffer[0] == 1 && buffer[1] == 1 {
				output = "YES"
			}

			line := fmt.Sprintf("Output=%s | StaticTime=%.1f | RGB=%s -> %s | FPS=%.2f | Resolution=%dx%d -> %dx%d | Game=%s",
				output, staticSeconds, hexColor, humanColor, fps, width, height, outW, outH, rom)
			out <- line

			time.Sleep(2 * time.Millisecond)
		}
	}()

	return out
}
