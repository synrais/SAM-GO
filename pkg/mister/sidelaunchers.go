package mister

import (
	"fmt"
	"strings"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/games"
)

// SideLaunchers lets you plug in system-specific launch rules.
// Returns (handled, err):
//   - handled = true → we launched the game, don't fall back.
//   - handled = false → fall back to standard LaunchGame flow.
func SideLaunchers(cfg *config.UserConfig, system games.System, path string) (bool, error) {
	switch strings.ToLower(system.Id) {

	// Example: Amiga special handling
	case "amiga":
		fmt.Printf("[SideLaunchers] Custom Amiga handling for %s\n", path)
		err := launchTempMgl(cfg, &system, path)
		return true, err

	// Example: NeoGeo special handling
	case "neogeo":
		fmt.Printf("[SideLaunchers] Custom NeoGeo handling for %s\n", path)
		err := launchTempMgl(cfg, &system, path)
		return true, err

	// Add more system IDs here as needed.
	// case "saturn":
	//     return true, customSaturnLaunch(cfg, &system, path)

	default:
		// no special handling → let LaunchGame do its normal thing
		return false, nil
	}
}

// helper: build temporary MGL and launch it
func launchTempMgl(cfg *config.UserConfig, system *games.System, path string) error {
	override, err := games.RunSystemHook(cfg, *system, path)
	if err != nil {
		return err
	}

	mgl, err := GenerateMgl(cfg, system, path, override)
	if err != nil {
		return err
	}

	tmpFile, err := writeTempFile(mgl)
	if err != nil {
		return err
	}

	return launchFile(tmpFile)
}
