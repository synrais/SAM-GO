package mister

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/games"
	"github.com/synrais/SAM-GO/pkg/utils"
)

// --------------------------------------------------
// Registry of sidelaunchers (system.Id → handler)
// --------------------------------------------------

var sideLauncherRegistry = map[string]func(*config.UserConfig, games.System, string) error{}

// registerSideLauncher is called by each sidelauncher to self-register
func registerSideLauncher(id string, fn func(*config.UserConfig, games.System, string) error) {
	id = strings.ToLower(id)
	sideLauncherRegistry[id] = fn
}

// SideLaunchers checks if system.Id has a sidelauncher
func SideLaunchers(cfg *config.UserConfig, system games.System, path string) (bool, error) {
	if fn, ok := sideLauncherRegistry[strings.ToLower(system.Id)]; ok {
		return true, fn(cfg, system, path)
	}
	return false, nil
}

// --------------------------------------------------
// AmigaVision sidelauncher
// --------------------------------------------------

func init() {
	registerSideLauncher("AmigaVision", LaunchAmigaVision)
}

func LaunchAmigaVision(cfg *config.UserConfig, system games.System, path string) error {
	if !strings.EqualFold(filepath.Ext(path), ".amiv") {
		return nil
	}

	cleanName := utils.RemoveFileExt(filepath.Base(path))
	fmt.Println("[SideLauncher] AmigaVision launching:", cleanName)

	amigaShared := findAmigaShared()
	if amigaShared == "" {
		return fmt.Errorf("games/Amiga/shared folder not found")
	}

	tmpShared := "/tmp/.SAM_tmp/Amiga_shared"
	_ = os.RemoveAll(tmpShared)
	_ = os.MkdirAll(tmpShared, 0755)

	if out, err := exec.Command("/bin/cp", "-a", amigaShared+"/.", tmpShared).CombinedOutput(); err != nil {
		fmt.Printf("[WARN] copy shared failed: %v (output: %s)\n", err, string(out))
	}

	bootFile := filepath.Join(tmpShared, "ags_boot")
	content := cleanName + "\n\n"
	if err := os.WriteFile(bootFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write ags_boot: %v", err)
	}

	unmount(amigaShared)
	if err := bindMount(tmpShared, amigaShared); err != nil {
		return err
	}

	return LaunchCore(cfg, system)
}

// --------------------------------------------------
// CD32 sidelauncher (placeholder)
// --------------------------------------------------

func init() {
	registerSideLauncher("CD32", LaunchCD32)
}

func LaunchCD32(cfg *config.UserConfig, system games.System, path string) error {
	fmt.Println("[SideLauncher] CD32 placeholder launch:", path)
	return LaunchCore(cfg, system)
}
