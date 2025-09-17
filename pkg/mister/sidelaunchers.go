package mister

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/games"
)

// sidelauncherRegistry maps systemId → special launcher
var sidelauncherRegistry = map[string]func(cfg *config.UserConfig, system games.System, path string) error{}

// registerSidelauncher adds a new special handler.
func registerSidelauncher(systemID string, fn func(cfg *config.UserConfig, system games.System, path string) error) {
	sidelauncherRegistry[systemID] = fn
}

// SideLaunchers checks the registry for a matching handler.
func SideLaunchers(cfg *config.UserConfig, system games.System, path string) (bool, error) {
	if fn, ok := sidelauncherRegistry[system.Id]; ok {
		return true, fn(cfg, system, path)
	}
	return false, nil
}

//
// --- AmigaVision Loader ---
//
func amigaVisionLoader(cfg *config.UserConfig, system games.System, path string) error {
	amigaShared := findAmigaShared()
	if amigaShared == "" {
		return fmt.Errorf("AmigaVision: shared folder not found")
	}

	tmpShared := "/tmp/.SAM_tmp/Amiga_shared"
	_ = os.RemoveAll(tmpShared)
	_ = os.MkdirAll(tmpShared, 0755)

	// copy real shared into tmp
	if out, err := exec.Command("/bin/cp", "-a", amigaShared+"/.", tmpShared).CombinedOutput(); err != nil {
		fmt.Printf("[WARN] AmigaVision copy shared failed: %v (output: %s)\n", err, string(out))
	}

	// write ags_boot file with the .amiv filename (basename only)
	bootFile := filepath.Join(tmpShared, "ags_boot")
	content := filepath.Base(path) + "\n\n"
	if err := os.WriteFile(bootFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("AmigaVision: failed to write ags_boot: %v", err)
	}

	// bind mount over real shared
	unmount(amigaShared)
	if err := bindMount(tmpShared, amigaShared); err != nil {
		return fmt.Errorf("AmigaVision: bind mount failed: %v", err)
	}

	// launch Amiga core normally
	return LaunchCore(cfg, system)
}

//
// --- CD32 Placeholder ---
//
func cd32Loader(cfg *config.UserConfig, system games.System, path string) error {
	fmt.Printf("[CD32] Placeholder loader for: %s\n", path)
	time.Sleep(1 * time.Second)
	return LaunchGame(cfg, system, path)
}

//
// --- Init registry ---
//
func init() {
	registerSidelauncher("Amiga", amigaVisionLoader)
	registerSidelauncher("CD32", cd32Loader)
}
