package attract

import (
	"fmt"
	"os"

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
func TryCustomLoader(system *games.System, runPath string) (bool, error) {
	if fn, ok := registry[system.Id]; ok {
		fmt.Printf("[CUSTOM] Using custom loader for %s\n", system.Id)
		return true, fn(system, runPath)
	}
	return false, nil
}

// patchAmigaCD32Cfg overwrites the placeholder with the actual runPath.
func patchAmigaCD32Cfg(cfgPath string, runPath string) error {
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}

	placeholder := []byte("gamepath.ext")
	idx := bytes.Index(data, placeholder)
	if idx == -1 {
		return fmt.Errorf("placeholder %q not found in cfg", placeholder)
	}

	if idx+len(runPath) > len(data) {
		return fmt.Errorf("runPath too long for cfg (would exceed file size)")
	}

	// Overwrite directly
	copy(data[idx:], []byte(runPath))

	return os.WriteFile(cfgPath, data, 0644)
}

func init() {
	RegisterCustomLoader("AMIGACD32", func(system *games.System, runPath string) error {
		fmt.Println("[AMIGACD32] Custom loader starting…")

		// 1. Write embedded blank cfg to /tmp
		tmpCfg := "/tmp/AmigaCD32.cfg"
		if err := os.WriteFile(tmpCfg, assets.BlankAmigaCD32Cfg, 0644); err != nil {
			return fmt.Errorf("failed to write temp cfg: %w", err)
		}
		fmt.Printf("[AMIGACD32] Base config written to %s\n", tmpCfg)

		// 2. Patch cfg with the actual game path
		absGame, err := filepath.Abs(runPath)
		if err != nil {
			return fmt.Errorf("failed to resolve game path: %w", err)
		}
		if err := patchAmigaCD32Cfg(tmpCfg, absGame); err != nil {
			return fmt.Errorf("failed to patch cfg: %w", err)
		}
		fmt.Printf("[AMIGACD32] Patched config with %s\n", absGame)

		// 3. Build the special MGL
		mgl := `<mistergamedescription>
	<rbf>_computer/minimig</rbf>
	<setname same_dir="1">AmigaCD32</setname>
</mistergamedescription>`

		// 4. Write it to the same tmp file normal launchers use
		tmpMgl := config.LastLaunchFile
		if err := os.WriteFile(tmpMgl, []byte(mgl), 0644); err != nil {
			return fmt.Errorf("failed to write custom MGL: %w", err)
		}
		fmt.Printf("[AMIGACD32] Custom MGL written to %s\n", tmpMgl)

		// 5. Launch it
		if err := mister.LaunchGenericFile(&config.UserConfig{}, tmpMgl); err != nil {
			return fmt.Errorf("failed to launch AmigaCD32 MGL: %w", err)
		}

		fmt.Println("[AMIGACD32] Game launched successfully!")
		return nil
	})
}
