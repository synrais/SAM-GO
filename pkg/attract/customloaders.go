package attract

import (
	"fmt"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/games"
	"github.com/synrais/SAM-GO/pkg/mister"
)

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
	// Example: Custom loader for AmigaCD32
	RegisterCustomLoader("AMIGACD32", func(system *games.System, runPath string) error {
		fmt.Println("[AMIGACD32] Custom loader starting…")

		// ✅ Pre-launch hook
		// (e.g. copy BIOS, mount ISO, tweak config)
		fmt.Println("[AMIGACD32] Preparing CD-ROM boot…")

		// Normal game launch
		if err := mister.LaunchGame(&config.UserConfig{}, system, runPath); err != nil {
			return fmt.Errorf("failed to launch AmigaCD32: %w", err)
		}

		// ✅ Post-launch hook
		fmt.Println("[AMIGACD32] Game launched successfully!")

		return nil
	})
}
