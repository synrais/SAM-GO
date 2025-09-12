package run

import (
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/games"
	"github.com/synrais/SAM-GO/pkg/mister"
)

// Globals to expose last played info
var (
	LastPlayedSystem games.System
	LastPlayedPath   string
)

// internal helper to update globals
func setLastPlayed(system games.System, path string) {
	LastPlayedSystem = system
	LastPlayedPath = path
}

// parseMglForGamePath opens an .mgl file and returns the <file> path, if any.
type mglFile struct {
	Path string `xml:"path,attr"`
}
type mglDoc struct {
	Files []mglFile `xml:"file"`
}

func parseMglForGamePath(mglPath string) (string, error) {
	data, err := os.ReadFile(mglPath)
	if err != nil {
		return "", err
	}
	var doc mglDoc
	if err := xml.Unmarshal(data, &doc); err != nil {
		return "", err
	}
	if len(doc.Files) == 0 {
		return "", nil // valid MGL but no <file> entries
	}
	return doc.Files[0].Path, nil
}

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

		// record last played (Amiga special case)
		setLastPlayed(games.Systems["Amiga"], runPath)

		// launch minimig core
		return mister.LaunchCore(&config.UserConfig{}, games.Systems["Amiga"])
	}

	// Case 2: MGL file (case-insensitive extension check)
	if strings.EqualFold(filepath.Ext(runPath), ".mgl") {
		realPath, err := parseMglForGamePath(runPath)
		if err == nil && realPath != "" {
			if system, err := games.BestSystemMatch(&config.UserConfig{}, realPath); err == nil {
				setLastPlayed(system, realPath)
			} else {
				// fallback: store generic MGL path if system not resolved
				setLastPlayed(games.System{}, runPath)
			}
		} else {
			// fallback: no <file> in MGL
			setLastPlayed(games.System{}, runPath)
		}
		return mister.LaunchGenericFile(&config.UserConfig{}, runPath)
	}

	// Case 3: generic file path
	system, _ := games.BestSystemMatch(&config.UserConfig{}, runPath)
	setLastPlayed(system, runPath)
	return mister.LaunchGame(&config.UserConfig{}, system, runPath)
}
