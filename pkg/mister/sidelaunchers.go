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
		offsetRomPath  = 0x0C   // AmigaVision.rom
		offsetHdfPath  = 0x418  // AmigaVision.hdf
		offsetSavePath = 0x81A  // AmigaVision-Saves.hdf
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
		fmt.Printf("[AmigaVision] Patched offset 0x%X -> %s\n", offset, replacement)
		return nil
	}
	// ----------------

	// 1. Prepare tmp work dir (extract only if fresh)
	tmpDir := "/tmp/.SAM_tmp/AmigaVision"
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		if err := os.MkdirAll(tmpDir, 0755); err != nil {
			return fmt.Errorf("failed to create tmp dir: %w", err)
		}
		if err := assets.ExtractZip(assets.AmigaVisionZip, tmpDir); err != nil {
			return fmt.Errorf("failed to extract AmigaVision assets: %w", err)
		}
		fmt.Printf("[AmigaVision] Extracted embedded assets to %s\n", tmpDir)
	} else {
		fmt.Printf("[AmigaVision] Reusing existing tmp dir: %s\n", tmpDir)
	}

	// 2. Locate system folder(s)
	sysPaths := games.GetSystemPaths(cfg, []games.System{system})
	if len(sysPaths) == 0 {
		return fmt.Errorf("[AmigaVision] No valid system paths found for %s", system.Name)
	}

	var pseudoRoot string
	for _, sp := range sysPaths {
		if _, err := os.Stat(filepath.Join(sp.Path, "AmigaVision.hdf")); err == nil {
			pseudoRoot = sp.Path
			break
		}
		if _, err := os.Stat(filepath.Join(sp.Path, "AmigaVision.rom")); err == nil {
			pseudoRoot = sp.Path
			break
		}
		if _, err := os.Stat(filepath.Join(sp.Path, "MegaAGS.hdf")); err == nil {
			pseudoRoot = sp.Path
			break
		}
	}
	if pseudoRoot == "" {
		pseudoRoot = sysPaths[0].Path
	}
	fmt.Printf("[AmigaVision] Using pseudoRoot = %s\n", pseudoRoot)

	// 3. Create patched AmigaVision.cfg in tmp
	misterCfg := "/media/fat/config/AmigaVision.cfg"
	tmpCfg := filepath.Join(tmpDir, "AmigaVision.cfg")

	data, err := os.ReadFile(filepath.Join(tmpDir, "AmigaVision.cfg"))
	if err != nil {
		return fmt.Errorf("failed to load AmigaVision.cfg from tmp: %w", err)
	}

	// --- Patch ROM (prefer user’s, else copy embedded to disk) ---
	romPath := filepath.Join(pseudoRoot, "AmigaVision.rom")
	if _, err := os.Stat(romPath); os.IsNotExist(err) {
		srcRom := filepath.Join(tmpDir, "AmigaVision.rom")
		if err := exec.Command("/bin/cp", srcRom, romPath).Run(); err != nil {
			return fmt.Errorf("failed to seed AmigaVision.rom to disk: %w", err)
		}
		fmt.Printf("[AmigaVision] Seeded ROM to %s\n", romPath)
	} else {
		fmt.Printf("[AmigaVision] Using existing ROM: %s\n", romPath)
	}
	if err := patchAt(data, offsetRomPath, cleanPath(romPath)); err != nil {
		return err
	}

	// --- Patch HDF (prefer AmigaVision, else MegaAGS) ---
	hdfPath := filepath.Join(pseudoRoot, "AmigaVision.hdf")
	if _, err := os.Stat(hdfPath); err == nil {
		_ = patchAt(data, offsetHdfPath, cleanPath(hdfPath))
	} else {
		megaHdfPath := filepath.Join(pseudoRoot, "MegaAGS.hdf")
		if _, err := os.Stat(megaHdfPath); err == nil {
			_ = patchAt(data, offsetHdfPath, cleanPath(megaHdfPath))
		}
	}

	// --- Patch Saves (prefer AmigaVision, else MegaAGS, optional) ---
	savePath := filepath.Join(pseudoRoot, "AmigaVision-Saves.hdf")
	if _, err := os.Stat(savePath); err == nil {
		_ = patchAt(data, offsetSavePath, cleanPath(savePath))
	} else {
		megaSavePath := filepath.Join(pseudoRoot, "MegaAGS-Saves.hdf")
		if _, err := os.Stat(megaSavePath); err == nil {
			_ = patchAt(data, offsetSavePath, cleanPath(megaSavePath))
		}
	}

	if err := os.WriteFile(tmpCfg, data, 0644); err != nil {
		return fmt.Errorf("failed to save patched AmigaVision.cfg: %w", err)
	}

	// Copy patched cfg to FAT if missing, else bind-mount
	if _, err := os.Stat(misterCfg); os.IsNotExist(err) {
		if err := exec.Command("/bin/cp", tmpCfg, misterCfg).Run(); err != nil {
			return fmt.Errorf("failed to copy patched cfg: %w", err)
		}
	} else {
		_ = exec.Command("umount", misterCfg).Run()
		cmd := exec.Command("mount", "--bind", tmpCfg, misterCfg)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to bind-mount cfg: %v (output: %s)", err, string(out))
		}
	}

	// 4. Handle shared dir
	sharedDir := filepath.Join(pseudoRoot, "shared")
	tmpShared := filepath.Join(tmpDir, "shared")

	// If user’s shared doesn’t exist, seed it from embedded
	if _, err := os.Stat(sharedDir); os.IsNotExist(err) {
		if err := os.MkdirAll(sharedDir, 0755); err != nil {
			return fmt.Errorf("failed to create shared dir: %w", err)
		}
		if out, err := exec.Command("/bin/cp", "-a", tmpShared+"/.", sharedDir).CombinedOutput(); err != nil {
			return fmt.Errorf("failed to seed shared to disk: %v (output: %s)", err, string(out))
		}
		fmt.Printf("[AmigaVision] Seeded shared to %s\n", sharedDir)
	}

	// Write ags_boot into tmpShared
	bootFile := filepath.Join(tmpShared, "ags_boot")
	cleanName := utils.RemoveFileExt(filepath.Base(path))
	if err := os.WriteFile(bootFile, []byte(cleanName+"\n\n"), 0644); err != nil {
		return fmt.Errorf("failed to write ags_boot: %v", err)
	}

	// Bind-mount tmpShared over system shared
	_ = exec.Command("umount", sharedDir).Run()
	if out, err := exec.Command("mount", "--bind", tmpShared, sharedDir).CombinedOutput(); err != nil {
		return fmt.Errorf("failed to bind-mount shared: %v (output: %s)", err, string(out))
	}

	// 5. Build minimal MGL
	mgl := `<mistergamedescription>
	<rbf>_computer/minimig</rbf>
	<setname same_dir="1">AmigaVision</setname>
</mistergamedescription>`
	tmpMgl := config.LastLaunchFile
	if err := os.WriteFile(tmpMgl, []byte(mgl), 0644); err != nil {
		return fmt.Errorf("failed to write MGL: %w", err)
	}

	// 6. Launch
	if err := launchFile(tmpMgl); err != nil {
		return fmt.Errorf("failed to launch AmigaVision MGL: %w", err)
	}

	fmt.Println("[SIDELAUNCHER] AmigaVision launched successfully!")
	return nil
}

