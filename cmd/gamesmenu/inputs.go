package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/input"
)

// -------------------------
// Input detector test (gamesmenu.sh -inputs)
// -------------------------

// testInputs runs all three input detectors and prints every press
// until Ctrl+C. It runs them all whatever SAM.ini says, so each can be
// tested, and notes any that SAM.ini has turned off for attract mode.
func testInputs() {
	fmt.Println("=== SAM input detector test (Ctrl+C to stop) ===")
	fmt.Println("Press keys, click or move a mouse, or use a controller.")
	fmt.Println("Launch a game first to test while a core is running.")
	if cfg, err := config.Load(); err == nil {
		off := ""
		for _, d := range []struct {
			name string
			on   bool
		}{
			{"Keyboard", cfg.InputDetector.Keyboard},
			{"Mouse", cfg.InputDetector.Mouse},
			{"Joystick", cfg.InputDetector.Joystick},
		} {
			if !d.on {
				off += " " + d.name
			}
		}
		if off != "" {
			fmt.Printf("Note: turned off in SAM.ini [InputDetector], so attract mode won't use:%s\n", off)
		}
	}
	fmt.Println()

	events := input.Start(input.Options{Keyboard: true, Mouse: true, Joystick: true})
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	for {
		select {
		case ev := <-events:
			fmt.Printf("[Input] %s\n", ev)
		case <-stop:
			fmt.Println("\n=== Stopped ===")
			return
		}
	}
}
