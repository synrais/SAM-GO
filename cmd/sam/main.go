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
	"github.com/synrais/SAM-GO/pkg/history"
	"github.com/synrais/SAM-GO/pkg/input"
	"github.com/synrais/SAM-GO/pkg/run"
	"github.com/synrais/SAM-GO/pkg/staticdetector"
)

const socketPath = "/tmp/sam.sock"

func dumpConfig(cfg *config.UserConfig) {
	fmt.Printf("INI Debug ->\n")
	fmt.Printf("  Attract: Include=%v | Exclude=%v | PlayTime=%s | Random=%v\n",
		cfg.Attract.Include, cfg.Attract.Exclude, cfg.Attract.PlayTime, cfg.Attract.Random)
	fmt.Printf("  List: Exclude=%v\n", cfg.List.Exclude)
	fmt.Printf("  Search: Filter=%v | Sort=%s\n", cfg.Search.Filter, cfg.Search.Sort)
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
		if err := attract.RunList(args); err != nil {
			fmt.Fprintln(os.Stderr, "List failed:", err)
		}
	case "-run":
		if err := run.Run(args); err != nil {
			fmt.Fprintln(os.Stderr, "Run failed:", err)
		}
	case "-attract":
		attract.Run(args)
	case "-back":
		if _, err := history.PlayBack(); err != nil {
			fmt.Fprintln(os.Stderr, "Back failed:", err)
		}
	case "-next":
		if _, err := history.PlayNext(); err != nil {
			fmt.Fprintln(os.Stderr, "Next failed:", err)
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

	cfg, err := config.LoadUserConfig("SAM", &config.UserConfig{})
	if err != nil {
		fmt.Println("Config load error:", err)
	} else {
		fmt.Println("Loaded config from:", cfg.IniPath)
		dumpConfig(cfg)
	}

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
