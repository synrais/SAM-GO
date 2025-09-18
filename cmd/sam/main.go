package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/synrais/SAM-GO/pkg/assets"
	"github.com/synrais/SAM-GO/pkg/attract"
	"github.com/synrais/SAM-GO/pkg/config"
    "github.com/synrais/SAM-GO/pkg/games"
	"github.com/synrais/SAM-GO/pkg/input"
	"github.com/synrais/SAM-GO/pkg/run"
	"github.com/synrais/SAM-GO/pkg/staticdetector"
)

const socketPath = "/tmp/sam.sock"
const iniFileName = "SAM.ini"

// dumpConfig prints the parsed settings in a readable way
func dumpConfig(cfg *config.UserConfig) {
	fmt.Printf("INI Debug ->\n")

	// Attract
	fmt.Printf("  Attract:\n")
	fmt.Printf("    PlayTime=%s | Random=%v\n", cfg.Attract.PlayTime, cfg.Attract.Random)
	fmt.Printf("    Include=%v | Exclude=%v\n", cfg.Attract.Include, cfg.Attract.Exclude)
	fmt.Printf("    UseStaticDetector=%v\n", cfg.Attract.UseStaticDetector)

	// List
	fmt.Printf("  List:\n")
	fmt.Printf("    Exclude=%v\n", cfg.List.Exclude)
	fmt.Printf("    UseBlacklist=%v | UseStaticlist=%v | UseWhitelist=%v\n",
    cfg.List.UseBlacklist, cfg.List.UseStaticlist, cfg.List.UseWhitelist)


	// Input detector
	fmt.Printf("  InputDetector: Mouse=%v | Keyboard=%v | Joystick=%v\n",
		cfg.InputDetector.Mouse, cfg.InputDetector.Keyboard, cfg.InputDetector.Joystick)
	fmt.Printf("    KeyboardMap=%v\n", cfg.InputDetector.KeyboardMap)
	fmt.Printf("    MouseMap=%v\n", cfg.InputDetector.MouseMap)
	fmt.Printf("    JoystickMap=%v\n", cfg.InputDetector.JoystickMap)

	// Static detector (global)
	fmt.Printf("  StaticDetector:\n")
	fmt.Printf("    BlackThreshold=%.0f | StaticThreshold=%.0f | Grace=%.0f\n",
		cfg.StaticDetector.BlackThreshold, cfg.StaticDetector.StaticThreshold, cfg.StaticDetector.Grace)
	fmt.Printf("    SkipBlack=%v | WriteBlackList=%v | SkipStatic=%v | WriteStaticList=%v\n",
		cfg.StaticDetector.SkipBlack, cfg.StaticDetector.WriteBlackList,
		cfg.StaticDetector.SkipStatic, cfg.StaticDetector.WriteStaticList)

	// Static detector overrides
	if len(cfg.StaticDetector.Systems) > 0 {
		fmt.Println("    Overrides:")
		for sys, ov := range cfg.StaticDetector.Systems {
			var parts []string
			if ov.BlackThreshold != nil {
				parts = append(parts, fmt.Sprintf("BlackThreshold=%.0f", *ov.BlackThreshold))
			}
			if ov.StaticThreshold != nil {
				parts = append(parts, fmt.Sprintf("StaticThreshold=%.0f", *ov.StaticThreshold))
			}
			if ov.SkipBlack != nil {
				parts = append(parts, fmt.Sprintf("SkipBlack=%v", *ov.SkipBlack))
			}
			if ov.WriteBlackList != nil {
				parts = append(parts, fmt.Sprintf("WriteBlackList=%v", *ov.WriteBlackList))
			}
			if ov.SkipStatic != nil {
				parts = append(parts, fmt.Sprintf("SkipStatic=%v", *ov.SkipStatic))
			}
			if ov.WriteStaticList != nil {
				parts = append(parts, fmt.Sprintf("WriteStaticList=%v", *ov.WriteStaticList))
			}
			if ov.Grace != nil {
				parts = append(parts, fmt.Sprintf("Grace=%.0f", *ov.Grace))
			}

			if len(parts) > 0 {
				fmt.Printf("      %s -> %s\n", sys, strings.Join(parts, " | "))
			} else {
				fmt.Printf("      %s -> (no overrides)\n", sys)
			}
		}
	}

	// Disable rules
	if len(cfg.Disable) > 0 {
		fmt.Println("  Disable Rules:")
		for sys, rules := range cfg.Disable {
			var parts []string
			if len(rules.Folders) > 0 {
				parts = append(parts, fmt.Sprintf("Folders=%v", rules.Folders))
			}
			if len(rules.Files) > 0 {
				parts = append(parts, fmt.Sprintf("Files=%v", rules.Files))
			}
			if len(rules.Extensions) > 0 {
				parts = append(parts, fmt.Sprintf("Extensions=%v", rules.Extensions))
			}

			if len(parts) > 0 {
				fmt.Printf("    %s -> %s\n", sys, strings.Join(parts, " | "))
			} else {
				fmt.Printf("    %s -> (no rules)\n", sys)
			}
		}
	}
}

