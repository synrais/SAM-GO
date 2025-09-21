package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/synrais/SAM-GO/pkg/assets"
	"github.com/synrais/SAM-GO/pkg/mister"
)

const playerDir = "/tmp/mrext-mplayer"

// setupPlayer extracts the embedded mplayer.zip into /tmp
func setupPlayer() error {
	if err := os.MkdirAll(playerDir, 0755); err != nil {
		return err
	}
	return assets.ExtractZipBytes(assets.MPlayerZip, playerDir)
}

// setVirtualTerm switches to a given virtual terminal (e.g. "9" or "1")
func setVirtualTerm(id string) error {
	cmd := exec.Command("chvt", id)
	return cmd.Run()
}

// writeTty writes directly to a tty device
func writeTty(id string, s string) error {
	tty := "/dev/tty" + id
	f, err := os.OpenFile(tty, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(s)
	return err
}

// hide/show cursor
func hideCursor(vt string) error { return writeTty(vt, "\033[?25l") }
func showCursor(vt string) error { return writeTty(vt, "\033[?25h") }

// setupRemotePlay prepares the VT and video mode before running mplayer
func setupRemotePlay() error {
	// Kill any running MiSTer core/menu first
	if err := mister.KillRunning(); err != nil {
		fmt.Printf("[WARN] failed to kill running core: %v\n", err)
	}

	if err := setVirtualTerm("9"); err != nil {
		return fmt.Errorf("switch VT: %w", err)
	}
	if err := hideCursor("9"); err != nil {
		return fmt.Errorf("hide cursor: %w", err)
	}
	if err := mister.SetVideoMode(640, 480); err != nil {
		return fmt.Errorf("set video mode: %w", err)
	}
	return nil
}

// cleanupRemotePlay restores MiSTer to normal menu
func cleanupRemotePlay() {
	_ = showCursor("9")
	_ = setVirtualTerm("1")
	_ = mister.LaunchMenu()
}

// runMplayer executes the extracted mplayer binary
func runMplayer(path string) error {
	playerPath := filepath.Join(playerDir, "mplayer")
	cmd := exec.Command(playerPath, path)
	cmd.Env = append(os.Environ(), "LD_LIBRARY_PATH="+playerDir)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: mplayer <moviefile>")
		os.Exit(1)
	}
	movie := os.Args[1]

	if err := setupPlayer(); err != nil {
		panic(err)
	}

	if err := setupRemotePlay(); err != nil {
		panic(err)
	}

	if err := runMplayer(movie); err != nil {
		panic(err)
	}

	cleanupRemotePlay()
}
