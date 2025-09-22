package attract

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
	"github.com/synrais/SAM-GO/pkg/mister"
)

// registry holds all custom loaders keyed by system ID
var registry = map[string]func(*games.System, string) error{}

// RegisterCustomLoader allows adding a loader for a specific system ID.
func RegisterCustomLoader(systemID string, fn func(*games.System, string) error) {
	registry[systemID] = fn
}

// TryCustomLoader checks if a system has a registered custom loader.
// Returns (true, error) if handled, (false, nil) if not.
func TryCustomLoader(system games.System, runPath string) (bool, error) {
	if fn, ok := registry[system.Id]; ok {
		fmt.Printf("[CUSTOM] Using custom loader for %s\n", system.Id)
		return true, fn(&system, runPath)
	}
	return false, nil
}

// patchAmigaCD32Cfg overwrites the placeholders with actual paths.
func patchAmigaCD32Cfg(cfgPath string, gamePath string, romPath string, hdfPath string, savePath string) error {
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}

	placeholders := map[string]string{
		"gamepath.ext": gamePath,
		"/AGS.rom":     romPath,
		"/CD32.hdf":    hdfPath,
		"/AGS-SAVES.hdf": savePath,
	}

	for ph, val := range placeholders {
		if val == "" {
			continue
		}
		idx := bytes.Index(data, []byte(ph))
		if idx == -1 {
			continue
		}
		if idx+len(val) > len(data) {
			return fmt.Errorf("replacement for %s too long in cfg", ph)
		}
		copy(data[idx:], []byte(val))
	}

	return os.WriteFile(cfgPath, data, 0644)
}

func init() {
	RegisterCustomLoader("AmigaCD32", func(system *games.System, runPath string) error {
		fmt.Println("[AmigaCD32] Custom loader starting…")

		// 1. Prep temp folder
		tmpDir := "/tmp/.SAM_tmp/AmigaCD32"
		if err := os.MkdirAll(tmpDir, 0755); err != nil {
			return fmt.Errorf("failed to make tmp dir: %w", err)
		}

		// 2. Extract embedded files into tmp
		romTmp := filepath.Join(tmpDir, "AmigaVision.rom")
		hdfTmp := filepath.Join(tmpDir, "AmigaCD32.hdf")
		if err := os.WriteFile(romTmp, assets.AmigaVisionRom, 0644); err != nil {
			return fmt.Errorf("failed to write ROM: %w", err)
		}
		if err := os.WriteFile(hdfTmp, assets.AmigaCD32Hdf, 0644); err != nil {
			return fmt.Errorf("failed to write HDF: %w", err)
		}

		// 3. Locate system folder
		cfg := &config.UserConfig{}
		sysPaths := games.GetActiveSystemPaths(cfg, []games.System{*system})
		if len(sysPaths) == 0 {
			return fmt.Errorf("could not resolve AmigaCD32 system folder")
		}
		sysFolder := sysPaths[0].Path
		fmt.Printf("[AmigaCD32] System folder resolved: %s\n", sysFolder)

		// 4. Prepare bind-mount targets inside /media
		romTarget := filepath.Join(sysFolder, "AGS.rom")
		hdfTarget := filepath.Join(sysFolder, "CD32.hdf")

		// 5. Bind mount extracted files
		mount := func(src, dst string) error {
			if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
				return err
			}
			cmd := exec.Command("mount", "--bind", src, dst)
			if out, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("mount failed: %v, %s", err, string(out))
			}
			return nil
		}
		if err := mount(romTmp, romTarget); err != nil {
			return err
		}
		if err := mount(hdfTmp, hdfTarget); err != nil {
			return err
		}
		fmt.Println("[AmigaCD32] Bind mounts ready")

		// 6. Look for saves HDF
		saveTarget := ""
		saveCandidate := filepath.Join(sysFolder, "AmigaVision-Saves.hdf")
		if _, err := os.Stat(saveCandidate); err == nil {
			saveTarget = filepath.Join("/usb0", strings.TrimPrefix(saveCandidate, "/media/usb0/"))
			fmt.Printf("[AmigaCD32] Found saves HDF: %s\n", saveTarget)
		}

		// 7. Write blank cfg to tmp and patch
		tmpCfg := "/tmp/AmigaCD32.cfg"
		if err := os.WriteFile(tmpCfg, assets.BlankAmigaCD32Cfg, 0644); err != nil {
			return fmt.Errorf("failed to write blank cfg: %w", err)
		}

		gamePath := runPath
		if strings.HasPrefix(gamePath, "/media/") {
			gamePath = gamePath[len("/media"):]
		}
		romPath := strings.Replace(romTarget, "/media", "", 1)
		hdfPath := strings.Replace(hdfTarget, "/media", "", 1)

		if err := patchAmigaCD32Cfg(tmpCfg, gamePath, romPath, hdfPath, saveTarget); err != nil {
			return err
		}
		fmt.Printf("[AmigaCD32] Config patched at %s\n", tmpCfg)

		// 8. Write custom MGL
		mgl := `<mistergamedescription>
	<rbf>_computer/minimig</rbf>
	<setname same_dir="1">AmigaCD32</setname>
</mistergamedescription>`
		tmpMgl := config.LastLaunchFile
		if err := os.WriteFile(tmpMgl, []byte(mgl), 0644); err != nil {
			return fmt.Errorf("failed to write MGL: %w", err)
		}
		fmt.Printf("[AmigaCD32] Custom MGL written to %s\n", tmpMgl)

		// 9. Launch via MGL
		if err := mister.LaunchGenericFile(cfg, tmpMgl); err != nil {
			return fmt.Errorf("failed to launch AmigaCD32: %w", err)
		}

		fmt.Println("[AmigaCD32] Game launched successfully!")
		return nil
	})
}
