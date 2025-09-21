package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	playerDir = "/tmp/mrext-mplayer"
	menuCore  = "/media/fat/menu.rbf"
)

// --- helpers ---
func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func killMister() { _ = exec.Command("killall", "-q", "MiSTer").Run() }

func fbset(width, height int) error {
	return runCmd("fbset", "-g",
		fmt.Sprint(width), fmt.Sprint(height),
		fmt.Sprint(width), fmt.Sprint(height),
		"32")
}

func setVirtualTerm(id string) error { return runCmd("chvt", id) }

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

func hideCursor(vt string) { _ = writeTty(vt, "\033[?25l") }
func showCursor(vt string) { _ = writeTty(vt, "\033[?25h") }

// --- setup/cleanup ---
func setupRemotePlay() error {
	killMister()
	if err := fbset(640, 480); err != nil {
		return fmt.Errorf("fbset failed: %w", err)
	}
	if err := setVirtualTerm("9"); err != nil {
		return fmt.Errorf("chvt 9 failed: %w", err)
	}
	hideCursor("9")
	return nil
}

func cleanupRemotePlay() {
	showCursor("9")
	_ = setVirtualTerm("1")
	// Relaunch MiSTer menu core
	_ = runCmd(menuCore)
}

// --- player ---
func runMplayer(path string) error {
	playerPath := filepath.Join(playerDir, "mplayer")
	cmd := exec.Command(playerPath, path)
	cmd.Env = append(os.Environ(), "LD_LIBRARY_PATH="+playerDir)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// --- main ---
func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: mplayer <moviefile>")
		os.Exit(1)
	}
	movie := os.Args[1]

	if err := setupRemotePlay(); err != nil {
		panic(err)
	}

	if err := runMplayer(movie); err != nil {
		panic(err)
	}

	cleanupRemotePlay()
}
