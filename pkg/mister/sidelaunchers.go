package mister

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/synrais/SAM-GO/pkg/assets"
	"github.com/synrais/SAM-GO/pkg/config"
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
}

// SideLaunchers checks if system.Id has a sidelauncher
func SideLaunchers(cfg *config.UserConfig, system games.System, path string) (bool, error) {
	fn, ok := sideLauncherRegistry[strings.ToLower(system.Id)]
	if !ok {
		return false, nil
	}

	cleanName := utils.RemoveFileExt(filepath.Base(path))
	fmt.Printf("[SIDELAUNCHER] %s: %s\n", system.Id, cleanName)

	return true, fn(cfg, system, path)
}

// --------------------------------------------------
// Helpers
// --------------------------------------------------

func patchAt(data []byte, offset int, replacement string) error {
	const fieldLength = 256
	if len(replacement) > fieldLength {
		return fmt.Errorf("replacement too long for field at 0x%X", offset)
	}
	copy(data[offset:], []byte(replacement))
	for i := offset + len(replacement); i < offset+fieldLength; i++ {
		data[i] = 0x00
	}
	return nil
}

func cleanPath(p string) string {
	return "../" + strings.TrimPrefix(p, "/media/")
}

// searchFileAcrossSysPaths looks across all sysPaths for the first matching file.
func searchFileAcrossSysPaths(sysPaths []games.SystemPath, candidates ...string) string {
	for _, sp := range sysPaths {
		for _, name := range candidates {
			full := filepath.Join(sp.Path, name)
			if _, err := os.Stat(full); err == nil {
				return full
			}
		}
	}
	return ""
}

// --------------------------------------------------
// AmigaVision
// --------------------------------------------------

func LaunchAmigaVision(cfg *config.UserConfig, system games.System, path string) error {
	fmt.Println("[SIDELAUNCHER] AmigaVision launch starting…")

	if !strings.EqualFold(filepath.Ext(path), ".ags") {
		return nil
	}

	const (
		offsetRomPath  = 0x0C
		offsetHdfPath  = 0x418
		offsetSavePath = 0x81A
	)

	tmpDir := "/tmp/.SAM_tmp/AmigaVision"
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		if err := os.MkdirAll(tmpDir, 0755); err != nil {
			return fmt.Errorf("failed to create tmp dir: %w", err)
		}
		if err := assets.ExtractZip(assets.AmigaVisionZip, tmpDir); err != nil {
			return fmt.Errorf("failed to extract AmigaVision assets: %w", err)
		}
	}

	sysPaths := games.GetSystemPaths(cfg, []games.System{system})
	if len(sysPaths) == 0 {
		return fmt.Errorf("[AmigaVision] No valid system paths found")
	}
	pseudoRoot := sysPaths[0].Path

	misterCfg := "/media/fat/config/AmigaVision.cfg"
	tmpCfg := filepath.Join(tmpDir, "AmigaVision.cfg")

	data, err := os.ReadFile(tmpCfg)
	if err != nil {
		return fmt.Errorf("failed to load AmigaVision.cfg: %w", err)
	}

	// --- ROM ---
	romPath := searchFileAcrossSysPaths(sysPaths, "AmigaVision.rom")
	if romPath == "" {
		srcRom := filepath.Join(tmpDir, "AmigaVision.rom")
		romPath = filepath.Join(pseudoRoot, "AmigaVision.rom")
		_ = exec.Command("/bin/cp", srcRom, romPath).Run()
	}
	_ = patchAt(data, offsetRomPath, cleanPath(romPath))

	// --- HDF ---
	hdfPath := searchFileAcrossSysPaths(sysPaths, "AmigaVision.hdf", "MegaAGS.hdf")
	if hdfPath != "" {
		_ = patchAt(data, offsetHdfPath, cleanPath(hdfPath))
	}

	// --- Saves ---
	savePath := searchFileAcrossSysPaths(sysPaths, "AmigaVision-Saves.hdf", "MegaAGS-Saves.hdf")
	if savePath == "" {
		savePath = filepath.Join(pseudoRoot, "AmigaVision-Saves.hdf")
		if err := assets.ExtractZipFile(assets.AmigaCD32Zip, "AmigaVision-Saves.hdf", savePath); err != nil {
			return fmt.Errorf("failed to seed AmigaVision-Saves.hdf: %w", err)
		}
	}
	_ = patchAt(data, offsetSavePath, cleanPath(savePath))

	if err := os.WriteFile(tmpCfg, data, 0644); err != nil {
		return fmt.Errorf("failed to save patched cfg: %w", err)
	}

	if _, err := os.Stat(misterCfg); os.IsNotExist(err) {
		_ = exec.Command("/bin/cp", tmpCfg, misterCfg).Run()
	} else {
		_ = exec.Command("umount", misterCfg).Run()
		_ = exec.Command("mount", "--bind", tmpCfg, misterCfg).Run()
	}

	// --- Shared dir ---
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

	// --- MGL ---
	mgl := `<mistergamedescription>
	<rbf>_computer/minimig</rbf>
	<setname same_dir="1">AmigaVision</setname>
</mistergamedescription>`
	_ = os.WriteFile(config.LastLaunchFile, []byte(mgl), 0644)

	return launchFile(config.LastLaunchFile)
}

