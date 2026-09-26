package mister

import (
	"fmt"
	"os"
	"path/filepath"
	s "strings"

	"github.com/synrais/SAMenu/pkg/config"
	"github.com/synrais/SAMenu/pkg/games"
)

func GenerateMgl(cfg *config.Config, system *games.System, path string, override string) (string, error) {
	// override the system rbf with the user specified one
	for _, setCore := range cfg.Systems.SetCore {
		parts := s.SplitN(setCore, ":", 2)
		if len(parts) != 2 {
			continue
		}

		if s.EqualFold(parts[0], system.Id) {
			system.Rbf = parts[1]
			break
		}
	}

	mgl := fmt.Sprintf("<mistergamedescription>\n\t<rbf>%s</rbf>\n", system.Rbf)

	if system.SetName != "" {
		sameDir := ""
		if system.SetNameSameDir {
			sameDir = " same_dir=\"1\""
		}

		mgl += fmt.Sprintf("\t<setname%s>%s</setname>\n", sameDir, system.SetName)
	}

	if path == "" {
		mgl += "</mistergamedescription>"
		return mgl, nil
	} else if override != "" {
		mgl += override
		mgl += "</mistergamedescription>"
		return mgl, nil
	}

	mglDef, err := games.PathToMglDef(*system, path)
	if err != nil {
		return "", err
	}

	mgl += fmt.Sprintf("<file delay=\"%d\" type=\"%s\" index=\"%d\" path=\"../../../../..%s\"/>\n", mglDef.Delay, mglDef.Method, mglDef.Index, path)
	mgl += "</mistergamedescription>"
	return mgl, nil
}

func writeTempFile(content string) (string, error) {
	tmpFile, err := os.Create(config.LastLaunchFile)
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	_, err = tmpFile.WriteString(content)
	if err != nil {
		return "", err
	}
	return tmpFile.Name(), nil
}

func launchFile(path string) error {
	_, err := os.Stat(config.CmdInterface)
	if err != nil {
		return fmt.Errorf("command interface not accessible: %s", err)
	}

	if !(s.HasSuffix(s.ToLower(path), ".mgl") || s.HasSuffix(s.ToLower(path), ".mra") || s.HasSuffix(s.ToLower(path), ".rbf")) {
		return fmt.Errorf("not a valid launch file: %s", path)
	}

	cmd, err := os.OpenFile(config.CmdInterface, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer cmd.Close()

	cmd.WriteString(fmt.Sprintf("load_core %s\n", path))

	return nil
}

func launchTempMgl(cfg *config.Config, system *games.System, path string) error {
	override, err := games.RunSystemHook(cfg, *system, path)
	if err != nil {
		return err
	}

	mgl, err := GenerateMgl(cfg, system, path, override)
	if err != nil {
		return err
	}

	tmpFile, err := writeTempFile(mgl)
	if err != nil {
		return err
	}

	return launchFile(tmpFile)
}

func LaunchGame(cfg *config.Config, system games.System, path string) error {
	// [BiosSkip]: create the virtual pad before the game loads.
	startAutoInput(cfg, system)

	// check if sidelaunchers wants to handle this system specially
	if handled, err := SideLaunchers(cfg, system, path); handled {
		return err
	}

	switch s.ToLower(filepath.Ext(path)) {
	case ".mra":
		// Arcade launchers are already valid files
		err := launchFile(path)
		if err != nil {
			return err
		}
	case ".mgl":
		// Pre-made .mgl file → just launch
		err := launchFile(path)
		if err != nil {
			return err
		}
		if ActiveGameEnabled() {
			SetActiveGame(path)
		}
	default:
		// Generic game file → build temporary MGL and launch
		err := launchTempMgl(cfg, &system, path)
		if err != nil {
			return err
		}
		if ActiveGameEnabled() {
			SetActiveGame(path)
		}
	}

	return nil
}

func LaunchMenu() error {
	if _, err := os.Stat(config.CmdInterface); err != nil {
		return fmt.Errorf("command interface not accessible: %s", err)
	}

	cmd, err := os.OpenFile(config.CmdInterface, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer cmd.Close()

	// TODO: don't hardcode here
	cmd.WriteString(fmt.Sprintf("load_core %s\n", filepath.Join(config.SdFolder, "menu.rbf")))

	return nil
}

// LaunchGenericFile Given a generic file path, launch it using the correct method, if possible.
func LaunchGenericFile(cfg *config.Config, path string) error {

	system, _ := games.BestSystemMatch(cfg, path)

	// check if sidelaunchers wants to handle this system specially
	if system.Id != "" {
		if handled, err := SideLaunchers(cfg, system, path); handled {
			return err
		}
	}

	var err error
	isGame := false
	ext := s.ToLower(filepath.Ext(path))

	switch ext {
	case ".mra":
		err = launchFile(path)
	case ".mgl":
		err = launchFile(path)
		isGame = true
	case ".rbf":
		err = launchFile(path)
	default:
		if system.Id == "" {
			return fmt.Errorf("unknown file type: %s", ext)
		}
		err = launchTempMgl(cfg, &system, path)
		isGame = true
	}
	if err != nil {
		return err
	}

	// Track active game if applicable
	if ActiveGameEnabled() && isGame {
		if err := SetActiveGame(path); err != nil {
			return err
		}
	}

	return nil
}

// SetMute mutes or unmutes MiSTer's sound (its "volume mute" command).
// A mute from attract mode is noted in muteMarker, so it can be undone
// even if attract mode is stopped without cleaning up.
func SetMute(on bool) error {
	f, err := os.OpenFile(config.CmdInterface, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	cmd := "volume unmute\n"
	if on {
		cmd = "volume mute\n"
	}
	_, err = f.WriteString(cmd)
	if on {
		_ = os.WriteFile(muteMarker, nil, 0644)
	} else {
		_ = os.Remove(muteMarker)
	}
	return err
}

const muteMarker = "/tmp/SAMenu_attract_muted"

// UndoStaleMute unmutes if attract mode muted the sound and is no longer
// running to unmute it (e.g. it was killed).
func UndoStaleMute(attractRunning bool) {
	if _, err := os.Stat(muteMarker); err == nil && !attractRunning {
		_ = SetMute(false)
	}
}
