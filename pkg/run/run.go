package run

import (
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/games"
	"github.com/synrais/SAM-GO/pkg/mister"
)

// Globals to expose last played info
var (
	LastPlayedSystem games.System
	LastPlayedPath   string
	LastPlayedName   string // basename without extension (or raw AmigaVision name)
	LastStartTime    time.Time
)

// GetLastPlayed returns the last system, path, clean name, and start time.
func GetLastPlayed() (system games.System, path, name string, start time.Time) {
	return LastPlayedSystem, LastPlayedPath, LastPlayedName, LastStartTime
}

// internal helper to update globals
func setLastPlayed(system games.System, path string) {
	LastPlayedSystem = system
	LastPlayedPath = path
	LastStartTime = time.Now()

	// derive clean display name
	if !strings.Contains(path, "/") && !strings.Contains(path, "\\") {
		// AmigaVision-style: already just a name
		LastPlayedName = path
	} else {
		base := filepath.Base(path)
		LastPlayedName = strings.TrimSuffix(base, filepath.Ext(base))
	}
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
	cmd := exec.Command("/bin/mount", "--bind", src, dst)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mount failed: %v (output: %s)", err, string(out))
	}
	return nil
}

func unmount(path string) {
	cmd := exec.Command("/bin/umount", path)
	_ = cmd.Run() // ignore errors, same as shell script
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
func Run(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("Usage: SAM -run <path-or-name>")
	}
	runPath := args[0]

	// Case 1: AmigaVision name (no slash/backslash → special case)
	if !strings.ContainsAny(runPath, "/\\") {
		amigaShared := findAmigaShared()
		if amigaShared == "" {
			return fmt.Errorf("games/Amiga/shared folder not found")
		}

		tmpShared := "/tmp/.SAM_tmp/Amiga_shared"
		_ = os.RemoveAll(tmpShared)
		_ = os.MkdirAll(tmpShared, 0755)

		// copy real shared into tmp
		if out, err := exec.Command("/bin/cp", "-a", amigaShared+"/.", tmpShared).CombinedOutput(); err != nil {
			fmt.Printf("[WARN] copy shared failed: %v (output: %s)\n", err, string(out))
		}

		// write ags_boot file
		bootFile := filepath.Join(tmpShared, "ags_boot")
		content := runPath + "\n\n"
		if err := os.WriteFile(bootFile, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write ags_boot: %v", err)
		}

		// bind mount over real shared
		unmount(amigaShared)
		if err := bindMount(tmpShared, amigaShared); err != nil {
			return err
		}

		// record last played (Amiga special case)
		setLastPlayed(games.Systems["Amiga"], runPath)

		// launch minimig core
		return mister.LaunchCore(&config.UserConfig{}, games.Systems["Amiga"])
	}

	// Case 2: MGL file
	if strings.EqualFold(filepath.Ext(runPath), ".mgl") {
		realPath, err := parseMglForGamePath(runPath)
		if err == nil && realPath != "" {
			if system, err := games.BestSystemMatch(&config.UserConfig{}, realPath); err == nil {
				setLastPlayed(system, realPath)
			} else {
				setLastPlayed(games.System{}, runPath)
			}
		} else {
			setLastPlayed(games.System{}, runPath)
		}
		return mister.LaunchGenericFile(&config.UserConfig{}, runPath)
	}

	// Case 3: generic file path
	system, _ := games.BestSystemMatch(&config.UserConfig{}, runPath)
	setLastPlayed(system, runPath)
	return mister.LaunchGame(&config.UserConfig{}, system, runPath)
}
