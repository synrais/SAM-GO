package attract

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
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
		return true, fn(&system, runPath) // pass pointer inside
	}
	return false, nil
}

func init() {
	// 🔹 match systems.go: Id: "AmigaCD32"
	RegisterCustomLoader("AmigaCD32", func(system *games.System, runPath string) error {
		fmt.Println("[AmigaCD32] Custom loader starting…")

		// --- Prepare temp working dir ---
		tmpRoot := "/tmp/.SAM_tmp/AmigaCD32"
		if err := os.MkdirAll(tmpRoot, 0755); err != nil {
			return fmt.Errorf("failed to create tmp dir: %w", err)
		}

		// --- Write embedded blank cfg to tmp ---
		tmpCfg := filepath.Join(tmpRoot, "AmigaCD32.cfg")
		if err := os.WriteFile(tmpCfg, assets.BlankAmigaCD32Cfg, 0644); err != nil {
			return fmt.Errorf("failed to write temp cfg: %w", err)
		}
		fmt.Printf("[AmigaCD32] Base config written to %s\n", tmpCfg)

		// --- Copy bundled assets (HDF + ROM) into tmpRoot ---
		romPath := filepath.Join(tmpRoot, "AmigaVision.rom")
		if err := os.WriteFile(romPath, assets.AmigaVisionRom, 0644); err != nil {
			return fmt.Errorf("failed to write AmigaVision.rom: %w", err)
		}
		hdfPath := filepath.Join(tmpRoot, "AmigaCD32.hdf")
		if err := os.WriteFile(hdfPath, assets.AmigaCD32Hdf, 0644); err != nil {
			return fmt.Errorf("failed to write AmigaCD32.hdf: %w", err)
		}

		// --- Resolve game path ---
		absGame, err := filepath.Abs(runPath)
		if err != nil {
			return fmt.Errorf("failed to resolve game path: %w", err)
		}
		if strings.HasPrefix(absGame, "/media/") {
			absGame = absGame[len("/media/"):] // → "/usb0/games/…"
		}

		// --- Prepare patch set ---
		cfgData, err := os.ReadFile(tmpCfg)
		if err != nil {
			return err
		}

		patches := map[string]string{
			"gamepath.ext": absGame, // 🔹 game path
			"/AGS-SAVES.hdf": filepath.Join(tmpRoot, "AmigaVision-Saves.hdf"),
			"/CD32.hdf":      hdfPath,
			"/AGS.rom":       romPath,
		}

		for placeholder, replacement := range patches {
			idx := bytes.Index(cfgData, []byte(placeholder))
			if idx == -1 {
				continue // skip silently if not present
			}
			if idx+len(replacement) > len(cfgData) {
				return fmt.Errorf("replacement too long for placeholder %s", placeholder)
			}
			copy(cfgData[idx:], []byte(replacement))
			fmt.Printf("[AmigaCD32] Patched %s -> %s\n", placeholder, replacement)
		}

		if err := os.WriteFile(tmpCfg, cfgData, 0644); err != nil {
			return fmt.Errorf("failed to write patched cfg: %w", err)
		}

		// --- Bind mounts ---
		binds := [][2]string{
			{tmpCfg, "/media/fat/config/AmigaCD32.cfg"},
			{romPath, filepath.Join(tmpRoot, "AmigaVision.rom")},
			{hdfPath, filepath.Join(tmpRoot, "AmigaCD32.hdf")},
		}
		for _, pair := range binds {
			if err := exec.Command("mount", "--bind", pair[0], pair[1]).Run(); err != nil {
				return fmt.Errorf("bind mount failed %s -> %s: %w", pair[0], pair[1], err)
			}
			fmt.Printf("[AmigaCD32] Bind mounted %s -> %s\n", pair[0], pair[1])
		}

		// --- Build MGL ---
		mgl := `<mistergamedescription>
	<rbf>_computer/minimig</rbf>
	<setname same_dir="1">AmigaCD32</setname>
</mistergamedescription>`

		tmpMgl := config.LastLaunchFile
		if err := os.WriteFile(tmpMgl, []byte(mgl), 0644); err != nil {
			return fmt.Errorf("failed to write custom MGL: %w", err)
		}
		fmt.Printf("[AmigaCD32] Custom MGL written to %s\n", tmpMgl)

		// --- Launch ---
		if err := mister.LaunchGenericFile(&config.UserConfig{}, tmpMgl); err != nil {
			return fmt.Errorf("failed to launch AmigaCD32 MGL: %w", err)
		}

		fmt.Println("[AmigaCD32] Game launched successfully!")
		return nil
	})
}
