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
	"github.com/synrais/SAM-GO/pkg/run"
	"github.com/synrais/SAM-GO/pkg/staticdetector"
	"github.com/synrais/SAM-GO/pkg/utils"
)

// Run executes attract mode using the provided config and args.
func Run(cfg *config.UserConfig, args []string) {
	attractCfg := cfg.Attract

	// Ensure gamelists are built using CreateGamelists
	systemPaths := games.GetSystemPaths(cfg, games.AllSystems())
	if CreateGamelists(cfg, config.GamelistDir(), systemPaths, false) == 0 {
		fmt.Fprintln(os.Stderr, "[Attract] List build failed: no games indexed")
	}

	// Load lists into cache
	for _, system := range games.AllSystems() {
		files, _ := filepath.Glob(filepath.Join(config.GamelistDir(), "*_"+system.Id+"_gamelist.txt"))
		for _, f := range files {
			lines, err := utils.ReadLines(f)
			if err != nil {
				continue
			}
			lines, counts, _ := ApplyFilterlists(config.GamelistDir(), system.Id, lines, cfg)
			cache.SetList(filepath.Base(f), lines)
			if counts["White"] > 0 || counts["Black"] > 0 || counts["Static"] > 0 || counts["Folder"] > 0 || counts["File"] > 0 {
				fmt.Printf("[Attract] %s - White: %d, Black: %d, Static: %d, Folder: %d, File: %d\n",
					system.Id, counts["White"], counts["Black"], counts["Static"], counts["Folder"], counts["File"])
			}
		}
	}

	skipCh := make(chan struct{}, 1)
	backCh := make(chan struct{}, 1)

	silent := false
	for _, a := range args {
		if a == "-s" || a == "--silent" {
			silent = true
		}
	}

	if attractCfg.UseStaticDetector {
		go func() {
			for ev := range staticdetector.Stream(cfg, skipCh) {
				if !silent {
					fmt.Printf("[Attract] %s\n", ev)
				}
			}
		}()
	}

	if cfg.InputDetector.Mouse || cfg.InputDetector.Keyboard || cfg.InputDetector.Joystick {
		input.RelayInputs(cfg,
			func() { select { case backCh <- struct{}{}: default: } },
			func() { select { case skipCh <- struct{}{}: default: } },
		)
	}

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

	playGame := func(gamePath, systemID string, ts float64) {
	Launch:
		for {
			name := filepath.Base(gamePath)
			name = strings.TrimSuffix(name, filepath.Ext(name))
			if !silent {
				fmt.Printf("[Attract] %s - %s <%s>\n",
					time.Now().Format("15:04:05"), name, gamePath)
			}
			run.Run([]string{gamePath})

			wait := ParsePlayTime(attractCfg.PlayTime, r)
			if ts > 0 {
				skipDuration := time.Duration(ts*float64(time.Second)) +
					time.Duration(cfg.List.SkipafterStatic)*time.Second
				if skipDuration < wait {
					wait = skipDuration
				}
			}

			deadline := time.Now().Add(wait)
			for time.Now().Before(deadline) {
				if input.IsSearching() {
					time.Sleep(100 * time.Millisecond)
					deadline = deadline.Add(100 * time.Millisecond)
					continue
				}
				remaining := time.Until(deadline)
				select {
				case <-time.After(remaining):
					if next, ok := PlayNext(); ok {
						gamePath = next
						systemID = ""
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
						systemID = ""
						ts = 0
						continue Launch
					}
					return
				case <-backCh:
					if prev, ok := PlayBack(); ok {
						gamePath = prev
						systemID = ""
						ts = 0
						continue Launch
					}
				}
			}
			if next, ok := PlayNext(); ok {
				gamePath = next
				systemID = ""
				ts = 0
				continue Launch
			}
			return
		}
	}

	for {
		select {
		case <-backCh:
			if prev, ok := PlayBack(); ok {
				playGame(prev, "", 0)
				continue
			}
		case <-skipCh:
			if next, ok := PlayNext(); ok {
				playGame(next, "", 0)
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
		systemID := strings.TrimSuffix(filepath.Base(listKey), "_gamelist.txt")

		if Disabled(systemID, gamePath, cfg) {
			lines = append(lines[:index], lines[index+1:]...)
			cache.SetList(listKey, lines)
			continue
		}

		Play(gamePath)
		playGame(gamePath, systemID, ts)

		lines = append(lines[:index], lines[index+1:]...)
		cache.SetList(listKey, lines)
	}
}
