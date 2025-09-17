package mister

import (
	"fmt"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/games"
)

// SideLauncher provides system-specific overrides for launching.
// Return true if handled, false if not.
func SideLauncher(cfg *config.UserConfig, system games.System, path string) (bool, error) {
	switch system.Id {
	case "Amiga":
		// Example: always force a Kickstart injection override
		fmt.Println("[SideLauncher] Special handling for Amiga")
		override := `<file type="hardfile" path="../../../../../games/Amiga/Kickstart.rom"/>`
		mgl, err := GenerateMgl(cfg, &system, path, override)
		if err != nil {
			return true, err
		}
		tmpFile, err := writeTempFile(mgl)
		if err != nil {
			return true, err
		}
		return true, launchFile(tmpFile)

	case "NeoGeo":
		fmt.Println("[SideLauncher] Special handling for NeoGeo")
		// You could, for example, always preload a BIOS
		return true, launchTempMgl(cfg, &system, path)

	// Add new systems here with custom rules

	default:
		return false, nil
	}
}
