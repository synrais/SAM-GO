package attract

import (
	"bytes"
	"fmt"
	"os"
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

func init() {
	RegisterCustomLoader("AmigaCD32", func(system *games.System, runPath string) error {
		fmt.Println("[AmigaCD32] Custom loader starting…")

		// 1. Write embedded blank cfg to /tmp
		tmpCfg := "/tmp/AmigaCD32.cfg"
		if err := os.WriteFile(tmpCfg, assets.BlankAmigaCD32Cfg, 0644); err != nil {
			return fmt.Errorf("failed to write temp cfg: %w", err)
		}
		fmt.Printf("[AmigaCD32] Base config written to %s\n", tmpCfg)

		// load cfg into memory for patching
		data, err := os.ReadFile(tmpCfg)
		if err != nil {
			return fmt.Errorf("failed to read cfg: %w", err)
		}

		// 2. Patch cfg with the actual game path
		absGame, err := filepath.Abs(runPath)
		if err != nil {
			return fmt.Errorf("failed to resolve game path: %w", err)
		}
		if strings.HasPrefix(absGame, "/media/") {
			absGame = absGame[len("/media/"):] // "/media/usb0/..." → "/usb0/..."
		}
		if idx := bytes.Index(data, []byte("gamepath.ext")); idx != -1 {
			if idx+len(absGame) <= len(data) {
				copy(data[idx:], []byte(absGame))
				fmt.Printf("[AmigaCD32] Patched game path: %s\n", absGame)
			}
		}

		// 2b. Look for AmigaVision-Saves.hdf in system folders
		for _, sysPath := range system.Paths {
			candidate := filepath.Join(sysPath, "AmigaVision-Saves.hdf")
			if _, err := os.Stat(candidate); err == nil {
				saveAbs, _ := filepath.Abs(candidate)
				if strings.HasPrefix(saveAbs, "/media/") {
					saveAbs = saveAbs[len("/media/"):]
				}
				if idx := bytes.Index(data, []byte("/AGS-SAVES.hdf")); idx != -1 {
					if idx+len(saveAbs) <= len(data) {
						copy(data[idx:], []byte(saveAbs))
						fmt.Printf("[AmigaCD32] Patched save HDF: %s\n", saveAbs)
					}
				}
				break
			}
		}

		// write patched cfg back out
		if err := os.WriteFile(tmpCfg, data, 0644); err != nil {
			return fmt.Errorf("failed to write patched cfg: %w", err)
		}

		// 3. Build the special MGL
		mgl := `<mistergamedescription>
	<rbf>_computer/minimig</rbf>
	<setname same_dir="1">AmigaCD32</setname>
</mistergamedescription>`

		// 4. Write MGL
		tmpMgl := config.LastLaunchFile
		if err := os.WriteFile(tmpMgl, []byte(mgl), 0644); err != nil {
			return fmt.Errorf("failed to write custom MGL: %w", err)
		}
		fmt.Printf("[AmigaCD32] Custom MGL written to %s\n", tmpMgl)

		// 5. Launch it
		if err := mister.LaunchGenericFile(&config.UserConfig{}, tmpMgl); err != nil {
			return fmt.Errorf("failed to launch AmigaCD32 MGL: %w", err)
		}

		fmt.Println("[AmigaCD32] Game launched successfully!")
		return nil
	})
}
