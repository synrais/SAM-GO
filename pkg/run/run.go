package run

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/wizzomafizzo/mrext/pkg/config"
	"github.com/wizzomafizzo/mrext/pkg/games"
	"github.com/wizzomafizzo/mrext/pkg/mister"
)

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func bindMount(src, dst string) error {
	_ = os.MkdirAll(dst, 0755)
	cmd := exec.Command("mount", "-o", "bind", src, dst)
	return cmd.Run()
}

func unmount(path string) {
	// ignore errors – unmount may fail if nothing is mounted
	_ = exec.Command("umount", path).Run()
}

func findAmigaShared() string {
	// look in configured system paths
	amigaPaths := games.GetSystemPaths(&config.UserConfig{}, []games.System{games.Systems["Amiga"]})
	for _, p := range amigaPaths {
		candidate := filepath.Join(p.Path, "shared")
		if pathExists(candidate) {
			return candidate
		}
	}

	// fallback: try usb0-3
	for i := 0; i < 4; i++ {
		usbCandidate := fmt.Sprintf("/media/usb%d/games/Amiga/shared", i)
		if pathExists(usbCandidate) {
			return usbCandidate
		}
	}

	// fallback: fat
	if pathExists("/media/fat/games/Amiga/shared") {
		return "/media/fat/games/Amiga/shared"
	}
	return ""
}

// Run launches a game or AmigaVision target.
// It no longer exits the process – caller handles errors.
func Run(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("Usage: SAM -run <path-or-name>")
	}
	runPath := args[0]

	// Case 1: AmigaVision name (anything without slash/backslash)
	if !strings.ContainsAny(runPath, "/\\") {
		amigaShared := findAmigaShared()
		if amigaShared == "" {
			return fmt.Errorf("games/Amiga/shared folder not found")
		}

		tmpShared := "/tmp/.SAM_tmp/Amiga_shared"
		_ = os.RemoveAll(tmpShared)
		_ = os.MkdirAll(tmpShared, 0755)

		// copy real shared into tmp
		_ = exec.Command("cp", "-a", amigaShared+"/.", tmpShared).Run()

		// write ags_boot file
		bootFile := filepath.Join(tmpShared, "ags_boot")
		content := runPath + "\n\n"
		_ = os.WriteFile(bootFile, []byte(content), 0644)

		// bind mount over real shared
		unmount(amigaShared)
		if err := bindMount(tmpShared, amigaShared); err != nil {
			return fmt.Errorf("bind mount failed: %v", err)
		}

		// launch minimig core
		return mister.LaunchCore(&config.UserConfig{}, games.Systems["Amiga"])
	}

	// Case 2: MGL file (case-insensitive extension check)
	if strings.EqualFold(filepath.Ext(runPath), ".mgl") {
		return mister.LaunchGenericFile(&config.UserConfig{}, runPath)
	}

	// Case 3: generic file path
	system, _ := games.BestSystemMatch(&config.UserConfig{}, runPath)
	return mister.LaunchGame(&config.UserConfig{}, system, runPath)
}
