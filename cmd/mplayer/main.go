package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/synrais/SAM-GO/pkg/assets"
)

const (
	playerDir            = "/tmp/mrext-mplayer"
	menuCore             = "/media/fat/menu.rbf"
	misterCmdDevice      = "/dev/MiSTer_cmd"
	samvideoDisplayWait  = 2 * time.Second // adjust if needed
	defaultResolution    = "640 480"       // fallback resolution
)

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func setupPlayer() error {
	if err := os.MkdirAll(playerDir, 0755); err != nil {
		return err
	}
	return assets.ExtractZipBytes(assets.MPlayerZip, playerDir)
}

// tell MiSTer to load the menu core
func loadMenuCore() error {
	cmd := exec.Command("sh", "-c", fmt.Sprintf("echo load_core %s > %s", menuCore, misterCmdDevice))
	return cmd.Run()
}

// set resolution via vmode
func setResolution(resSpace string) error {
	if resSpace == "" {
		resSpace = defaultResolution
	}
	return runCmd("vmode", "-r", resSpace, "rgb32")
}

// run mplayer with nice -n -20 and LD_LIBRARY_PATH
func runMplayer(path string, nosound bool) error {
	playerPath := filepath.Join(playerDir, "mplayer")

	args := []string{
		"-msglevel", "all=0:statusline=5",
	}
	if nosound {
		args = append(args, "-nosound")
	}
	args = append(args, path)

	cmd := exec.Command("nice", append([]string{"-n", "-20", playerPath}, args...)...)
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
	videoFile := os.Args[1]

	// Extract mplayer + libs
	if err := setupPlayer(); err != nil {
		panic(err)
	}

	// Ask MiSTer to load menu core
	if err := loadMenuCore(); err != nil {
		panic(fmt.Errorf("failed to load menu core: %w", err))
	}
	time.Sleep(samvideoDisplayWait)

	// Set resolution (for now static, later detect like SAM does)
	if err := setResolution(defaultResolution); err != nil {
		fmt.Println("warning: failed to set resolution:", err)
	}

	// Run mplayer
	if err := runMplayer(videoFile, false); err != nil {
		panic(err)
	}

	// After playback, MiSTer will still be in menu core
	fmt.Println("Playback finished, back in MiSTer menu.")
}
