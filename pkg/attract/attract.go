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
var bgmStop chan struct{}

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
		bgmStop = make(chan struct{})
		go func() {
			for {
				select {
				case <-bgmStop:
					return
				default:
					track := bgmPlayer.GetRandomTrack()
					if track == "" {
						time.Sleep(1 * time.Second)
						continue
					}
					bgmPlayer.Play(track)
				}
			}
		}()
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

	fmt.Printf("[DEBUG] Include groups expanded to: %v\n", include)
	fmt.Printf("[DEBUG] Exclude groups expanded to: %v\n", exclude)

	files = FilterAllowed(files, include, exclude)
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

	// Pick first game using Next()
	if first, ok := Next(cfg, r); ok {
		fmt.Printf("[Attract] First pick -> %s\n", filepath.Base(first))
	} else {
		fmt.Println("[Attract] No game available to start attract mode.")
		return
	}

	// 🎵 Stop background music before starting first game
	if bgmStop != nil {
		close(bgmStop)
		bgmStop = nil
		bgmPlayer = nil
	}

	// 🔥 Start static detector with optional draining/printing
	go func() {
		for ev := range Stream(cfg) {
			if showStream {
				fmt.Println(ev.String()) // full diagnostic string
			}
			// else: silently drain
		}
	}()

	// Kick off the ticker for the first interval
	wait := ParsePlayTime(cfg.Attract.PlayTime, r)
	ResetAttractTicker(wait)

	// Input map
	inputMap := AttractInputMap(cfg, r, inputCh)

	// Event loop
	for {
		select {
		case <-AttractTickerChan():
			// Advance automatically
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
