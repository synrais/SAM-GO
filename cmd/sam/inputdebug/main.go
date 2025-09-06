package main

/*
#include <linux/input.h>
*/
import "C"

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Matches C struct input_event from <linux/input.h>
type inputEvent struct {
	Sec   int64
	Usec  int64
	Type  uint16
	Code  uint16
	Value int32
}

// Represents one device we’re monitoring
type InputDevice struct {
	Name   string
	Dev    string
	SysFs  string
	Vendor string
	Product string
}

// Enumerate all input devices by parsing /proc/bus/input/devices
func getInputDevices() ([]InputDevice, error) {
	file, err := os.Open("/proc/bus/input/devices")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var devices []InputDevice
	var dev InputDevice

	sc := bufio.NewScanner(file)
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			if dev.Dev != "" {
				devices = append(devices, dev)
			}
			dev = InputDevice{}
			continue
		}

		switch line[0] {
		case 'I':
			parts := strings.Split(line, " ")
			for _, p := range parts {
				if strings.HasPrefix(p, "Vendor=") {
					dev.Vendor = strings.TrimPrefix(p, "Vendor=")
				}
				if strings.HasPrefix(p, "Product=") {
					dev.Product = strings.TrimPrefix(p, "Product=")
				}
			}
		case 'N':
			// Name is quoted
			if idx := strings.Index(line, "\""); idx >= 0 {
				dev.Name = line[idx+1 : len(line)-1]
			}
		case 'H':
			// Handlers list (includes eventX, jsX, etc)
			handlers := strings.Split(line, " ")
			for _, h := range handlers {
				if strings.HasPrefix(h, "event") {
					dev.Dev = "/dev/input/" + h
				}
			}
		case 'S':
			dev.SysFs = "/sys" + line[9:]
		}
	}

	if dev.Dev != "" {
		devices = append(devices, dev)
	}

	return devices, nil
}

// Read loop for a single device
func readDevice(dev InputDevice, wg *sync.WaitGroup) {
	defer wg.Done()

	fmt.Printf("[DEBUG] Opening device: %s (%s)\n", dev.Dev, dev.Name)

	f, err := os.Open(dev.Dev)
	if err != nil {
		fmt.Printf("[ERROR] Could not open %s: %v\n", dev.Dev, err)
		return
	}
	defer f.Close()

	for {
		var ev inputEvent
		err := binary.Read(f, binary.LittleEndian, &ev)
		if err != nil {
			fmt.Printf("[ERROR] Reading from %s: %v\n", dev.Dev, err)
			return
		}

		// Debug: print raw event
		fmt.Printf("[EVENT] Dev=%s Name=%s Type=%d Code=%d Value=%d Time=%d.%06d\n",
			dev.Dev, dev.Name, ev.Type, ev.Code, ev.Value, ev.Sec, ev.Usec)

		// Hook: put your logic here
		// Example: detect key press
		if ev.Type == C.EV_KEY {
			if ev.Value == 1 {
				fmt.Printf("[DEBUG] Key DOWN code=%d from %s\n", ev.Code, dev.Name)
			} else if ev.Value == 0 {
				fmt.Printf("[DEBUG] Key UP code=%d from %s\n", ev.Code, dev.Name)
			}
		}

		if ev.Type == C.EV_ABS {
			fmt.Printf("[DEBUG] Absolute axis code=%d value=%d from %s\n", ev.Code, ev.Value, dev.Name)
		}

		if ev.Type == C.EV_REL {
			fmt.Printf("[DEBUG] Relative movement code=%d value=%d from %s\n", ev.Code, ev.Value, dev.Name)
		}
	}
}

func main() {
	devices, err := getInputDevices()
	if err != nil {
		fmt.Println("Error reading devices:", err)
		return
	}

	if len(devices) == 0 {
		fmt.Println("No input devices found.")
		return
	}

	fmt.Println("=== Input Devices Found ===")
	for _, d := range devices {
		fmt.Printf("- %s (%s) [%s:%s]\n", d.Dev, d.Name, d.Vendor, d.Product)
	}
	fmt.Println("===========================")

	var wg sync.WaitGroup
	for _, dev := range devices {
		wg.Add(1)
		go readDevice(dev, &wg)
	}

	wg.Wait()
}