// splitCommands splits args into multiple command slices separated by "--".
func splitCommands(args []string) [][]string {
	var cmds [][]string
	current := []string{}
	for _, a := range args {
		if a == "--" {
			if len(current) > 0 {
				cmds = append(cmds, current)
				current = []string{}
			}
			continue
		}
		current = append(current, a)
	}
	if len(current) > 0 {
		cmds = append(cmds, current)
	}
	return cmds
}

func handleCommand(cfg *config.UserConfig, cmd string, args []string, skipCh chan struct{}) {
	switch cmd {
	case "-list":
    	systemPaths := games.GetSystemPaths(cfg, games.AllSystems())
    	if attract.CreateGamelists(cfg, config.GamelistDir(), systemPaths, false) == 0 {
        	fmt.Fprintln(os.Stderr, "List failed: no games indexed")
    	}
	case "-run":
		if err := run.Run(args); err != nil {
			fmt.Fprintln(os.Stderr, "Run failed:", err)
		}
	case "-attract":
		attract.Run(cfg, args)
	case "-back":
		if _, ok := attract.PlayBack(); !ok {
			fmt.Fprintln(os.Stderr, "Back failed: no previous game in history")
		}
	case "-next":
		if _, ok := attract.PlayNext(); !ok {
			fmt.Fprintln(os.Stderr, "Next failed: no next game in history")
		}
	case "-mouse":
		for line := range input.StreamMouse() {
			fmt.Println(line)
		}
	case "-joystick":
		for line := range input.StreamJoysticks() {
			fmt.Println(line)
		}
	case "-keyboard":
		for line := range input.StreamKeyboards() {
			fmt.Println(line)
		}
	case "-static":
		for ev := range staticdetector.Stream(cfg, skipCh) {
			fmt.Println(ev)
		}
	default:
		fmt.Printf("Unknown tool: %s\n", cmd)
	}
}

func commandProcessor(cfg *config.UserConfig, ch <-chan []string, skipCh chan struct{}) {
	for args := range ch {
		if len(args) == 0 {
			continue
		}
		go handleCommand(cfg, args[0], args[1:], skipCh)
	}
}

func handleConnection(conn net.Conn, ch chan<- []string) {
	defer conn.Close()
	data, err := io.ReadAll(conn)
	if err != nil || len(data) == 0 {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		ch <- strings.Split(line, "\x00")
	}
}

func startServer(ch chan<- []string) {
	os.Remove(socketPath)
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "listen error:", err)
		os.Exit(1)
	}

	go func() {
		defer ln.Close()
		for {
			conn, err := ln.Accept()
			if err != nil {
				continue
			}
			go handleConnection(conn, ch)
		}
	}()
}

func sendToRunningInstance(cmds [][]string) bool {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return false
	}
	defer conn.Close()
	parts := make([]string, 0, len(cmds))
	for _, args := range cmds {
		parts = append(parts, strings.Join(args, "\x00"))
	}
	fmt.Fprint(conn, strings.Join(parts, "\n"))
	return true
}

func main() {
	// Restrict the Go runtime heap to reduce the overall virtual memory footprint.
	debug.SetMemoryLimit(128 * 1024 * 1024) // 128MB soft limit

	if len(os.Args) < 2 {
		fmt.Println("Usage: SAM <command> [flags] [-- <command> [flags] ...]")
		os.Exit(1)
	}

	cmds := splitCommands(os.Args[1:])

	if sendToRunningInstance(cmds) {
		return
	}

	// INI path lives next to executable
	exePath, _ := os.Executable()
	iniPath := filepath.Join(filepath.Dir(exePath), iniFileName)

	// Generate INI if missing
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

	// Dump settings
	dumpConfig(cfg)

	// Start command server
	commandChan := make(chan []string)
	skipCh := make(chan struct{}, 1) // shared skip channel for static detector
	startServer(commandChan)
	defer os.Remove(socketPath)

	go commandProcessor(cfg, commandChan, skipCh)
	for _, cmd := range cmds {
		commandChan <- cmd
	}

	select {}
}
