package main

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/synrais/SAM-GO/pkg/attract"
	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/games"
	"github.com/synrais/SAM-GO/pkg/gamesdb"
	"github.com/synrais/SAM-GO/pkg/mister"
	"github.com/synrais/SAM-GO/pkg/music"
)

// -------------------------
// Command Line
// -------------------------
//
// Options for running the games menu from outside (SSH, scripts, Zaparoo,
// Home Assistant...). They're listed at the top of SAM.ini as a cheat
// sheet. None of them take the menu's lock, so they never stop a running
// menu or attract mode (except -launch and -random, which end attract
// mode on purpose to start their game).

// remoteCommand sends a command to a running attract mode.
func remoteCommand(cmd string) {
	if err := attract.SendCommand(cmd); err != nil {
		fmt.Println("Attract mode isn't running.")
		os.Exit(1)
	}
	fmt.Printf("Sent %q to attract mode.\n", cmd)
}

// buildDatabase indexes the games, printing progress.
func buildDatabase(cfg *config.Config) ([]MenuFile, error) {
	prevSystem, prevTotal, done := "", 0, 0
	if _, err := gamesdb.NewNamesIndex(cfg, games.AllSystems(), func(s gamesdb.IndexStatus) {
		if prevSystem != "" {
			done++
			fmt.Printf("[DB] %d/%d %s: %d games (total %d)\n", done, s.Total-1, prevSystem, s.Files-prevTotal, s.Files)
		}
		prevSystem, prevTotal = s.SystemId, s.Files
	}); err != nil {
		return nil, err
	}
	return loadMenuDb()
}

// loadOrBuild loads the games database, building it first if needed.
func loadOrBuild(cfg *config.Config) ([]MenuFile, error) {
	if files, err := loadMenuDb(); err == nil {
		return files, nil
	}
	fmt.Println("[Menu] No games database found, building...")
	return buildDatabase(cfg)
}

func mustConfig() *config.Config {
	cfg, err := config.Load()
	if err != nil {
		fmt.Println("Couldn't load SAM.ini:", err)
		os.Exit(1)
	}
	return cfg
}

// listGames prints every game in the database (the old -print).
func listGames() {
	files, err := loadOrBuild(mustConfig())
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	for _, f := range files {
		name := f.Name
		if f.Ext != "" {
			name += "." + f.Ext
		}
		fmt.Println(filepath.Join(f.MenuPath, name))
	}
}

// rebuildDatabase rebuilds the games database with no screens.
func rebuildDatabase() {
	files, err := buildDatabase(mustConfig())
	if err != nil {
		fmt.Println("Rebuild failed:", err)
		os.Exit(1)
	}
	fmt.Printf("Done: %d games.\n", len(files))
}

// endAttract stops a running attract mode, leaving its game running, so
// -launch and -random can start theirs.
func endAttract() {
	if attract.SendCommand("quit") == nil {
		fmt.Println("Stopping attract mode...")
		for i := 0; i < 20 && attract.SendCommand("") == nil; i++ {
			time.Sleep(250 * time.Millisecond)
		}
	}
}

// launchFile launches one game file (with BIOS skip, as a menu launch).
func launchFile(path string) {
	cfg := mustConfig()
	abs, err := filepath.Abs(path)
	if err == nil {
		path = abs
	}
	sys, err := games.BestSystemMatch(cfg, path)
	if err != nil {
		// Not a game file of a known system: cores (.rbf), MGL files and
		// arcade files (.mra) can still be launched from anywhere.
		endAttract()
		if err := mister.LaunchGenericFile(cfg, path); err != nil {
			fmt.Println("Couldn't launch it:", err)
			os.Exit(1)
		}
		fmt.Printf("Launched %s.\n", filepath.Base(path))
		return
	}
	endAttract()
	if err := mister.LaunchGame(cfg, sys, path); err != nil {
		fmt.Println("Launch failed:", err)
		os.Exit(1)
	}
	fmt.Printf("Launched %s (%s).\n", filepath.Base(path), sys.Name)
}

// launchRandom launches a random game, from the systems or groups given
// ("Console", "Nintendo", "SNES") or from everything.
func launchRandom(args []string) {
	cfg := mustConfig()
	files, err := loadOrBuild(cfg)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	var want map[string]bool
	if len(args) > 0 {
		var names []string
		for _, a := range args {
			names = append(names, strings.Split(a, ",")...)
		}
		want, _ = games.ResolveSystems(names)
	}
	var pool []MenuFile
	for _, f := range files {
		if want == nil || want[strings.ToLower(f.SystemId)] {
			pool = append(pool, f)
		}
	}
	if len(pool) == 0 {
		fmt.Println("No games found for:", strings.Join(args, " "))
		os.Exit(1)
	}
	f := pool[rand.Intn(len(pool))]
	sys, err := games.GetSystem(f.SystemId)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	endAttract()
	if err := mister.LaunchGame(cfg, *sys, f.Path); err != nil {
		fmt.Println("Launch failed:", err)
		os.Exit(1)
	}
	fmt.Printf("Launched %s (%s).\n", f.Name, sys.Name)
}

// startInSearch opens the games menu straight into its Search screen.
var startInSearch bool

// openOnTV opens the games menu (or its Search) on the TV from outside:
// through a running attract mode, or with a background helper.
func openOnTV(search bool) {
	mode := "menu"
	if search {
		mode = "search"
	}
	if attract.SendCommand(mode) == nil {
		fmt.Println("Asked attract mode to open the games menu on the TV.")
		return
	}
	exe, err := os.Executable()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	cmd := exec.Command(exe, "-openmenu", mode)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		fmt.Println("Couldn't open the games menu:", err)
		os.Exit(1)
	}
	_ = cmd.Process.Release()
	fmt.Println("Opening the games menu on the TV...")
}

// musicCommand controls the music player: start, stop, next or status.
func musicCommand(cmd string) {
	switch strings.ToLower(cmd) {
	case "start":
		exe, err := os.Executable()
		if err == nil {
			err = music.Start(exe)
		}
		if err != nil {
			fmt.Println("Couldn't start the music player:", err)
			os.Exit(1)
		}
		fmt.Println("Music player:", music.Status())
	case "stop":
		music.Stop()
		fmt.Println("Music player stopped.")
	case "next":
		if err := music.Send("next"); err != nil {
			fmt.Println("The music player isn't running.")
			os.Exit(1)
		}
		fmt.Println("Next track.")
	case "status":
		fmt.Println("Music player:", music.Status())
	default:
		fmt.Println("Usage: gamesmenu.sh -music start|stop|next|status")
		os.Exit(1)
	}
}

// bootStart runs from user-startup.sh at MiSTer startup: it waits for the
// MiSTer to finish starting, then opens the games menu or attract mode.
func bootStart(what string) {
	if err := mister.WaitForMiSTer(); err != nil {
		fmt.Println(err)
		return
	}
	switch what {
	case "attract":
		_ = startAttractInBackground()
	case "menu":
		_ = mister.OpenGamesMenu(false)
	}
}
