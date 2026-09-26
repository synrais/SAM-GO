package mister

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/synrais/SAMenu/pkg/config"
)

// Console text size
//
// The Linux console that scripts draw on sits on MiSTer's framebuffer. Its
// resolution is the TV output divided by 1 to 4 (fb_size in MiSTer.ini),
// and the same font on a smaller framebuffer means bigger text. MiSTer
// also changes it at runtime with "fb_cmd0 <format> <rb> <divider>", and
// the console re-lays itself out straight away.

// OnConsole reports whether this program is on the MiSTer's own screen
// (the Linux console) rather than, say, an SSH session.
func OnConsole() bool {
	link, err := os.Readlink("/proc/self/fd/0")
	return err == nil && strings.HasPrefix(link, "/dev/tty") && link != "/dev/tty"
}

// ConsoleSize returns the console's size in characters.
func ConsoleSize() (rows, cols int, err error) {
	ws, err := unix.IoctlGetWinsize(0, unix.TIOCGWINSZ)
	if err != nil {
		return 0, 0, err
	}
	return int(ws.Row), int(ws.Col), nil
}

// fbFormat reads the framebuffer's current colour format, so a size
// change keeps it ("fmt rb width height stride").
func fbFormat() (fmtNum, rb int) {
	fmtNum = 8888
	b, err := os.ReadFile("/sys/module/MiSTer_fb/parameters/mode")
	if err != nil {
		return
	}
	f := strings.Fields(string(b))
	if len(f) >= 2 {
		if n, err := strconv.Atoi(f[0]); err == nil && n != 0 {
			fmtNum = n
		}
		if n, err := strconv.Atoi(f[1]); err == nil {
			rb = n
		}
	}
	return
}

// SetFbDivider sets the framebuffer to the TV output divided by div (1 to
// 4), and waits for the console to take the new size (up to 2 seconds).
func SetFbDivider(div int) error {
	if div < 1 || div > 4 {
		return fmt.Errorf("divider %d out of range", div)
	}
	beforeRows, beforeCols, _ := ConsoleSize()
	f, err := os.OpenFile(config.CmdInterface, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	fm, rb := fbFormat()
	_, err = fmt.Fprintf(f, "fb_cmd0 %d %d %d\n", fm, rb, div)
	f.Close()
	if err != nil {
		return err
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
		if r, c, err := ConsoleSize(); err == nil && (r != beforeRows || c != beforeCols) {
			time.Sleep(100 * time.Millisecond) // let it settle
			return nil
		}
	}
	return nil // same size as before (already at this divider)
}

// SetFbSize sets the framebuffer to an exact size ("fb_cmd1"), e.g.
// 640x480 for videos, and waits for it to take (up to 2 seconds). MiSTer's
// scaler still fills the TV with it.
func SetFbSize(width, height int) error {
	if !FbFits(width, height) {
		return fmt.Errorf("%dx%d is too big for the framebuffer", width, height)
	}
	f, err := os.OpenFile(config.CmdInterface, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	fm, rb := fbFormat()
	_, err = fmt.Fprintf(f, "fb_cmd1 %d %d %d %d\n", fm, rb, width, height)
	f.Close()
	if err != nil {
		return err
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if w, h := FbSize(); w == width && h == height {
			time.Sleep(100 * time.Millisecond) // let it settle
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil
}

// fbMaxPixels is MiSTer's framebuffer memory limit (FB_SIZE in its
// video.cpp). A divider giving a bigger framebuffer than this corrupts
// the screen, and fb_cmd0 doesn't check it.
const fbMaxPixels = 1920 * 1080

// FbSize returns the framebuffer's current size in pixels.
func FbSize() (width, height int) {
	b, err := os.ReadFile("/sys/module/MiSTer_fb/parameters/mode")
	if err != nil {
		return 0, 0
	}
	f := strings.Fields(string(b))
	if len(f) >= 4 {
		width, _ = strconv.Atoi(f[2])
		height, _ = strconv.Atoi(f[3])
	}
	return
}

// FbFits reports whether a framebuffer of this size fits MiSTer's memory.
func FbFits(width, height int) bool { return width*height <= fbMaxPixels }
