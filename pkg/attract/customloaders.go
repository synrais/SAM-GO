package attract

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/synrais/SAM-GO/pkg/assets"
	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/games"
	"github.com/synrais/SAM-GO/pkg/mister"
)

// registry maps system IDs to their custom loader functions.
var registry = map[string]func(*games.System, string) error{}

// RegisterCustomLoader allows adding a loader for a specific system ID.
func RegisterCustomLoader(systemID string, fn func(*games.System, string) error) {
	registry[systemID] = fn
}

// TryCustomLoader checks if a system has a registered custom loader.
// Returns (true, error) if handled, (false, nil) if not).
func TryCustomLoader(system *games.System, runPath string) (bool, error) {
	if fn, ok := registry[system.ID]; ok {
		fmt.Printf("[CUSTOM] Using custom loader for %s\n", system.ID)
		return true, fn(system, runPath)
	}
	return false, nil
}

func init() {
	// 🔹 Full custom loader for AmigaCD32
	RegisterCustomLoader("AMIGACD32", func(system *games.System, runPath string) error {
		fmt.Println("[AMIGACD32] Custom loader starting…")

		// 1. Write the embedded blank config to /tmp
		tmpCfg := "/tmp/AmigaCD32.cfg"
		if err := os.WriteFile(tmpCfg, assets.BlankAmigaCD32Cfg, 0644); err != nil {
			return fmt.Errorf("failed to write temp cfg: %w", err)
		}
		fmt.Printf("[AMIGACD32] Wrote base config to %s\n", tmpCfg)

		// 2. Patch the cfg with the game path
		gamePath, err := filepath.Abs(runPath)
		if err != nil {
			return fmt.Errorf("failed to resolve runPath: %w", err)
		}
		if err := patchAmigaCD32Cfg(tmpCfg, gamePath); err != nil {
			return fmt.Errorf("failed to patch cfg: %w", err)
		}
		fmt.Printf("[AMIGACD32] Patched config with game %s\n", gamePath)

		// 3. Bind-mount the new cfg over MiSTer.ini (optional for now)
		_ = syscall.Mount(tmpCfg, "/media/fat/MiSTer.ini", "", syscall.MS_BIND, "")
		fmt.Println("[AMIGACD32] Bound cfg over MiSTer.ini")

		// 4. Launch AmigaCD32 core via MiSTer
		if err := mister.LaunchCore("/media/fat/_Console/AmigaCD32.rbf", tmpCfg); err != nil {
			return fmt.Errorf("failed to launch AmigaCD32 core: %w", err)
		}

		fmt.Println("[AMIGACD32] Game launched successfully!")
		return nil
	})
}

// patchAmigaCD32Cfg replaces the CD image line in the cfg with the given runPath
func patchAmigaCD32Cfg(cfgPath string, runPath string) error {
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}

	// Replace POI spot "gamepath.ext" with actual runPath
	newData := []byte{}
	for _, line := range bytes.Split(data, []byte("\n")) {
		if bytes.Contains(line, []byte("gamepath.ext")) {
			line = []byte(runPath)
		}
		newData = append(newData, line...)
		newData = append(newData, '\n')
	}

	return os.WriteFile(cfgPath, newData, 0644)
}
