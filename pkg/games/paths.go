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
// Shared helper
// --------------------------------------------------

// searchFileAcrossSysPaths looks across all system paths for a filename.
func searchFileAcrossSysPaths(sysPaths []games.PathResult, filename string) string {
	for _, sp := range sysPaths {
		fullPath := filepath.Join(sp.Path, filename)
		if _, err := os.Stat(fullPath); err == nil {
			return fullPath
		}
	}
	return ""
}

// --------------------------------------------------
// AmigaVision
// --------------------------------------------------

func LaunchAmigaVision(cfg *config.UserConfig, system games.System, path string) error {
	fmt.Println("[SIDELAUNCHER] AmigaVision launch starting…")

	// Only handle .ags files
	if !strings.EqualFold(filepath.Ext(path), ".ags") {
		return nil
	}

	// --- Helpers ---
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

	// 1. Prepare tmp work dir
	tmpDir := "/tmp/.SAM_tmp/AmigaVision"
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		if err := os.MkdirAll(tmpDir, 0755); err != nil {
			return err
		}
		if err := assets.ExtractZip(assets.AmigaVisionZip, tmpDir); err != nil {
			return fmt.Errorf("failed to extract AmigaVision assets: %w", err)
		}
	}

	// 2. Locate system folder(s)
	sysPaths := games.GetSystemPaths(cfg, []games.System{system})
	if len(sysPaths) == 0 {
		return fmt.Errorf("[AmigaVision] No valid system paths found")
	}

	// 3. Create patched AmigaVision.cfg in tmp
	misterCfg := "/media/fat/config/AmigaVision.cfg"
	tmpCfg := filepath.Join(tmpDir, "AmigaVision.cfg")

	data, err := os.ReadFile(tmpCfg)
	if err != nil {
		return fmt.Errorf("failed to load AmigaVision.cfg: %w", err)
	}

	// ROM
	romPath := searchFileAcrossSysPaths(sysPaths, "AmigaVision.rom")
	if romPath == "" {
		srcRom := filepath.Join(tmpDir, "AmigaVision.rom")
		dest := filepath.Join(sysPaths[0].Path, "AmigaVision.rom")
		if err := exec.Command("/bin/cp", srcRom, dest).Run(); err != nil {
			return fmt.Errorf("failed to seed AmigaVision.rom: %w", err)
		}
		romPath = dest
	}
	_ = patchAt(data, offsetRomPath, cleanPath(romPath))

	// HDF
	hdfPath := searchFileAcrossSysPaths(sysPaths, "AmigaVision.hdf")
	if hdfPath == "" {
		hdfPath = searchFileAcrossSysPaths(sysPaths, "MegaAGS.hdf")
	}
	if hdfPath != "" {
		_ = patchAt(data, offsetHdfPath, cleanPath(hdfPath))
	}

	// Saves
	savePath := searchFileAcrossSysPaths(sysPaths, "AmigaVision-Saves.hdf")
	if savePath == "" {
		savePath = searchFileAcrossSysPaths(sysPaths, "MegaAGS-Saves.hdf")
	}
	if savePath == "" {
		// fallback: extract from CD32 zip if totally missing
		dest := filepath.Join(sysPaths[0].Path, "AmigaVision-Saves.hdf")
		if err := assets.ExtractZipFile(assets.AmigaCD32Zip, "AmigaVision-Saves.hdf", dest); err != nil {
			return fmt.Errorf("failed to seed AmigaVision-Saves.hdf: %w", err)
		}
		savePath = dest
	}
	_ = patchAt(data, offsetSavePath, cleanPath(savePath))

	if err := os.WriteFile(tmpCfg, data, 0644); err != nil {
		return err
	}

	// Copy patched cfg to FAT if missing, else bind-mount
	if _, err := os.Stat(misterCfg); os.IsNotExist(err) {
		if err := exec.Command("/bin/cp", tmpCfg, misterCfg).Run(); err != nil {
			return err
		}
	} else {
		_ = exec.Command("umount", misterCfg).Run()
		_ = exec.Command("mount", "--bind", tmpCfg, misterCfg).Run()
	}

	// Handle shared dir
	sharedDir := filepath.Join(sysPaths[0].Path, "shared")
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

	// Build MGL
	mgl := `<mistergamedescription>
	<rbf>_computer/minimig</rbf>
	<setname same_dir="1">AmigaVision</setname>
</mistergamedescription>`
	tmpMgl := config.LastLaunchFile
	_ = os.WriteFile(tmpMgl, []byte(mgl), 0644)

	// Launch
	return launchFile(tmpMgl)
}

