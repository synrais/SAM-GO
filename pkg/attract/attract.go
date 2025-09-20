package attract

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/games"
	"github.com/synrais/SAM-GO/pkg/input"
	"github.com/synrais/SAM-GO/pkg/utils"
)

// PrepareAttractLists builds gamelists, applies filters,
// then starts InitAttract to handle input and launch the main loop.
func PrepareAttractLists(cfg *config.UserConfig) {
	// 1. Ensure gamelists are built.
	systemPaths := games.GetSystemPaths(cfg, games.AllSystems())
	if CreateGamelists(cfg, config.GamelistDir(), systemPaths, false) == 0 {
		fmt.Fprintln(os.Stderr, "[Attract] List build failed: no games indexed")
		os.Exit(1)
	}

	// 2. Collect gamelist files.
	files, _ := filepath.Glob(filepath.Join(config.GamelistDir(), "*_gamelist.txt"))
	if len(files) == 0 {
		fmt.Println("[Attract] No gamelists found.")
		os.Exit(1)
	}

	// 3. Apply include/exclude filters.
	attractCfg := cfg.Attract
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

	files = FilterAllowed(files, include, exclude)
	if len(files) == 0 {
		fmt.Println("[Attract] No allowed gamelists after filtering")
		os.Exit(1)
	}

	// 4. Hand over to InitAttract.
	InitAttract(cfg, files)
}

// InitAttract sets up input relay and hands off to RunAttractLoop.
func InitAttract(cfg *config.UserConfig, files []string) {
	// Skip/back channels
	skipCh := make(chan struct{}, 1)
	backCh := make(chan struct{}, 1)

	// Relay keyboard/mouse/joystick input
	if cfg.InputDetector.Mouse || cfg.InputDetector.Keyboard || cfg.InputDetector.Joystick {
		input.RelayInputs(cfg,
			func() { select { case backCh <- struct{}{}: default: } },
			func() { select { case skipCh <- struct{}{}: default: } },
		)
	}

	// Now start the attract loop
	RunAttractLoop(cfg, files, skipCh, backCh)
}

// RunAttractLoop runs the attract mode loop until interrupted.
func RunAttractLoop(cfg *config.UserConfig, files []string, skipCh, backCh chan struct{}) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	fmt.Println("[Attract] Running. Ctrl-C to exit.")

	// Start static detector here, so it’s tied to the attract loop
	if cfg.Attract.UseStaticDetector {
		go func() {
			for ev := range Stream(cfg, skipCh) {
				fmt.Printf("[Attract] %s\n", ev)
			}
		}()
	}

	for {
		// Pick random gamelist
		listKey := files[r.Intn(len(files))]
		lines, err := utils.ReadLines(listKey)
		if err != nil || len(lines) == 0 {
			continue
		}

		// Pick random entry
		index := 0
		if cfg.Attract.Random {
			index = r.Intn(len(lines))
		}
		ts, gamePath := utils.ParseLine(lines[index])

		// Print + run game
		name := filepath.Base(gamePath)
		name = strings.TrimSuffix(name, filepath.Ext(name))
		fmt.Printf("[Attract] %s - %s <%s>\n",
			time.Now().Format("15:04:05"), name, gamePath)
		Run([]string{gamePath})

		// Decide wait time
		wait := ParsePlayTime(cfg.Attract.PlayTime, r)
		if ts > 0 {
			skipDuration := time.Duration(ts*float64(time.Second)) +
				time.Duration(cfg.List.SkipafterStatic)*time.Second
			if skipDuration < wait {
				wait = skipDuration
			}
		}

		// Wait or respond to skip/back
		timer := time.NewTimer(wait)
		select {
		case <-timer.C:
			// natural advance
		case <-skipCh:
			fmt.Println("[Attract] Skipped")
		case <-backCh:
			if prev, ok := PlayBack(); ok {
				gamePath = prev
				ts = 0
				timer.Stop()
				// jump straight to next iteration
				continue
			}
		}
	}
}
