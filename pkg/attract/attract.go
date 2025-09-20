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

// PrepareAttractLists builds gamelists, applies filters, then starts InitAttract.
func PrepareAttractLists(cfg *config.UserConfig) {
	systemPaths := games.GetSystemPaths(cfg, games.AllSystems())
	if CreateGamelists(cfg, config.GamelistDir(), systemPaths, false) == 0 {
		fmt.Fprintln(os.Stderr, "[Attract] List build failed: no games indexed")
		os.Exit(1)
	}

	files, _ := filepath.Glob(filepath.Join(config.GamelistDir(), "*_gamelist.txt"))
	if len(files) == 0 {
		fmt.Println("[Attract] No gamelists found.")
		os.Exit(1)
	}

	include, err := ExpandGroups(cfg.Attract.Include)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[Attract] Error expanding include groups: %v\n", err)
		os.Exit(1)
	}
	exclude, err := ExpandGroups(cfg.Attract.Exclude)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[Attract] Error expanding exclude groups: %v\n", err)
		os.Exit(1)
	}

	files = FilterAllowed(files, include, exclude)
	if len(files) == 0 {
		fmt.Println("[Attract] No allowed gamelists after filtering")
		os.Exit(1)
	}

	InitAttract(cfg, files)
}

// InitAttract sets up input relay and runs the main loop.
func InitAttract(cfg *config.UserConfig, files []string) {
	inputCh := make(chan string, 32) // shared channel for all input events
	go input.RelayInputs(inputCh)

	RunAttractLoop(cfg, files, inputCh)
}

// RunAttractLoop runs the attract mode loop until interrupted.
func RunAttractLoop(cfg *config.UserConfig, files []string, inputCh <-chan string) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	fmt.Println("[Attract] Running. Press ESC to exit.")

	for {
		// First pick / next pick handled by Next()
		path, ok := Next(nil, cfg, r)
		if !ok {
			continue
		}

		// Start timer for this game
		wait := ParsePlayTime(cfg.Attract.PlayTime, r)
		timer := time.NewTimer(wait)

		// Load input map
		inputMap := AttractInputMap(cfg, r, timer, inputCh)

	loop:
		for {
			select {
			case <-timer.C:
				break loop // auto-advance

			case ev := <-inputCh:
				evLower := strings.ToLower(ev)
				fmt.Printf("[DEBUG] Event received: %q\n", evLower)

				if action, ok := inputMap[evLower]; ok {
					action()
				}
			}
		}
	}
}
