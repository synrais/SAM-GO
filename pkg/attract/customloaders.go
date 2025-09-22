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

		// helper: patch any placeholder
		patchCfg := func(cfgPath, placeholder, newValue string) error {
			data, err := os.ReadFile(cfgPath)
			if err != nil {
				return err
			}
			idx := bytes.Index(data, []byte(placeholder))
			if idx == -1 {
				return fmt.Errorf("placeholder %q not found in cfg", placeholder)
			}
			if idx+len(newValue) > len(data) {
				return fmt.Errorf("%s too long for cfg (would exceed file size)", placeholder)
			}
			copy(data[idx:], []byte(newValue))
			return os.WriteFile(cfgPath, data, 0644)
		}

		// 2. Patch cfg with the actual CHD path
		absGame, err := filepath.Abs(runPath)
		if err != nil {
			return fmt.Errorf("failed to resolve game path: %w", err)
		}
		if strings.HasPrefix(absGame, "/media/") {
			absGame = absGame[len("/media/"):]
		}
		if err := patchCfg(tmpCfg, "gamepath.ext", absGame); err != nil {
			return fmt.Errorf("failed to patch cfg with game: %w", err)
		}
		fmt.Printf("[AmigaCD32] Patched config with game %s\n", absGame)

		// 3. Look for AmigaVision-Saves.hdf in the system’s folder
		paths := games.GetActiveSystemPaths(&config.UserConfig{}, []games.System{*system})
		for _, pr := range paths {
			if pr.System.Id != "AmigaCD32" {
				continue
			}
			saveFile := filepath.Join(pr.Path, "AmigaVision-Saves.hdf")
			if _, err := os.Stat(saveFile); err == nil {
				savePath := saveFile
				if strings.HasPrefix(savePath, "/media/") {
					savePath = savePath[len("/media/"):]
				}
				if err := patchCfg(tmpCfg, "AGS-SAVES.hdf", savePath); err != nil {
					return fmt.Errorf("failed to patch cfg with savefile: %w", err)
				}
				fmt.Printf("[AmigaCD32] Added savefile mapping: %s\n", savePath)
				break
			}
		}

		// 4. Build the special MGL
		mgl := `<mistergamedescription>
	<rbf>_computer/minimig</rbf>
	<setname same_dir="1">AmigaCD32</setname>
</mistergamedescription>`

		// 5. Write it to the same tmp file normal launchers use
		tmpMgl := config.LastLaunchFile
		if err := os.WriteFile(tmpMgl, []byte(mgl), 0644); err != nil {
			return fmt.Errorf("failed to write custom MGL: %w", err)
		}
		fmt.Printf("[AmigaCD32] Custom MGL written to %s\n", tmpMgl)

		// 6. Launch it
		if err := mister.LaunchGenericFile(&config.UserConfig{}, tmpMgl); err != nil {
			return fmt.Errorf("failed to launch AmigaCD32 MGL: %w", err)
		}

		fmt.Println("[AmigaCD32] Game launched successfully!")
		return nil
	})
}
