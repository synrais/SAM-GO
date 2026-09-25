package mister

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/bendahl/uinput"
	"github.com/synrais/SAM-GO/pkg/assets"
	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/input/virtualinput"
	"github.com/synrais/SAM-GO/pkg/games"
	"github.com/synrais/SAM-GO/pkg/utils"
)

// --------------------------------------------------
// Registry
// --------------------------------------------------

var sideLauncherRegistry = map[string]func(*config.Config, games.System, string) error{}

func registerSideLauncher(id string, fn func(*config.Config, games.System, string) error) {
	id = strings.ToLower(id)
	sideLauncherRegistry[id] = fn
}

func init() {
	registerSideLauncher("AmigaVision", LaunchAmigaVision)
	// FDS: now [BiosSkip.FDS] in SAM.ini.
	// GameAndWatch: its old button presses hit the core's power and time
	// buttons; add an [BiosSkip.GameAndWatch] sequence once confirmed.
}

// SideLaunchers checks if system.Id has a sidelauncher
func SideLaunchers(cfg *config.Config, system games.System, path string) (bool, error) {
	fn, ok := sideLauncherRegistry[strings.ToLower(system.Id)]
	if !ok {
		return false, nil
	}
	return true, fn(cfg, system, path)
}

// --------------------------------------------------
// AmigaVision
// --------------------------------------------------

func LaunchAmigaVision(cfg *config.Config, system games.System, path string) error {
	if !strings.EqualFold(filepath.Ext(path), ".ags") {
		return nil
	}

	cleanPath := func(p string) string {
		return "../" + strings.TrimPrefix(p, "/media/")
	}

	const (
		offsetRomPath  = 0x0C
		offsetHdfPath  = 0x418
		offsetSavePath = 0x81A
		fieldLength    = 256
	)

	patchAt := func(data []byte, offset int, replacement string) error {
		if len(replacement) > fieldLength {
			return fmt.Errorf("replacement too long for field at 0x%X", offset)
		}
		copy(data[offset:], []byte(replacement))
		for i := offset + len(replacement); i < offset+fieldLength; i++ {
			data[i] = 0x00
		}
		return nil
	}

	tmpDir := "/tmp/.SAM_tmp/AmigaVision"
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		if err := os.MkdirAll(tmpDir, 0755); err != nil {
			return err
		}
		if err := assets.ExtractZip(assets.AmigaVisionZip, tmpDir); err != nil {
			return err
		}
	}

	sysPaths := games.GetSystemPaths(cfg, []games.System{system})
	if len(sysPaths) == 0 {
		return fmt.Errorf("no valid system paths found for %s", system.Name)
	}

	var pseudoRoot string
	required := []string{"AmigaVision.hdf", "AmigaVision.rom", "MegaAGS.hdf", "AmigaVision-Saves.hdf", "MegaAGS-Saves.hdf"}
	for _, sp := range sysPaths {
		for _, f := range required {
			if _, err := os.Stat(filepath.Join(sp.Path, f)); err == nil {
				pseudoRoot = sp.Path
				break
			}
		}
		if pseudoRoot != "" {
			break
		}
	}
	if pseudoRoot == "" {
		pseudoRoot = sysPaths[0].Path
	}

	misterCfg := "/media/fat/config/AmigaVision.cfg"
	tmpCfg := filepath.Join(tmpDir, "AmigaVision.cfg")

	data, err := os.ReadFile(tmpCfg)
	if err != nil {
		return err
	}

	romPath := filepath.Join(pseudoRoot, "AmigaVision.rom")
	if _, err := os.Stat(romPath); os.IsNotExist(err) {
		srcRom := filepath.Join(tmpDir, "AmigaVision.rom")
		_ = exec.Command("/bin/cp", srcRom, romPath).Run()
	}
	_ = patchAt(data, offsetRomPath, cleanPath(romPath))

	hdfPath := filepath.Join(pseudoRoot, "AmigaVision.hdf")
	if _, err := os.Stat(hdfPath); err == nil {
		_ = patchAt(data, offsetHdfPath, cleanPath(hdfPath))
	} else {
		megaHdf := filepath.Join(pseudoRoot, "MegaAGS.hdf")
		if _, err := os.Stat(megaHdf); err == nil {
			_ = patchAt(data, offsetHdfPath, cleanPath(megaHdf))
		}
	}

	savePath := filepath.Join(pseudoRoot, "AmigaVision-Saves.hdf")
	if _, err := os.Stat(savePath); os.IsNotExist(err) {
		megaSave := filepath.Join(pseudoRoot, "MegaAGS-Saves.hdf")
		if _, err := os.Stat(megaSave); err == nil {
			savePath = megaSave
		} else {
			if err := assets.ExtractZipFile(assets.AmigaVisionSavesZip, "AmigaVision-Saves.hdf", savePath); err != nil {
				return err
			}
		}
	}
	_ = patchAt(data, offsetSavePath, cleanPath(savePath))

	_ = os.WriteFile(tmpCfg, data, 0644)

	if _, err := os.Stat(misterCfg); os.IsNotExist(err) {
		_ = exec.Command("/bin/cp", tmpCfg, misterCfg).Run()
	} else {
		_ = exec.Command("umount", misterCfg).Run()
		_ = exec.Command("mount", "--bind", tmpCfg, misterCfg).Run()
	}

	sharedDir := filepath.Join(pseudoRoot, "shared")
	tmpShared := filepath.Join(tmpDir, "shared")
	if _, err := os.Stat(sharedDir); os.IsNotExist(err) {
		_ = os.MkdirAll(sharedDir, 0755)
		_ = exec.Command("/bin/cp", "-a", tmpShared+"/.", sharedDir).Run()
	}
	bootFile := filepath.Join(tmpShared, "ags_boot")
	cleanName := utils.RemoveFileExt(filepath.Base(path))
	_ = os.WriteFile(bootFile, []byte(cleanName+"\n\n"), 0644)
	_ = exec.Command("umount", sharedDir).Run()
	_ = exec.Command("mount", "--bind", tmpShared, sharedDir).Run()

	mgl := `<mistergamedescription>
	<rbf>_computer/minimig</rbf>
	<setname same_dir="1">AmigaVision</setname>
</mistergamedescription>`
	tmpMgl := config.LastLaunchFile
	_ = os.WriteFile(tmpMgl, []byte(mgl), 0644)

	return launchFile(tmpMgl)
}

