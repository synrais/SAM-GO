package mister

import (
	"bytes"
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

func LaunchCD32(cfg *config.UserConfig, system games.System, path string) error {
	fmt.Println("[SIDELAUNCHER] AmigaCD32 launch starting…")

	// --- Local helpers ---
	unmount := func(p string) {
		fmt.Printf("[AmigaCD32] Attempting unmount of %s\n", p)
		if out, err := exec.Command("umount", p).CombinedOutput(); err != nil {
			fmt.Printf("[AmigaCD32] WARN: umount failed for %s: %v (output: %s)\n", p, err, string(out))
		}
	}

	bindMount := func(src, dst string) error {
		fmt.Printf("[AmigaCD32] bindMount start: %s -> %s\n", src, dst)
		_ = os.MkdirAll(filepath.Dir(dst), 0755)
		cmd := exec.Command("mount", "--bind", src, dst)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("[AmigaCD32] ERROR bindMount failed: %s -> %s\nOutput: %s", src, dst, string(out))
		}
		fmt.Printf("[AmigaCD32] bindMount ok: %s -> %s\n", src, dst)
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

	// 2. Locate system folder
	sysPaths := games.GetSystemPaths(cfg, []games.System{system})
	if len(sysPaths) == 0 {
		return fmt.Errorf("[AmigaCD32] ERROR: no system paths found for %s", system.Id)
	}
	pseudoRoot := sysPaths[0].Path
	fmt.Printf("[AmigaCD32] Using pseudoRoot = %s\n", pseudoRoot)

	// 3. Find AmigaVision.rom
	visionRom := filepath.Join(pseudoRoot, "AmigaVision.rom")
	if _, err := os.Stat(visionRom); err != nil {
		return fmt.Errorf("[AmigaCD32] ERROR: required file missing: %s", visionRom)
	}
	fmt.Printf("[AmigaCD32] Found AmigaVision.rom: %s\n", visionRom)

	// 4. Ensure AmigaCD32.hdf exists (copy embedded if missing)
	cd32hdf := filepath.Join(pseudoRoot, "AmigaCD32.hdf")
	if _, err := os.Stat(cd32hdf); os.IsNotExist(err) {
		fmt.Printf("[AmigaCD32] AmigaCD32.hdf not found, writing embedded copy to %s\n", cd32hdf)
		if err := os.WriteFile(cd32hdf, assets.AmigaCD32Hdf, 0644); err != nil {
			return fmt.Errorf("failed to create AmigaCD32.hdf: %w", err)
		}
	} else {
		fmt.Printf("[AmigaCD32] Found AmigaCD32.hdf: %s\n", cd32hdf)
	}

	// 5. Find optional save file
	var saveFile string
	candidate := filepath.Join(pseudoRoot, "AmigaVision-Saves.hdf")
	if _, err := os.Stat(candidate); err == nil {
		saveFile = candidate
		fmt.Printf("[AmigaCD32] Found save file: %s\n", saveFile)
	} else {
		fmt.Println("[AmigaCD32] No save file found")
	}

	// 6. Ensure real config file exists
	misterCfg := "/media/fat/config/AmigaCD32.cfg"
	if _, err := os.Stat(misterCfg); os.IsNotExist(err) {
		fmt.Printf("[AmigaCD32] No AmigaCD32.cfg in config, writing blank\n")
		if err := os.WriteFile(misterCfg, assets.BlankAmigaCD32Cfg, 0644); err != nil {
			return fmt.Errorf("failed to create base AmigaCD32.cfg: %w", err)
		}
	} else {
		fmt.Printf("[AmigaCD32] Found existing config: %s\n", misterCfg)
	}

	// 7. Patch tmp cfg
	tmpCfg := filepath.Join(tmpDir, "AmigaCD32.cfg")
	if err := os.WriteFile(tmpCfg, assets.BlankAmigaCD32Cfg, 0644); err != nil {
		return fmt.Errorf("failed to write base tmp cfg: %w", err)
	}
	fmt.Printf("[AmigaCD32] Base cfg written to %s\n", tmpCfg)

	absGame, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve game path: %w", err)
	}
	if strings.HasPrefix(absGame, "/media/") {
		absGame = absGame[len("/media/"):]
	}
	fmt.Printf("[AmigaCD32] Patching game path = %s\n", absGame)

	data, err := os.ReadFile(tmpCfg)
	if err != nil {
		return err
	}

	patch := func(marker, replacement string) error {
		idx := bytes.Index(data, []byte(marker))
		if idx == -1 {
			fmt.Printf("[AmigaCD32] marker %q not found in cfg\n", marker)
			return nil
		}
		if idx+len(replacement) > len(data) {
			return fmt.Errorf("%s replacement too long", marker)
		}
		copy(data[idx:], []byte(replacement))
		fmt.Printf("[AmigaCD32] Patched %s -> %s\n", marker, replacement)
		return nil
	}

	if err := patch("gamepath.ext", absGame); err != nil {
		return err
	}
	if err := patch("/AGS.rom", visionRom); err != nil {
		return err
	}
	if err := patch("/CD32.hdf", cd32hdf); err != nil {
		return err
	}
	if saveFile != "" {
		if err := patch("/AGS-SAVES.hdf", saveFile); err != nil {
			return err
		}
	}

	if err := os.WriteFile(tmpCfg, data, 0644); err != nil {
		return fmt.Errorf("failed to save patched cfg: %w", err)
	}
	fmt.Printf("[AmigaCD32] Patched cfg written to %s\n", tmpCfg)

	// dump first 64 bytes for debug
	fmt.Printf("[AmigaCD32] cfg head dump: %q\n", string(data[:64]))

	// 8. Bind-mount tmp cfg over real config
	unmount(misterCfg)
	if err := bindMount(tmpCfg, misterCfg); err != nil {
		return err
	}
	fmt.Println("[AmigaCD32] cfg bind-mount done")

	// 9. Build MGL
	mgl := `<mistergamedescription>
	<rbf>_computer/minimig</rbf>
	<setname same_dir="1">AmigaCD32</setname>
</mistergamedescription>`

	tmpMgl := config.LastLaunchFile
	fmt.Printf("[AmigaCD32] Writing MGL to %s\n", tmpMgl)
	if err := os.WriteFile(tmpMgl, []byte(mgl), 0644); err != nil {
		return fmt.Errorf("failed to write MGL: %w", err)
	}
	fmt.Println("[AmigaCD32] MGL written successfully")

	// 10. Launch
	fmt.Printf("[AmigaCD32] Launching with MGL: %s\n", tmpMgl)
	if err := LaunchGenericFile(cfg, tmpMgl); err != nil {
		return fmt.Errorf("failed to launch AmigaCD32 MGL: %w", err)
	}

	fmt.Println("[SIDELAUNCHER] AmigaCD32 game launched successfully!")
	return nil
}
