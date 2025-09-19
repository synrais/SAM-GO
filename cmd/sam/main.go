package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/synrais/SAM-GO/pkg/assets"
	"github.com/synrais/SAM-GO/pkg/attract"
	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/games"
)

const iniFileName = "SAM.ini"

// dumpConfig shows the loaded config in a dynamic, always up-to-date format.
func dumpConfig(cfg *config.UserConfig) {
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		fmt.Println("[MAIN] Failed to dump config:", err)
		return
	}
	fmt.Println("==== SAM Configuration ====")
	fmt.Println(string(out))
	fmt.Println("===========================")
}

func ensureIni() string {
	exePath, _ := os.Executable()
	iniPath := filepath.Join(filepath.Dir(exePath), iniFileName)

	if _, err := os.Stat(iniPath); os.IsNotExist(err) {
		fmt.Println("[MAIN] No INI found, generating from embedded defaults...")
		if err := os.WriteFile(iniPath, []byte(assets.DefaultSAMIni), 0644); err != nil {
			fmt.Fprintln(os.Stderr, "[MAIN] Failed to create default INI:", err)
			os.Exit(1)
		}
		fmt.Println("[MAIN] Generated default INI at", iniPath)
	} else {
		fmt.Println("[MAIN] Found INI at", iniPath)
	}
	return iniPath
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: SAM <command> [args]")
		os.Exit(1)
	}

	// Ensure INI exists or create one from embedded defaults
	iniPath := ensureIni()

	// Load config (with defaults)
	cfg, err := config.LoadUserConfig("SAM", config.UserConfig())
	if err != nil {
		fmt.Fprintln(os.Stderr, "[MAIN] Config load error:", err)
		os.Exit(1)
	}
	fmt.Println("[MAIN] Loaded config from:", iniPath)

	// Show config dynamically
	dumpConfig(cfg)

	// Handle commands
	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "-list":
		systemPaths := games.GetSystemPaths(cfg, games.AllSystems())
		if attract.CreateGamelists(cfg, config.GamelistDir(), systemPaths, false) == 0 {
			fmt.Fprintln(os.Stderr, "List failed: no games indexed")
		}
	case "-run":
		if err := attract.Run(args); err != nil {
			fmt.Fprintln(os.Stderr, "Run failed:", err)
		}
	case "-attract":
		attract.RunAttract(cfg, args)
	case "-back":
		if _, ok := attract.PlayBack(); !ok {
			fmt.Fprintln(os.Stderr, "Back failed: no previous game in history")
		}
	case "-next":
		if _, ok := attract.PlayNext(); !ok {
			fmt.Fprintln(os.Stderr, "Next failed: no next game in history")
		}
	default:
		fmt.Printf("Unknown command: %s\n", cmd)
	}
}
