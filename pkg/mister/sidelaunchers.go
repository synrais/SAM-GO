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

func init() {
	registerSideLauncher("AmigaVision", LaunchAmigaVision)
	registerSideLauncher("AmigaCD32", LaunchCD32)
}

func LaunchAmigaVision(cfg *config.UserConfig, system games.System, path string) error {
	// Only handle .ags files
	if !strings.EqualFold(filepath.Ext(path), ".ags") {
		return nil
	}

	// --- Local helpers (scoped only to AmigaVision) ---
	findAmigaShared := func() string {
		paths := games.GetSystemPaths(cfg, []games.System{system})
		for _, p := range paths {
			candidate := filepath.Join(p.Path, "shared")
			if st, err := os.Stat(candidate); err == nil && st.IsDir() {
				return candidate
			}
		}
		return ""
	}

	unmount := func(path string) {
		_ = exec.Command("umount", path).Run()
	}

	bindMount := func(src, dst string) error {
		_ = os.MkdirAll(dst, 0755)
		cmd := exec.Command("mount", "--bind", src, dst)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("bind mount failed: %v (output: %s)", err, string(out))
		}
		return nil
	}
	// --------------------------------------------------

	// Locate the Amiga shared folder
	amigaShared := findAmigaShared()
	if amigaShared == "" {
		return fmt.Errorf("games/%s/shared folder not found", system.Id)
	}

	// Prepare tmp shared dir
	tmpShared := "/tmp/.SAM_tmp/Amiga_shared"
	_ = os.RemoveAll(tmpShared)
	_ = os.MkdirAll(tmpShared, 0755)

	// Copy existing shared into tmp
	if out, err := exec.Command("/bin/cp", "-a", amigaShared+"/.", tmpShared).CombinedOutput(); err != nil {
		fmt.Printf("[WARN] copy shared failed: %v (output: %s)\n", err, string(out))
	}

	// Write ags_boot file with the clean name
	cleanName := utils.RemoveFileExt(filepath.Base(path))
	bootFile := filepath.Join(tmpShared, "ags_boot")
	content := cleanName + "\n\n"
	if err := os.WriteFile(bootFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write ags_boot: %v", err)
	}

	// Bind mount tmp over real shared
	unmount(amigaShared)
	if err := bindMount(tmpShared, amigaShared); err != nil {
		return err
	}

	// Launch the Amiga core with this system
	return LaunchCore(cfg, system)
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
		offsetGamePath  = 0xC1C  // game path
		fieldLength     = 256    // adjust if fields are smaller/larger
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

	// 2. Locate system folder(s)
	sysPaths := games.GetSystemPaths(cfg, []games.System{system})
	if len(sysPaths) == 0 {
		return fmt.Errorf("[AmigaCD32] No valid system paths found for %s", system.Name)
	}
	pseudoRoot := sysPaths[0].Path
	fmt.Printf("[AmigaCD32] Using system folder: %s\n", pseudoRoot)

	// 3. Ensure cfg file exists on FAT (only as a bind target)
	misterCfg := "/media/fat/config/AmigaCD32.cfg"
	if _, err := os.Stat(misterCfg); os.IsNotExist(err) {
		fmt.Printf("[AmigaCD32] No existing cfg at %s, writing blank one\n", misterCfg)
		if err := os.WriteFile(misterCfg, assets.BlankAmigaCD32Cfg, 0644); err != nil {
			return fmt.Errorf("failed to write initial AmigaCD32.cfg: %w", err)
		}
	}

	// 4. Create tmp cfg from embedded template
	tmpCfg := filepath.Join(tmpDir, "AmigaCD32.cfg")
	data := make([]byte, len(assets.BlankAmigaCD32Cfg))
	copy(data, assets.BlankAmigaCD32Cfg)

	// --- Game path ---
	absGame, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve game path: %w", err)
	}
	absGame = strings.TrimPrefix(absGame, "/media/")
	if err := patchAt(data, offsetGamePath, absGame); err != nil {
		return err
	}

	// --- ROM: prefer existing AmigaVision.rom, otherwise write embedded ---
	romPath := filepath.Join(pseudoRoot, "AmigaVision.rom")
	if _, err := os.Stat(romPath); err == nil {
		fmt.Printf("[AmigaCD32] Using existing AmigaVision ROM: %s\n", romPath)
	} else {
		fmt.Printf("[AmigaCD32] No AmigaVision ROM found, writing embedded ROM to %s\n", romPath)
		if err := os.WriteFile(romPath, assets.AmigaVisionRom, 0644); err != nil {
			return fmt.Errorf("failed to write embedded ROM: %w", err)
		}
	}
	if err := patchAt(data, offsetRomPath, cleanPath(romPath)); err != nil {
		return err
	}

	// --- HDF: prefer existing AmigaCD32.hdf, otherwise write embedded ---
	hdfPath := filepath.Join(pseudoRoot, "AmigaCD32.hdf")
	if _, err := os.Stat(hdfPath); err == nil {
		fmt.Printf("[AmigaCD32] Using existing AmigaCD32 HDF: %s\n", hdfPath)
	} else {
		fmt.Printf("[AmigaCD32] No AmigaCD32 HDF found, writing embedded HDF to %s\n", hdfPath)
		if err := os.WriteFile(hdfPath, assets.AmigaCD32Hdf, 0644); err != nil {
			return fmt.Errorf("failed to write embedded HDF: %w", err)
		}
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
