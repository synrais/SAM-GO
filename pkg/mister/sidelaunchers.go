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

	// 1. Prepare tmp work dir + extract embedded AmigaVision.zip
	tmpDir := "/tmp/.SAM_tmp/AmigaVision"
	_ = os.RemoveAll(tmpDir)
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return fmt.Errorf("failed to create tmp dir: %w", err)
	}
	if err := assets.ExtractZipBytes(assets.AmigaVisionZip, tmpDir); err != nil {
		return fmt.Errorf("failed to extract AmigaVision assets: %w", err)
	}
	fmt.Printf("[AmigaVision] Extracted embedded assets to %s\n", tmpDir)

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
		return fmt.Errorf("failed to load embedded AmigaVision.cfg: %w", err)
	}

	// --- Patch ROM (prefer user’s, else embedded from tmp) ---
	romPath := filepath.Join(pseudoRoot, "AmigaVision.rom")
	if _, err := os.Stat(romPath); os.IsNotExist(err) {
		romPath = filepath.Join(tmpDir, "AmigaVision.rom")
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

	// Save patched cfg into tmp
	if err := os.WriteFile(tmpCfg, data, 0644); err != nil {
		return fmt.Errorf("failed to save patched AmigaVision.cfg: %w", err)
	}

	// Handle final cfg on FAT (seed if missing, then always bind)
	if _, err := os.Stat(misterCfg); os.IsNotExist(err) {
		fmt.Printf("[AmigaVision] No existing cfg, copying patched cfg to %s\n", misterCfg)
		if err := exec.Command("/bin/cp", tmpCfg, misterCfg).Run(); err != nil {
			return fmt.Errorf("failed to copy patched cfg: %w", err)
		}
	} else {
		_ = exec.Command("umount", misterCfg).Run()
	}

	cmd := exec.Command("mount", "--bind", tmpCfg, misterCfg)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to bind-mount cfg: %v (output: %s)", err, string(out))
	}

	// 4. Prepare tmp shared + ags_boot
	sharedDir := filepath.Join(pseudoRoot, "shared")
	tmpShared := filepath.Join(tmpDir, "shared")

	_ = os.RemoveAll(tmpShared)
	if err := os.MkdirAll(tmpShared, 0755); err != nil {
		return fmt.Errorf("failed to create tmp shared: %w", err)
	}

	// Copy either user’s shared or embedded one
	if _, err := os.Stat(sharedDir); err == nil {
		if out, err := exec.Command("/bin/cp", "-a", sharedDir+"/.", tmpShared).CombinedOutput(); err != nil {
			fmt.Printf("[WARN] copy shared failed: %v (output: %s)\n", err, string(out))
		}
	} else {
		embeddedShared := filepath.Join(tmpDir, "shared")
		if out, err := exec.Command("/bin/cp", "-a", embeddedShared+"/.", tmpShared).CombinedOutput(); err != nil {
			return fmt.Errorf("failed to seed shared from embedded: %v (output: %s)", err, string(out))
		}
	}

	// Write ags_boot file
	bootFile := filepath.Join(tmpShared, "ags_boot")
	cleanName := utils.RemoveFileExt(filepath.Base(path))
	if err := os.WriteFile(bootFile, []byte(cleanName+"\n\n"), 0644); err != nil {
		return fmt.Errorf("failed to write ags_boot: %v", err)
	}

	// Bind-mount tmp shared over system shared
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

	// --- Local helpers ---
	cleanPath := func(p string) string {
		return "../" + strings.TrimPrefix(p, "/media/")
	}

	const (
		offsetRomPath   = 0x0C   // AmigaVision.rom
		offsetHdfPath   = 0x418  // AmigaCD32.hdf
		offsetSavePath  = 0x81A  // AmigaVision-Saves.hdf
		offsetGamePath  = 0xC1F  // game path (fixed)
		fieldLength     = 256    // safe default per field
	)

	patchAt := func(data []byte, offset int, replacement string) error {
		if len(replacement) > fieldLength {
			return fmt.Errorf("replacement too long for field at 0x%X", offset)
		}
		copy(data[offset:], []byte(replacement))
		for i := offset + len(replacement); i < offset+fieldLength; i++ {
			data[i] = 0x00
		}
		fmt.Printf("[AmigaCD32] Patched offset 0x%X -> %s\n", offset, replacement)
		return nil
	}
	// ----------------------

	// 1. Prepare tmp work dir
	tmpDir := "/tmp/.SAM_tmp/AmigaCD32"
	fmt.Printf("[AmigaCD32] Preparing tmp dir: %s\n", tmpDir)
	_ = os.RemoveAll(tmpDir)
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return fmt.Errorf("failed to create tmp dir: %w", err)
	}

	// 2. Locate system folder(s) and pick the best one
	sysPaths := games.GetSystemPaths(cfg, []games.System{system})
	if len(sysPaths) == 0 {
		return fmt.Errorf("[AmigaCD32] No valid system paths found for %s", system.Name)
	}

	var pseudoRoot string

	// Prefer saves
	for _, sp := range sysPaths {
		if _, err := os.Stat(filepath.Join(sp.Path, "AmigaVision-Saves.hdf")); err == nil {
			fmt.Printf("[AmigaCD32] Found save file in %s\n", sp.Path)
			pseudoRoot = sp.Path
			break
		}
	}

	// Else prefer ROM
	if pseudoRoot == "" {
		for _, sp := range sysPaths {
			if _, err := os.Stat(filepath.Join(sp.Path, "AmigaVision.rom")); err == nil {
				fmt.Printf("[AmigaCD32] Found ROM in %s\n", sp.Path)
				pseudoRoot = sp.Path
				break
			}
		}
	}

	// Else fallback
	if pseudoRoot == "" {
		pseudoRoot = sysPaths[0].Path
		fmt.Printf("[AmigaCD32] No saves or ROMs found, falling back to %s\n", pseudoRoot)
	} else {
		fmt.Printf("[AmigaCD32] Using pseudoRoot = %s\n", pseudoRoot)
	}

	// 3. Ensure cfg file exists on FAT (only as bind target)
	misterCfg := "/media/fat/config/AmigaCD32.cfg"
	if _, err := os.Stat(misterCfg); os.IsNotExist(err) {
		fmt.Printf("[AmigaCD32] No existing cfg at %s, writing blank one\n", misterCfg)
		if err := os.WriteFile(misterCfg, []byte{}, 0644); err != nil { // 🔹 placeholder
			return fmt.Errorf("failed to write initial AmigaCD32.cfg: %w", err)
		}
	}

	// 4. Create tmp cfg from a blank placeholder
	tmpCfg := filepath.Join(tmpDir, "AmigaCD32.cfg")
	data := make([]byte, 4096) // 🔹 placeholder size
	// ----------------------

	// --- Game path ---
	absGame, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve game path: %w", err)
	}
	absGame = strings.TrimPrefix(absGame, "/media/")
	if err := patchAt(data, offsetGamePath, absGame); err != nil {
		return err
	}

	// --- ROM: prefer existing AmigaVision.rom ---
	romPath := filepath.Join(pseudoRoot, "AmigaVision.rom")
	if _, err := os.Stat(romPath); err == nil {
		fmt.Printf("[AmigaCD32] Using existing AmigaVision ROM: %s\n", romPath)
	}
	if err := patchAt(data, offsetRomPath, cleanPath(romPath)); err != nil {
		return err
	}

	// --- HDF: prefer existing AmigaCD32.hdf ---
	hdfPath := filepath.Join(pseudoRoot, "AmigaCD32.hdf")
	if _, err := os.Stat(hdfPath); err == nil {
		fmt.Printf("[AmigaCD32] Using existing AmigaCD32 HDF: %s\n", hdfPath)
	}
	if err := patchAt(data, offsetHdfPath, cleanPath(hdfPath)); err != nil {
		return err
	}

	// --- Saves (optional) ---
	savePath := filepath.Join(pseudoRoot, "AmigaVision-Saves.hdf")
	if _, err := os.Stat(savePath); err == nil {
		fmt.Printf("[AmigaCD32] Found save file: %s\n", savePath)
		if err := patchAt(data, offsetSavePath, cleanPath(savePath)); err != nil {
			return err
		}
	}

	// Save patched cfg to tmp
	if err := os.WriteFile(tmpCfg, data, 0644); err != nil {
		return fmt.Errorf("failed to save patched cfg: %w", err)
	}
	fmt.Printf("[AmigaCD32] Patched cfg written to %s\n", tmpCfg)

	// 5. Replace cfg on FAT with bind-mount
	fmt.Printf("[AmigaCD32] Replacing cfg with bind-mount: %s -> %s\n", tmpCfg, misterCfg)
	_ = exec.Command("umount", misterCfg).Run() // ignore errors
	cmd := exec.Command("mount", "--bind", tmpCfg, misterCfg)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to bind-mount cfg: %v (output: %s)", err, string(out))
	}
	fmt.Println("[AmigaCD32] cfg bind-mount done")

	// 6. Build minimal MGL (no <file>)
	mgl := `<mistergamedescription>
	<rbf>_computer/minimig</rbf>
	<setname same_dir="1">AmigaCD32</setname>
</mistergamedescription>`

	tmpMgl := config.LastLaunchFile
	fmt.Printf("[AmigaCD32] Writing MGL to %s\n", tmpMgl)
	if err := os.WriteFile(tmpMgl, []byte(mgl), 0644); err != nil {
		return fmt.Errorf("failed to write MGL: %w", err)
	}
	fmt.Printf("[AmigaCD32] MGL contents:\n%s\n", mgl)
	fmt.Println("[AmigaCD32] MGL written successfully")

	// 7. Launch it directly
	fmt.Printf("[AmigaCD32] Launching with MGL: %s\n", tmpMgl)
	if err := launchFile(tmpMgl); err != nil {
		return fmt.Errorf("failed to launch AmigaCD32 MGL: %w", err)
	}

	fmt.Println("[SIDELAUNCHER] AmigaCD32 game launched successfully!")
	return nil
}
