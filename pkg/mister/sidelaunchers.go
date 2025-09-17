package mister

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/games"
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

	// Use the same cleaning logic here (local to AmigaVision)
	base := filepath.Base(path)
	cleanName := strings.TrimSuffix(base, filepath.Ext(base))

	fmt.Printf("[SIDELAUNCHER] %s launching: %s\n", system.Id, cleanName)

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
	cleanFileName := func(p string) string {
		base := filepath.Base(p)
		return strings.TrimSuffix(base, filepath.Ext(base))
	}

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
	cleanName := cleanFileName(path)
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
// CD32
// --------------------------------------------------

func init() {
	registerSideLauncher("CD32", LaunchCD32)
}

func LaunchCD32(cfg *config.UserConfig, system games.System, path string) error {
	fmt.Println("[SIDELAUNCHER] CD32 placeholder launch:", path)
	// TODO: implement CD32 rules
	return LaunchCore(cfg, system)
}
