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
)

// Global timer reference for attract mode
var AttractTimer *time.Timer

// Global BGM player
var bgmPlayer *Player

//
// -----------------------------
// Prepare + Init
// -----------------------------

// PrepareAttractLists builds gamelists in RAM, applies filters, then starts InitAttract.
func PrepareAttractLists(cfg *config.UserConfig, showStream bool) {
	// 🎵 Start background music during list building
	cfgBgm := GetConfig()
	if cfgBgm.Startup {
		bgmPlayer = &Player{
			Playlist: cfgBgm.Playlist,
			Playback: cfgBgm.Playback,
		}
		go bgmPlayer.StartLoop()
	}

	systemPaths := games.GetSystemPaths(cfg, games.AllSystems())
	if CreateGamelists(cfg, config.GamelistDir(), systemPaths, false) == 0 {
		fmt.Fprintln(os.Stderr, "[Attract] List build failed: no games indexed")
		os.Exit(1)
	}

	// 🔥 RAM-only gamelist keys
	files := ListKeys()
	if len(files) == 0 {
		fmt.Println("[Attract] No gamelists found in memory.")
		os.Exit(1)
	}

	fmt.Printf("[DEBUG] Found %d gamelists in memory: %v\n", len(files), files)

	// Filtering
	files = FilterAllowed(files, cfg.Attract.Include, cfg.Attract.Exclude)
	if len(files) == 0 {
		fmt.Println("[Attract] No allowed gamelists after filtering")
		os.Exit(1)
	}

	fmt.Printf("[DEBUG] Allowed gamelists after filtering: %v\n", files)

	InitAttract(cfg, files, showStream)
}

// InitAttract sets up input relay and runs the main loop.
func InitAttract(cfg *config.UserConfig, files []string, showStream bool) {
	inputCh := make(chan string, 32) // shared channel for all input events
	go input.RelayInputs(inputCh)

	RunAttractLoop(cfg, files, inputCh, showStream)
}

//
// -----------------------------
// Main Attract Loop
// -----------------------------

// RunAttractLoop runs the attract mode loop until interrupted.
func RunAttractLoop(cfg *config.UserConfig, files []string, inputCh <-chan string, showStream bool) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	fmt.Println("[Attract] Running. Press ESC to exit.")

	// Pick first game directly with PickRandomGame (no history write)
	first := PickRandomGame(cfg, r)
	if first == "" {
		fmt.Println("[Attract] No game available to start attract mode.")
		return
	}
		fmt.Printf("[Attract] First pick -> %s\n", filepath.Base(first))
	} else {
		fmt.Printf("[Attract] Failed to run first pick: %v\n", err)
		return
	}

	// 🎵 Stop background music before starting first game
	if bgmPlayer != nil {
		bgmPlayer.StopLoop()
		bgmPlayer = nil
	}

	// 🔥 Start static detector
	go func() {
		for ev := range Stream(cfg) {
			if showStream {
				fmt.Println(ev.String())
			}
			// else: silently drain
		}
	}()

	// Kick off ticker for first interval
	wait := ParsePlayTime(cfg.Attract.PlayTime, r)
	ResetAttractTicker(wait)

	// Input map
	inputMap := AttractInputMap(cfg, r, inputCh)

	// Event loop
	for {
		select {
		case <-AttractTickerChan():
			// Auto advance (this time recorded into history)
			if _, ok := Next(cfg, r); !ok {
				fmt.Println("[Attract] Failed to pick next game.")
			}

		case ev := <-inputCh:
			evLower := strings.ToLower(ev)
			fmt.Printf("[DEBUG] Event received: %q\n", evLower)

			if action, ok := inputMap[evLower]; ok {
				action()
			}
		}
	}
}
