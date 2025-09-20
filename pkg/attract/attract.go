package attract

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/synrais/SAM-GO/pkg/cache"
	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/games"
	"github.com/synrais/SAM-GO/pkg/input"
	"github.com/synrais/SAM-GO/pkg/utils"
)

// RunAttract is the main entrypoint for Attract Mode.
func RunAttract(cfg *config.UserConfig, args []string) {
	attractCfg := cfg.Attract

	// 1. Ensure gamelists are built.
	systemPaths := games.GetSystemPaths(cfg, games.AllSystems())
	if CreateGamelists(cfg, config.GamelistDir(), systemPaths, false) == 0 {
		fmt.Fprintln(os.Stderr, "[Attract] List build failed: no games indexed")
	}

	// 2. Load lists into cache.
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

	// Silent flag.
	silent := false
	for _, a := range args {
		if a == "-s" || a == "--silent" {
			silent = true
		}
	}

	// 4a. Static detector.
	if attractCfg.UseStaticDetector {
		go func() {
			for ev := range Stream(cfg, skipCh) {
				if !silent {
					fmt.Printf("[Attract] %s\n", ev)
				}
			}
		}()
	}

	// 4b. Input listeners.
	if cfg.InputDetector.Mouse || cfg.InputDetector.Keyboard || cfg.InputDetector.Joystick {
		input.RelayInputs(cfg,
			func() { select { case backCh <- struct{}{}: default: } },
			func() { select { case skipCh <- struct{}{}: default: } },
		)
	}

	// 5. Collect gamelists.
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
			Run([]string{gamePath}) // local Run from utils.go

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

			timer := time.NewTimer(wait)
			defer timer.Stop()

			for {
				if input.IsSearching() {
					timer.Stop()
					time.Sleep(100 * time.Millisecond)
					continue
				}

				select {
				case <-timer.C:
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
		}
	}

	// 6. Main loop.
	for {
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

		listKey := files[r.Intn(len(files))]
		lines := cache.GetList(listKey)
		if len(lines) == 0 {
			var newFiles []string
			for _, f := range files {
				if f != listKey {
					newFiles = append(newFiles, f)
				}
			}
			files = newFiles
			continue
		}

		index := 0
		if attractCfg.Random {
			index = r.Intn(len(lines))
		}
		ts, gamePath := utils.ParseLine(lines[index])

		Play(gamePath)
		playGame(gamePath, ts)

		lines = append(lines[:index], lines[index+1:]...)
		cache.SetList(listKey, lines)
	}
}

// ParsePlayTime parses playtime strings like "40" or "40-130" into a time.Duration
// and logs the exact interval chosen.
func ParsePlayTime(spec string, r *rand.Rand) time.Duration {
	parts := strings.Split(spec, "-")

	// Single value
	if len(parts) == 1 {
		val, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil || val <= 0 {
			val = 60 // fallback default
		}
		dur := time.Duration(val) * time.Second
		fmt.Printf("[Attract] Playtime set: %ds\n", val)
		return dur
	}

	// Range: min-max
	min, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	max, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil || min <= 0 || max <= 0 || min >= max {
		min, max = 30, 90 // fallback defaults
	}

	val := r.Intn(max-min+1) + min
	dur := time.Duration(val) * time.Second
	fmt.Printf("[Attract] Playtime set: random %ds (range %d-%ds)\n", val, min, max)
	return dur
}