// --------------------------------------------------
// AmigaCD32
// --------------------------------------------------

func LaunchCD32(cfg *config.UserConfig, system games.System, path string) error {
	fmt.Println("[SIDELAUNCHER] AmigaCD32 launch starting…")

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
			return fmt.Errorf("too long for 0x%X", offset)
		}
		copy(data[offset:], []byte(replacement))
		for i := offset + len(replacement); i < offset+fieldLength; i++ {
			data[i] = 0x00
		}
		return nil
	}

	// 1. Prepare tmp dir + config
	tmpDir := "/tmp/.SAM_tmp/AmigaCD32"
	_ = os.MkdirAll(tmpDir, 0755)
	tmpCfg := filepath.Join(tmpDir, "AmigaCD32.cfg")
	_ = assets.ExtractZipFile(assets.AmigaCD32Zip, "AmigaCD32.cfg", tmpCfg)

	// 2. Locate system folder(s)
	sysPaths := games.GetSystemPaths(cfg, []games.System{system})
	if len(sysPaths) == 0 {
		return fmt.Errorf("[AmigaCD32] No valid system paths found")
	}

	// 3. Ensure FAT cfg exists
	misterCfg := "/media/fat/config/AmigaCD32.cfg"
	if _, err := os.Stat(misterCfg); os.IsNotExist(err) {
		_ = assets.ExtractZipFile(assets.AmigaCD32Zip, "AmigaCD32.cfg", misterCfg)
	}

	// 4. Read cfg
	data, _ := os.ReadFile(tmpCfg)

	// ROM (use AmigaVision.rom if present, else CD32.rom)
	romPath := searchFileAcrossSysPaths(sysPaths, "AmigaVision.rom")
	if romPath == "" {
		romPath = searchFileAcrossSysPaths(sysPaths, "CD32.rom")
		if romPath == "" {
			dest := filepath.Join(sysPaths[0].Path, "CD32.rom")
			_ = assets.ExtractZipFile(assets.AmigaCD32Zip, "CD32.rom", dest)
			romPath = dest
		}
	}
	_ = patchAt(data, offsetRomPath, cleanPath(romPath))

	// HDFs (always seed if missing)
	for _, hdf := range []string{
		"CD32NoVolumeControl.hdf",
		"CD32NoICache.hdf",
		"CD32NoFastMemNoICache.hdf",
		"CD32NoFastMem.hdf",
		"CD32.hdf",
		"AmigaVision-Saves.hdf",
	} {
		hdfPath := searchFileAcrossSysPaths(sysPaths, hdf)
		if hdfPath == "" {
			dest := filepath.Join(sysPaths[0].Path, hdf)
			_ = assets.ExtractZipFile(assets.AmigaCD32Zip, hdf, dest)
			hdfPath = dest
		}
		if hdf == "CD32.hdf" {
			_ = patchAt(data, offsetHdfPath, cleanPath(hdfPath))
		}
		if hdf == "AmigaVision-Saves.hdf" {
			_ = patchAt(data, offsetSavePath, cleanPath(hdfPath))
		}
	}

	// Game path
	absGame, _ := filepath.Abs(path)
	absGame = strings.TrimPrefix(absGame, "/media/")
	_ = patchAt(data, offsetGamePath, absGame)

	_ = os.WriteFile(tmpCfg, data, 0644)

	// Bind tmp cfg over FAT cfg
	_ = exec.Command("umount", misterCfg).Run()
	_ = exec.Command("mount", "--bind", tmpCfg, misterCfg).Run()

	// Build MGL
	mgl := `<mistergamedescription>
	<rbf>_computer/minimig</rbf>
	<setname same_dir="1">AmigaCD32</setname>
</mistergamedescription>`
	tmpMgl := config.LastLaunchFile
	_ = os.WriteFile(tmpMgl, []byte(mgl), 0644)

	return launchFile(tmpMgl)
}