// --------------------------------------------------
// AmigaCD32
// --------------------------------------------------

func LaunchCD32(cfg *config.UserConfig, system games.System, path string) error {
	fmt.Println("[SIDELAUNCHER] AmigaCD32 launch starting…")

	// --- Helpers ---
	cleanPath := func(p string) string {
		return "../" + strings.TrimPrefix(p, "/media/")
	}

	const (
		offsetRomPath   = 0x0C   // CD32.rom
		offsetHdfPath   = 0x418  // CD32.hdf
		offsetSavePath  = 0x81A  // AmigaVision-Saves.hdf (optional)
		offsetGamePath  = 0xC1F  // game path
		fieldLength     = 256
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

	// Helper to seed a missing disk asset from zip
	seedAsset := func(assetName, destDir string) (string, error) {
		destPath := filepath.Join(destDir, assetName)
		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			if err := assets.ExtractZipFile(assets.AmigaCD32Zip, assetName, destPath); err != nil {
				return "", fmt.Errorf("failed to seed %s: %w", assetName, err)
			}
			fmt.Printf("[AmigaCD32] Seeded %s → %s\n", assetName, destPath)
		}
		return destPath, nil
	}
	// -------------------------------------------------

	// 1. Prepare tmp dir
	tmpDir := "/tmp/.SAM_tmp/AmigaCD32"
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		if err := os.MkdirAll(tmpDir, 0755); err != nil {
			return fmt.Errorf("failed to create tmp dir: %w", err)
		}
	}
	tmpCfg := filepath.Join(tmpDir, "AmigaCD32.cfg")
	if err := assets.ExtractZipFile(assets.AmigaCD32Zip, "AmigaCD32.cfg", tmpCfg); err != nil {
		return fmt.Errorf("failed to extract AmigaCD32.cfg: %w", err)
	}

	// 2. Locate system folder(s)
	sysPaths := games.GetSystemPaths(cfg, []games.System{system})
	if len(sysPaths) == 0 {
		return fmt.Errorf("[AmigaCD32] No valid system paths found for %s", system.Name)
	}
	pseudoRoot := sysPaths[0].Path

	// 3. Ensure FAT cfg exists
	misterCfg := "/media/fat/config/AmigaCD32.cfg"
	if _, err := os.Stat(misterCfg); os.IsNotExist(err) {
		if err := assets.ExtractZipFile(assets.AmigaCD32Zip, "AmigaCD32.cfg", misterCfg); err != nil {
			return fmt.Errorf("failed to seed AmigaCD32.cfg to FAT: %w", err)
		}
	}

	// 4. Read cfg for patching
	data, err := os.ReadFile(tmpCfg)
	if err != nil {
		return fmt.Errorf("failed to load tmp AmigaCD32.cfg: %w", err)
	}

	// --- ROM ---
	romPath, err := seedAsset("CD32.rom", pseudoRoot)
	if err != nil {
		return err
	}
	if err := patchAt(data, offsetRomPath, cleanPath(romPath)); err != nil {
		return err
	}

	// --- HDF ---
	hdfPath, err := seedAsset("CD32.hdf", pseudoRoot)
	if err != nil {
		return err
	}
	if err := patchAt(data, offsetHdfPath, cleanPath(hdfPath)); err != nil {
		return err
	}

	// --- Saves (optional, no seeding) ---
	savePath := filepath.Join(pseudoRoot, "AmigaVision-Saves.hdf")
	if _, err := os.Stat(savePath); err == nil {
		_ = patchAt(data, offsetSavePath, cleanPath(savePath))
	}

	// --- Game path ---
	absGame, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve game path: %w", err)
	}
	absGame = strings.TrimPrefix(absGame, "/media/")
	if err := patchAt(data, offsetGamePath, absGame); err != nil {
		return err
	}

	// Save patched tmp cfg
	if err := os.WriteFile(tmpCfg, data, 0644); err != nil {
		return fmt.Errorf("failed to save patched tmp cfg: %w", err)
	}

	// 5. Bind tmp cfg over FAT cfg
	_ = exec.Command("umount", misterCfg).Run()
	cmd := exec.Command("mount", "--bind", tmpCfg, misterCfg)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to bind-mount cfg: %v (output: %s)", err, string(out))
	}

	// 6. Build minimal MGL
	mgl := `<mistergamedescription>
	<rbf>_computer/minimig</rbf>
	<setname same_dir="1">AmigaCD32</setname>
</mistergamedescription>`
	tmpMgl := config.LastLaunchFile
	if err := os.WriteFile(tmpMgl, []byte(mgl), 0644); err != nil {
		return fmt.Errorf("failed to write MGL: %w", err)
	}

	// 7. Launch
	if err := launchFile(tmpMgl); err != nil {
		return fmt.Errorf("failed to launch AmigaCD32 MGL: %w", err)
	}

	fmt.Println("[SIDELAUNCHER] AmigaCD32 launched successfully!")
	return nil
}