// --------------------------------------------------
// FDS Sidelauncher
// --------------------------------------------------

func LaunchFDS(cfg *config.Config, system games.System, path string) error {
	gpd, err := virtualinput.NewGamepad(40 * time.Millisecond)
	if err != nil {
		return err
	}

	if err := launchTempMgl(cfg, &system, path); err != nil {
		_ = gpd.Close()
		return err
	}

	go func(g virtualinput.Gamepad) {
		time.Sleep(10 * time.Second)
		_ = g.Press(uinput.ButtonEast)
		_ = g.Close()
	}(gpd)

	return nil
}

// --------------------------------------------------
// GameAndWatch Sidelauncher (non-blocking + logging)
// --------------------------------------------------

func LaunchGameAndWatch(cfg *config.Config, system games.System, path string) error {
	gpd, err := virtualinput.NewGamepad(40 * time.Millisecond)
	if err != nil {
		return err
	}

	if err := launchTempMgl(cfg, &system, path); err != nil {
		_ = gpd.Close()
		return err
	}

	go func(g virtualinput.Gamepad) {
		time.Sleep(10 * time.Second)
		_ = g.Press(uinput.ButtonEast)
		time.Sleep(1 * time.Second)
		_ = g.Press(uinput.ButtonSouth)
		time.Sleep(1 * time.Second)
		_ = g.Press(uinput.ButtonWest)
		time.Sleep(1 * time.Second)
		_ = g.Press(uinput.ButtonNorth)
		time.Sleep(1 * time.Second)
		_ = g.Close()
	}(gpd)

	return nil
}
