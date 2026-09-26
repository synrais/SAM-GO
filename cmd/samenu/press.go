package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/synrais/SAMenu/pkg/input/virtualinput"
	"github.com/synrais/SAMenu/pkg/mister"
)

// -------------------------
// Virtual Press Test (SAMenu.sh -press ...)
// -------------------------

// testPress presses buttons on SAMenu's virtual pad or keyboard, to check
// they reach a running core, e.g. "SAMenu.sh -press start" with a game
// on its title screen.
func testPress(args []string) {
	seq := strings.Join(args, " ")
	steps, err := mister.ParseSequence(seq)
	if err != nil || len(steps) == 0 {
		fmt.Println("Usage: SAMenu.sh -press <buttons...>")
		fmt.Println("  pad:      a b x y l r select start home up down left right")
		fmt.Println("  keyboard: key:enter key:space key:esc key:f1 key:a ...")
		fmt.Println("  pauses:   a number of seconds, e.g. -press start 1 a")
		if err != nil {
			fmt.Println("Error:", err)
		}
		return
	}
	dev, err := mister.OpenDevices(steps)
	if err != nil {
		fmt.Println("Couldn't create the virtual device:", err)
		return
	}
	defer dev.Close()

	// MiSTer needs a moment to find a newly plugged-in device. Check it
	// really did: its main program opens each input device it uses.
	fmt.Println("Created SAMenu's virtual device, waiting for MiSTer to pick it up...")
	nodes := samDeviceNodes()
	if len(nodes) == 0 {
		fmt.Println("  !! No input device called SAMenu found in /proc/bus/input/devices")
	}
	for _, n := range nodes {
		fmt.Printf("  device: %s (%s)\n", n.node, n.info)
	}
	opened := false
	for i := 0; i < 20 && !opened; i++ { // up to 5 seconds
		time.Sleep(250 * time.Millisecond)
		for _, n := range nodes {
			if mainHasOpen(n.node) {
				opened = true
			}
		}
	}
	if opened {
		fmt.Println("  MiSTer has opened SAMenu's device: yes")
	} else {
		fmt.Println("  !! MiSTer has NOT opened SAMenu's device, so it can't see the presses")
	}
	time.Sleep(500 * time.Millisecond)
	if err := dev.Run(steps, func(s string) { fmt.Println("  " + s) }); err != nil {
		fmt.Println("Error:", err)
		return
	}
	time.Sleep(300 * time.Millisecond)
	fmt.Println("Done. Did the core react?")
}

type devNode struct{ node, info string }

// samDeviceNodes finds the /dev/input/event* nodes of SAMenu's virtual
// devices (named virtualinput.DeviceName).
func samDeviceNodes() []devNode {
	b, err := os.ReadFile("/proc/bus/input/devices")
	if err != nil {
		return nil
	}
	var out []devNode
	for _, block := range strings.Split(string(b), "\n\n") {
		if !strings.Contains(block, `Name="`+virtualinput.DeviceName+`"`) {
			continue
		}
		info := ""
		for _, line := range strings.Split(block, "\n") {
			if strings.HasPrefix(line, "I:") {
				info = strings.TrimSpace(strings.TrimPrefix(line, "I:"))
			}
			if strings.HasPrefix(line, "H: Handlers=") {
				for _, h := range strings.Fields(strings.TrimPrefix(line, "H: Handlers=")) {
					if strings.HasPrefix(h, "event") {
						out = append(out, devNode{"/dev/input/" + h, info + ", handlers: " + strings.TrimPrefix(line, "H: Handlers=")})
					}
				}
			}
		}
	}
	return out
}

// mainHasOpen reports whether MiSTer's main program has a file open.
func mainHasOpen(path string) bool {
	procs, _ := filepath.Glob("/proc/[0-9]*/comm")
	for _, c := range procs {
		if b, err := os.ReadFile(c); err != nil || strings.TrimSpace(string(b)) != "MiSTer" {
			continue
		}
		fds, _ := filepath.Glob(filepath.Join(filepath.Dir(c), "fd", "*"))
		for _, fd := range fds {
			if target, err := os.Readlink(fd); err == nil && target == path {
				return true
			}
		}
	}
	return false
}
