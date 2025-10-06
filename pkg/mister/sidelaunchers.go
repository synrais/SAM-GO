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

var sideLauncherRegistry = map[string]func(*config.UserConfig, games.System, string) error{}

func registerSideLauncher(id string, fn func(*config.UserConfig, games.System, string) error) {
	id = strings.ToLower(id)
	sideLauncherRegistry[id] = fn
}

func init() {
	registerSideLauncher("AmigaVision", LaunchAmigaVision)
	registerSideLauncher("AmigaCD32", LaunchCD32)
	registerSideLauncher("FDS", LaunchFDS)
	registerSideLauncher("GameNWatch", LaunchGameNWatch)
}

// SideLaunchers checks if system.Id has a sidelauncher
func SideLaunchers(cfg *config.UserConfig, system games.System, path string) (bool, error) {
	fn, ok := sideLauncherRegistry[strings.ToLower(system.Id)]
	if !ok {
		return false, nil
	}
	return true, fn(cfg, system, path)
}

// --------------------------------------------------
// AmigaVision
// --------------------------------------------------

func LaunchAmigaVision(cfg *config.UserConfig, system games.System, path string) error {
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
			if err := assets.ExtractZipFile(assets.AmigaCD32Zip, "AmigaVision-Saves.hdf", savePath); err != nil {
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
// AmigaCD32
// --------------------------------------------------

func LaunchCD32(cfg *config.UserConfig, system games.System, path string) error {
	cleanPath := func(p string) string {
		return "../" + strings.TrimPrefix(p, "/media/")
	}

	const (
		offsetRomPath  = 0x0C
		offsetHdfPath  = 0x418
		offsetSavePath = 0x81A
		offsetGamePath = 0xC1F
		fieldLength    = 256
	)

	patchAt := func(data []byte, offset int, replacement string) error {
		if len(replacement) > fieldLength {
			return fmt.Errorf("replacement too long")
		}
		copy(data[offset:], []byte(replacement))
		for i := offset + len(replacement); i < offset+fieldLength; i++ {
			data[i] = 0x00
		}
		return nil
	}

	seedAsset := func(assetName, destDir string) (string, error) {
		dest := filepath.Join(destDir, assetName)
		if _, err := os.Stat(dest); os.IsNotExist(err) {
			if err := assets.ExtractZipFile(assets.AmigaCD32Zip, assetName, dest); err != nil {
				return "", err
			}
		}
		return dest, nil
	}

	compatHDF := map[string]string{
		"chaos engine":            "CD32NoFastMem.hdf",
		"dangerous streets":       "CD32NoFastMem.hdf",
		"fears":                   "CD32NoFastMem.hdf",
		"humans":                  "CD32NoFastMem.hdf",
		"lotus trilogy":           "CD32NoFastMem.hdf",
		"pinball fantasies":       "CD32NoFastMem.hdf",
		"quik the thunder rabbit": "CD32NoFastMem.hdf",
		"soccer kid":              "CD32NoFastMem.hdf",
		"surf ninjas":             "CD32NoFastMem.hdf",
		"fire force":              "CD32NoFastMem.hdf",
		"dizzy collection":        "CD32NoFastMemNoICache.hdf",
		"ultimate body blows":     "CD32NoICache.hdf",
		"guardian":                "CD32NoVolumeControl.hdf",
		"arabian nights":          "CD32Winboot.hdf",
		"beneath a steel sky":     "CD32Winboot.hdf",
		"deep core":               "CD32Winboot.hdf",
		"fields of glory":         "CD32Winboot.hdf",
	}

	tmpDir := "/tmp/.SAM_tmp/AmigaCD32"
	_ = os.MkdirAll(tmpDir, 0755)
	tmpCfg := filepath.Join(tmpDir, "AmigaCD32.cfg")
	_ = assets.ExtractZipFile(assets.AmigaCD32Zip, "AmigaCD32.cfg", tmpCfg)

	sysPaths := games.GetSystemPaths(cfg, []games.System{system})
	if len(sysPaths) == 0 {
		return fmt.Errorf("no system paths")
	}

	var pseudoRoot string
	required := []string{"CD32.rom", "CD32.hdf", "AmigaVision-Saves.hdf"}
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

	misterCfg := "/media/fat/config/AmigaCD32.cfg"
	if _, err := os.Stat(misterCfg); os.IsNotExist(err) {
		_ = assets.ExtractZipFile(assets.AmigaCD32Zip, "AmigaCD32.cfg", misterCfg)
	}

	data, _ := os.ReadFile(tmpCfg)

	var romPath string
	foundRom := false
	for _, sp := range sysPaths {
		for _, candidate := range []string{"AmigaVision.rom", "MegaAGS.rom"} {
			cpath := filepath.Join(sp.Path, candidate)
			if _, err := os.Stat(cpath); err == nil {
				romPath = cpath
				foundRom = true
				break
			}
		}
		if foundRom {
			break
		}
	}
	if !foundRom {
		romPath, _ = seedAsset("CD32.rom", pseudoRoot)
	}
	_ = patchAt(data, offsetRomPath, cleanPath(romPath))

	hdfNames := []string{
		"CD32NoVolumeControl.hdf",
		"CD32NoICache.hdf",
		"CD32NoFastMemNoICache.hdf",
		"CD32NoFastMem.hdf",
		"CD32Winboot.hdf",
		"CD32.hdf",
	}
	for _, h := range hdfNames {
		_, _ = seedAsset(h, pseudoRoot)
	}

	gameName := strings.ToLower(filepath.Base(path))
	hdfToUse := "CD32.hdf"
	for key, alt := range compatHDF {
		if strings.Contains(gameName, key) {
			hdfToUse = alt
			break
		}
	}

	hdfPath := filepath.Join(pseudoRoot, hdfToUse)
	_ = patchAt(data, offsetHdfPath, cleanPath(hdfPath))

	var savePath string
	foundSave := false
	for _, sp := range sysPaths {
		for _, candidate := range []string{"AmigaVision-Saves.hdf", "MegaAGS-Saves.hdf"} {
			cpath := filepath.Join(sp.Path, candidate)
			if _, err := os.Stat(cpath); err == nil {
				savePath = cpath
				foundSave = true
				break
			}
		}
		if foundSave {
			break
		}
	}
	if !foundSave {
		savePath, _ = seedAsset("AmigaVision-Saves.hdf", pseudoRoot)
	}
	_ = patchAt(data, offsetSavePath, cleanPath(savePath))

	absGame, _ := filepath.Abs(path)
	absGame = strings.TrimPrefix(absGame, "/media/")
	_ = patchAt(data, offsetGamePath, absGame)

	_ = os.WriteFile(tmpCfg, data, 0644)
	_ = exec.Command("umount", misterCfg).Run()
	_ = exec.Command("mount", "--bind", tmpCfg, misterCfg).Run()

	mgl := `<mistergamedescription>
	<rbf>_computer/minimig</rbf>
	<setname same_dir="1">AmigaCD32</setname>
</mistergamedescription>`
	tmpMgl := config.LastLaunchFile
	_ = os.WriteFile(tmpMgl, []byte(mgl), 0644)

	if hdfToUse == "CD32Winboot.hdf" {
		go func() {
			kbd, err := virtualinput.NewKeyboard(40 * time.Millisecond)
			if err != nil {
				return
			}
			defer kbd.Close()
			time.Sleep(11 * time.Second)
			if code, ok := virtualinput.ToKeyboardCode("b"); ok {
				_ = kbd.Press(code)
			}
		}()
	}

	return launchFile(tmpMgl)
}

// --------------------------------------------------
// FDS Sidelauncher
// --------------------------------------------------

func LaunchFDS(cfg *config.UserConfig, system games.System, path string) error {
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
// GameNWatch Sidelauncher (non-blocking + logging)
// --------------------------------------------------

func LaunchGameNWatch(cfg *config.UserConfig, system games.System, path string) error {
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
