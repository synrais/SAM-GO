package attract

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/synrais/SAM-GO/pkg/cache"
	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/games"
	"github.com/synrais/SAM-GO/pkg/input"
	"github.com/synrais/SAM-GO/pkg/utils"
)

// RunAttract is the main entrypoint for Attract Mode.
// Timeline overview:
//
// 1. Build gamelists (CreateGamelists)
// 2. Load gamelists into cache
// 3. Setup channels for Skip / Back actions
// 4. Start static detector + input listeners
// 5. Gather allowed system gamelists (respect include/exclude groups)
// 6. Enter the main Attract loop:
//    - Pick a system list and game
//    - Play game for configured time
//    - React to Skip / Back / Timer events
//    - Remove the game from the active list
//    - If all games exhausted, refresh from cache
//
// The loop continues indefinitely until killed (Ctrl+C).
func RunAttract(cfg *config.UserConfig, args []string) {
	attractCfg := cfg.Attract

	// 1. Ensure gamelists are built (or refreshed).
	systemPaths := games.GetSystemPaths(cfg, games.AllSystems())
	if CreateGamelists(cfg, config.GamelistDir(), systemPaths, false) == 0 {
		fmt.Fprintln(os.Stderr, "[Attract] List build failed: no games indexed")
	}

	// 2. Load lists into cache memory for quick access.
	for _, system := range games.AllSystems() {
		files, _ := filepath.Glob(filepath.Join(config.GamelistDir(), "*_"+system.Id+"_gamelist.txt"))
		for _, f := range files {
			lines, err := utils.ReadLines(f)
			if err != nil {
				continue
			}
			cache.SetList(filepath.Base(f), lines)
		}
	}

	// 3. Channels for navigation (skip/next/back).
	skipCh := make(chan struct{}, 1)
	backCh := make(chan struct{}, 1)

	// Optional silent flag for less logging.
	silent := false
	for _, a := range args {
		if a == "-s" || a == "--silent" {
			silent = true
		}
	}

	// 4a. Static detector watches for inactivity and pushes Skip events.
	if attractCfg.UseStaticDetector {
		go func() {
			for ev := range Stream(cfg, skipCh) {
				if !silent {
					fmt.Printf("[Attract] %s\n", ev)
				}
			}
		}()
	}

	// 4b. Input listeners (keyboard/mouse/joystick) for Skip/Back actions.
	if cfg.InputDetector.Mouse || cfg.InputDetector.Keyboard || cfg.InputDetector.Joystick {
		input.RelayInputs(cfg,
			func() { select { case backCh <- struct{}{}: default: } },
			func() { select { case skipCh <- struct{}{}: default: } },
		)
	}

	// 5. Collect gamelists in cache, apply include/exclude filters.
	allKeys := cache.ListKeys()
	var allFiles []string
	for _, k := range allKeys {
		if strings.HasSuffix(k, "_gamelist.txt") {
			allFiles = append(allFiles, k)
		}
	}

	include, err := ExpandGroups(attractCfg.Include)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[Attract] Error expanding include groups: %v\n", err)
		os.Exit(1)
	}
	exclude, err := ExpandGroups(attractCfg.Exclude)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[Attract] Error expanding exclude groups: %v\n", err)
		os.Exit(1)
	}

	files := FilterAllowed(allFiles, include, exclude)
	if len(files) == 0 {
		fmt.Println("[Attract] No gamelists found in cache")
		os.Exit(1)
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	if !silent {
		fmt.Println("[Attract] Running. Ctrl-C to exit.")
	}

	// Helper: play one game and handle skip/back/timer.
	playGame := func(gamePath string, ts float64) {
	Launch:
		for {
			name := filepath.Base(gamePath)
			name = strings.TrimSuffix(name, filepath.Ext(name))
			if !silent {
				fmt.Printf("[Attract] %s - %s <%s>\n",
					time.Now().Format("15:04:05"), name, gamePath)
			}
			Run([]string{gamePath}) // now local Run from utils.go

			// Decide how long to keep game running.
			wait := ParsePlayTime(attractCfg.PlayTime, r)
			if ts > 0 {
				// Staticlist timestamp may shorten duration.
				skipDuration := time.Duration(ts*float64(time.Second)) +
					time.Duration(cfg.List.SkipafterStatic)*time.Second
				if skipDuration < wait {
					wait = skipDuration
				}
			}

			// Loop until timer expires, Skip pressed, or Back pressed.
			deadline := time.Now().Add(wait)
			for time.Now().Before(deadline) {
				if input.IsSearching() {
					// Pause timer if in search mode.
					time.Sleep(100 * time.Millisecond)
					deadline = deadline.Add(100 * time.Millisecond)
					continue
				}
				remaining := time.Until(deadline)
				select {
				case <-time.After(remaining):
					if next, ok := PlayNext(); ok {
						gamePath = next
						ts = 0
						continue Launch
					}
					return
				case <-skipCh:
					if !silent {
						fmt.Println("[Attract] Skipped")
					}
					if next, ok := PlayNext(); ok {
						gamePath = next
						ts = 0
						continue Launch
					}
					return
				case <-backCh:
					if prev, ok := PlayBack(); ok {
						gamePath = prev
						ts = 0
						continue Launch
					}
				}
			}
			// Timer ran out → auto advance.
			if next, ok := PlayNext(); ok {
				gamePath = next
				ts = 0
				continue Launch
			}
			return
		}
	}

	// 6. Main attract loop: forever cycle.
	for {
		// Handle instant Back/Skip requests outside main play loop.
		select {
		case <-backCh:
			if prev, ok := PlayBack(); ok {
				playGame(prev, 0)
				continue
			}
		case <-skipCh:
			if next, ok := PlayNext(); ok {
				playGame(next, 0)
				continue
			}
		default:
		}

		// If exhausted, reset cache and start fresh.
		if len(files) == 0 {
			if !silent {
				fmt.Println("[Attract] All systems exhausted — refreshing from cache")
			}
			cache.ResetAll()

			allKeys = cache.ListKeys()
			allFiles = nil
			for _, k := range allKeys {
				if strings.HasSuffix(k, "_gamelist.txt") {
					allFiles = append(allFiles, k)
				}
			}
			files = FilterAllowed(allFiles, include, exclude)

			if len(files) == 0 {
				fmt.Println("[Attract] No gamelists even after reset, exiting.")
				return
			}
		}

		// Pick random system and game from its list.
		listKey := files[r.Intn(len(files))]
		lines := cache.GetList(listKey)
		if len(lines) == 0 {
			// Drop empty list from rotation.
			var newFiles []string
			for _, f := range files {
				if f != listKey {
					newFiles = append(newFiles, f)
				}
			}
			files = newFiles
			continue
		}

		// Pick one entry (first or random).
		index := 0
		if attractCfg.Random {
			index = r.Intn(len(lines))
		}
		ts, gamePath := utils.ParseLine(lines[index])

		// Mark in history and run.
		Play(gamePath)
		playGame(gamePath, ts)

		// Remove game from list so it won’t repeat until refresh.
		lines = append(lines[:index], lines[index+1:]...)
		cache.SetList(listKey, lines)
	}
}
