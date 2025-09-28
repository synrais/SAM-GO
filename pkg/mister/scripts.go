package mister

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/input/virtualinput"
)

type Script struct {
	Name     string `json:"name"`
	Filename string `json:"filename"`
	Path     string `json:"path"`
}

func IsMenuRunning() bool {
	activeCore, err := GetActiveCoreName()
	if err != nil {
		return false
	}
	return activeCore == config.MenuCore
}

func IsScriptRunning() bool {
	cmd := "ps ax | grep /tmp/script | grep -v grep"
	out, err := exec.Command("sh", "-c", cmd).Output()
	if err != nil {
		return false
	}
	return len(out) > 0
}

func KillActiveScript() error {
	if !IsScriptRunning() {
		return nil
	}
	cmd := "ps ax | grep /tmp/script | grep -v grep | awk '{print $1}' | xargs kill"
	return exec.Command("sh", "-c", cmd).Run()
}

func ScriptCanLaunch() bool {
	return IsMenuRunning() && !IsScriptRunning()
}

func openConsole(kbd *virtualinput.Keyboard) error {
	if !IsMenuRunning() {
		return fmt.Errorf("cannot open console, active core is not menu")
	}

	getTty := func() (string, error) {
		sys := "/sys/devices/virtual/tty/tty0/active"
		if _, err := os.Stat(sys); err != nil {
			return "", err
		}
		tty, err := os.ReadFile(sys)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(tty)), nil
	}

	// Switch to tty3, then try F9 until tty1 is active
	if err := exec.Command("chvt", "3").Run(); err != nil {
		return err
	}

	tries := 0
	for {
		if tries > 20 {
			return fmt.Errorf("could not switch to tty1")
		}

		if code, ok := virtualinput.ToKeyboardCode("f9"); ok {
			if err := kbd.Press(code); err != nil {
				return fmt.Errorf("failed to press F9: %w", err)
			}
		}

		time.Sleep(50 * time.Millisecond)

		tty, err := getTty()
		if err != nil {
			return err
		}
		if tty == "tty1" {
			break
		}
		tries++
	}
	return nil
}

func GetAllScripts() ([]Script, error) {
	scripts := make([]Script, 0)

	files, err := os.ReadDir(config.ScriptsFolder)
	if err != nil {
		return scripts, err
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		fn := file.Name()
		if strings.HasSuffix(strings.ToLower(fn), ".sh") {
			scripts = append(scripts, Script{
				Name:     strings.TrimSuffix(fn, filepath.Ext(fn)),
				Filename: fn,
				Path:     filepath.Join(config.ScriptsFolder, fn),
			})
		}
	}
	return scripts, nil
}

func RunScript(path string) error {
	if _, err := os.Stat(path); err != nil {
		return err
	}
	if !ScriptCanLaunch() {
		return fmt.Errorf("script cannot be launched, active core is not menu or script is already running")
	}

	// Create virtual keyboard
	kbd, err := virtualinput.NewKeyboard(40 * time.Millisecond)
	if err != nil {
		return fmt.Errorf("failed to create virtual keyboard: %w", err)
	}
	defer kbd.Close()

	// Open console
	if err := openConsole(kbd); err != nil {
		return err
	}

	// Reserve tty2 for scripts
	if err := exec.Command("chvt", "2").Run(); err != nil {
		return err
	}

	// Script launcher wrapper
	launcher := fmt.Sprintf(`#!/bin/bash
export LC_ALL=en_US.UTF-8
export HOME=/root
export LESSKEY=/media/fat/linux/lesskey
cd $(dirname "%s")
%s
`, path, path)

	if err := os.WriteFile("/tmp/script", []byte(launcher), 0755); err != nil {
		return err
	}

	if err := exec.Command(
		"/sbin/agetty",
		"-a", "root",
		"-l", "/tmp/script",
		"--nohostname",
		"-L", "tty2", "linux",
	).Run(); err != nil {
		return err
	}

	// Exit console with F12
	if code, ok := virtualinput.ToKeyboardCode("f12"); ok {
		if err := kbd.Press(code); err != nil {
			return fmt.Errorf("failed to press F12: %w", err)
		}
	}

	return nil
}
