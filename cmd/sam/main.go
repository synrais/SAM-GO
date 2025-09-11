package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"runtime/debug"
	"strings"

	"github.com/synrais/SAM-GO/pkg/attract"
	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/input"
	"github.com/synrais/SAM-GO/pkg/list"
	"github.com/synrais/SAM-GO/pkg/run"
	"github.com/synrais/SAM-GO/pkg/staticdetector"
)

const socketPath = "/tmp/sam.sock"

func dumpConfig(cfg *config.UserConfig) {
	fmt.Printf("INI Debug ->\n")
	fmt.Printf("  Attract: Systems=%v | PlayTime=%s | Random=%v\n",
		cfg.Attract.Systems, cfg.Attract.PlayTime, cfg.Attract.Random)
	fmt.Printf("  List: Exclude=%v\n", cfg.List.Exclude)
	fmt.Printf("  Search: Filter=%v | Sort=%s\n", cfg.Search.Filter, cfg.Search.Sort)
	fmt.Printf("  LastPlayed: Name=%s | DisableLastPlayed=%v | RecentFolder=%s | DisableRecentFolder=%v\n",
		cfg.LastPlayed.Name, cfg.LastPlayed.DisableLastPlayed,
		cfg.LastPlayed.RecentFolderName, cfg.LastPlayed.DisableRecentFolder)
	fmt.Printf("  Remote: Mdns=%v | SyncSSHKeys=%v | CustomLogo=%s | AnnounceGameUrl=%s\n",
		cfg.Remote.MdnsService, cfg.Remote.SyncSSHKeys,
		cfg.Remote.CustomLogo, cfg.Remote.AnnounceGameUrl)
	fmt.Printf("  NFC: ConnStr=%s | AllowCommands=%v | DisableSounds=%v | Probe=%v\n",
		cfg.Nfc.ConnectionString, cfg.Nfc.AllowCommands,
		cfg.Nfc.DisableSounds, cfg.Nfc.ProbeDevice)
	fmt.Printf("  Systems: GamesFolder=%v | SetCore=%v\n",
		cfg.Systems.GamesFolder, cfg.Systems.SetCore)

	if len(cfg.Disable) > 0 {
		fmt.Printf("  Disable Rules:\n")
		for sys, rules := range cfg.Disable {
			fmt.Printf("    %s -> Folders=%v | Files=%v | Extensions=%v\n",
				sys, rules.Folders, rules.Files, rules.Extensions)
		}
	}
}

func handleCommand(cmd string, args []string) {	switch cmd {
	case "-list":
		list.Run(args)
	case "-run":
		if err := run.Run(args); err != nil {
			fmt.Fprintln(os.Stderr, "Run failed:", err)
		}
	case "-attract":
		attract.Run(args)
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
		for line := range staticdetector.Stream() {
			fmt.Println(line)
		}
	default:
		fmt.Printf("Unknown tool: %s\n", cmd)
			}
}

func commandProcessor(ch <-chan []string) {
	for args := range ch {
		if len(args) == 0 {
			continue
		}
		handleCommand(args[0], args[1:])
	}
}

func handleConnection(conn net.Conn, ch chan<- []string) {
	defer conn.Close()
	data, err := io.ReadAll(conn)
	if err != nil {
		return
	}
	if len(data) == 0 {
		return
	}
	ch <- strings.Split(string(data), "\x00")
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

func sendToRunningInstance(args []string) bool {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return false
	}
	defer conn.Close()
	fmt.Fprint(conn, strings.Join(args, "\x00"))
	return true
}

func main() {
	// Restrict the Go runtime heap to reduce the overall virtual memory footprint.
	// This keeps the process from reserving excessively large address space by default.
	debug.SetMemoryLimit(128 * 1024 * 1024) // 128MB soft limit

	if len(os.Args) < 2 {
		fmt.Println("Usage: SAM -list [flags] | -run [flags] | -attract [flags] | -mouse | -joystick | -keyboard | -static")
		os.Exit(1)
	}

	if sendToRunningInstance(os.Args[1:]) {
		return
	}

	cfg, err := config.LoadUserConfig("SAM", &config.UserConfig{})
	if err != nil {
		fmt.Println("Config load error:", err)
	} else {
		fmt.Println("Loaded config from:", cfg.IniPath)
		dumpConfig(cfg)
	}

	commandChan := make(chan []string)
	startServer(commandChan)
	defer os.Remove(socketPath)

	go commandProcessor(commandChan)
	commandChan <- os.Args[1:]

	select {}
}
