package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/synrais/SAM-GO/pkg/assets"
	"github.com/synrais/SAM-GO/pkg/attract"
	"github.com/synrais/SAM-GO/pkg/config"
)

const iniFileName = "SAM.ini"

// main is the entrypoint for SAM.
// It ensures config exists, loads it, and then hands off to Attract Mode.
func main() {
	debug.SetMemoryLimit(128 * 1024 * 1024) // 128MB soft limit

	exePath, _ := os.Executable()
	iniPath := filepath.Join(filepath.Dir(exePath), iniFileName)

	// Ensure SAM.ini exists
	if _, err := os.Stat(iniPath); os.IsNotExist(err) {
		fmt.Println("[MAIN] No INI found, generating from embedded default...")
		if err := os.WriteFile(iniPath, []byte(assets.DefaultSAMIni), 0644); err != nil {
			fmt.Fprintln(os.Stderr, "[MAIN] Failed to create default INI:", err)
			os.Exit(1)
		}
		fmt.Println("[MAIN] Generated default INI at", iniPath)
	} else {
		fmt.Println("[MAIN] Found INI at", iniPath)
	}

	// Load config
	cfg, err := config.LoadUserConfig("SAM", &config.UserConfig{})
	if err != nil {
		fmt.Fprintln(os.Stderr, "[MAIN] Config load error:", err)
		os.Exit(1)
	}
	fmt.Println("[MAIN] Loaded config from:", cfg.IniPath)

	// Hand off directly to attract mode
	attract.PrepareAttract(cfg)
}
