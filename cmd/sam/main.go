package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/synrais/SAM-GO/pkg/assets"
	"github.com/synrais/SAM-GO/pkg/attract"
	"github.com/synrais/SAM-GO/pkg/config"
)

const iniFileName = "SAM.ini"

// CLI flags
var (
	streamDebug = flag.Bool("s", false, "Enable static detector stream debug output")
	runPath     = flag.String("run", "", "Run a single game by path")
	menuMode    = flag.Bool("menu", false, "Launch interactive game browser menu")
)

func main() {
	// keep memory low on MiSTer
	debug.SetMemoryLimit(128 * 1024 * 1024) // 128MB soft limit
	flag.Parse()

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

	// --- Mode selection ---
	switch {
	case *runPath != "":
		// Direct run mode
		if err := attract.Run([]string{*runPath}); err != nil {
			fmt.Fprintln(os.Stderr, "[MAIN] Run error:", err)
			os.Exit(1)
		}

	default:
		// Attract mode (with optional -s stream debug)
		attract.PrepareAttractLists(cfg, *streamDebug)
	}
}