// --------------------------------------------------
// AmigaCD32
// --------------------------------------------------

func LaunchCD32(cfg *config.UserConfig, system games.System, path string) error {
	fmt.Println("[SIDELAUNCHER] AmigaCD32 launch starting…")

	const (
		offsetRomPath  = 0x0C
		offsetHdfPath  = 0x418
		offsetSavePath = 0x81A
		offsetGamePath = 0xC1F
	)

	tmpDir := "/tmp/.SAM_tmp/AmigaCD32"
	_ = os.MkdirAll(tmpDir, 0755)
	tmpCfg := filepath.Join(tmpDir, "AmigaCD32.cfg")
	_ = assets.ExtractZipFile(assets.AmigaCD32Zip, "AmigaCD32.cfg", tmpCfg)

	sysPaths := games.GetSystemPaths(cfg, []games.System{system})
	if len(sysPaths) == 0 {
		return fmt.Errorf("[AmigaCD32] No valid system paths found")
	}
	pseudoRoot := sysPaths[0].Path

	misterCfg := "/media/fat/config/AmigaCD32.cfg"
	if _, err := os.Stat(misterCfg); os.IsNotExist(err) {
		_ = assets.ExtractZipFile(assets.AmigaCD32Zip, "AmigaCD32.cfg", misterCfg)
	}

	data, err := os.ReadFile(tmpCfg)
	if err != nil {
		return err
	}

	// --- ROM ---
	romPath := searchFileAcrossSysPaths(sysPaths, "AmigaVision.rom", "CD32.rom")
	if romPath == "" {
		romPath = filepath.Join(pseudoRoot, "CD32.rom")
		_ = assets.ExtractZipFile(assets.AmigaCD32Zip, "CD32.rom", romPath)
	}
	_ = patchAt(data, offsetRomPath, cleanPath(romPath))

	// --- HDFs (all variants) ---
	hdfFiles := []string{
		"CD32NoVolumeControl.hdf",
		"CD32NoICache.hdf",
		"CD32NoFastMemNoICache.hdf",
		"CD32NoFastMem.hdf",
		"CD32.hdf",
	}
	for _, name := range hdfFiles {
		full := searchFileAcrossSysPaths(sysPaths, name)
		if full == "" {
			full = filepath.Join(pseudoRoot, name)
			_ = assets.ExtractZipFile(assets.AmigaCD32Zip, name, full)
		}
		if name == "CD32.hdf" {
			_ = patchAt(data, offsetHdfPath, cleanPath(full))
		}
	}

	// --- Saves ---
	savePath := searchFileAcrossSysPaths(sysPaths, "AmigaVision-Saves.hdf", "MegaAGS-Saves.hdf")
	if savePath == "" {
		savePath = filepath.Join(pseudoRoot, "AmigaVision-Saves.hdf")
		_ = assets.ExtractZipFile(assets.AmigaCD32Zip, "AmigaVision-Saves.hdf", savePath)
	}
	_ = patchAt(data, offsetSavePath, cleanPath(savePath))

	// --- Game path ---
	absGame, _ := filepath.Abs(path)
	absGame = strings.TrimPrefix(absGame, "/media/")
	_ = patchAt(data, offsetGamePath, absGame)

	_ = os.WriteFile(tmpCfg, data, 0644)
	_ = exec.Command("umount", misterCfg).Run()
	_ = exec.Command("mount", "--bind", tmpCfg, misterCfg).Run()

	// --- MGL ---
	mgl := `<mistergamedescription>
	<rbf>_computer/minimig</rbf>
	<setname same_dir="1">AmigaCD32</setname>
</mistergamedescription>`
	_ = os.WriteFile(config.LastLaunchFile, []byte(mgl), 0644)

	return launchFile(config.LastLaunchFile)
}
