package attract

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/synrais/SAM-GO/pkg/cache"
	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/history"
	"github.com/synrais/SAM-GO/pkg/input"
	"github.com/synrais/SAM-GO/pkg/run"
	"github.com/synrais/SAM-GO/pkg/staticdetector"
	"github.com/synrais/SAM-GO/pkg/utils"
)

// parsePlayTime handles "40" or "40-130"
func parsePlayTime(value string, r *rand.Rand) time.Duration {
	if strings.Contains(value, "-") {
		parts := strings.SplitN(value, "-", 2)
		min, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
		max, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
		if max > min {
			return time.Duration(r.Intn(max-min+1)+min) * time.Second
		}
		return time.Duration(min) * time.Second
	}
	secs, _ := strconv.Atoi(value)
	return time.Duration(secs) * time.Second
}

// matchesPattern checks if string matches a wildcard (*foo*, bar*, *baz)
func matchesPattern(s, pattern string) bool {
	p := strings.ToLower(pattern)
	s = strings.ToLower(s)

	if strings.HasPrefix(p, "*") && strings.HasSuffix(p, "*") {
		return strings.Contains(s, strings.Trim(p, "*"))
	}
	if strings.HasPrefix(p, "*") {
		return strings.HasSuffix(s, strings.TrimPrefix(p, "*"))
	}
	if strings.HasSuffix(p, "*") {
		return strings.HasPrefix(s, strings.TrimSuffix(p, "*"))
	}
	return s == p
}

// disabled checks if a game should be blocked by rules
func disabled(system string, gamePath string, cfg *config.UserConfig) bool {
	rules, ok := cfg.Disable[system]
	if !ok {
		return false
	}

	base := filepath.Base(gamePath)
	ext := filepath.Ext(gamePath)
	dir := filepath.Base(filepath.Dir(gamePath))

	for _, f := range rules.Folders {
		if matchesPattern(dir, f) {
			return true
		}
	}
	for _, f := range rules.Files {
		if matchesPattern(base, f) {
			return true
		}
	}
	for _, e := range rules.Extensions {
		if strings.EqualFold(ext, e) {
			return true
		}
	}
	return false
}

// rebuildLists calls SAM -list to regenerate gamelists.
func rebuildLists() {
	fmt.Println("All gamelists empty. Rebuilding with SAM -list...")

	exe, _ := os.Executable()
	cmd := exec.Command(exe, "-list", "-overwrite")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()

	fmt.Println("Rebuilt gamelists, cache refreshed.")
}

// filterAllowed applies include/exclude restrictions case-insensitively.
func filterAllowed(all []string, include, exclude []string) []string {
	var filtered []string
	for _, sys := range all {
		base := strings.TrimSuffix(filepath.Base(sys), "_gamelist.txt")
		if len(include) > 0 {
			match := false
			for _, s := range include {
				if strings.EqualFold(strings.TrimSpace(s), base) {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}
		skip := false
		for _, s := range exclude {
			if strings.EqualFold(strings.TrimSpace(s), base) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		filtered = append(filtered, sys)
	}
	return filtered
}

// Run is the entry point for the attract tool.
func Run(args []string) {
	cfg, _ := config.LoadUserConfig("SAM", &config.UserConfig{})
	attractCfg := cfg.Attract

	// Build gamelists before processing
	listArgs := []string{}
	if attractCfg.FreshListsEachLoad {
		listArgs = append(listArgs, "-overwrite")
	}
	if err := RunList(listArgs); err != nil {
		fmt.Fprintln(os.Stderr, "List build failed:", err)
	}
	ProcessLists("/media/fat/Scripts/.MiSTer_SAM/SAM_Gamelists", cfg)

	// control channels
	skipCh := make(chan struct{}, 1)
	backCh := make(chan struct{}, 1)

	// parse extra flags
	silent := false
	for _, a := range args {
		if a == "-s" || a == "--silent" {
			silent = true
		}
	}

	// Start static detector
	if attractCfg.UseStaticDetector {
		go func() {
			for ev := range staticdetector.Stream(cfg, skipCh) {
				if !silent {
					fmt.Println(ev)
				}
			}
		}()
	}

	// Hook inputs → back = backCh, next = skipCh
	if cfg.InputDetector.Mouse || cfg.InputDetector.Keyboard || cfg.InputDetector.Joystick {
		input.RelayInputs(cfg,
			func() { // Back
				select {
				case backCh <- struct{}{}:
				default:
				}
			},
			func() { // Next
				select {
				case skipCh <- struct{}{}:
				default:
				}
			},
		)
	}

	// Collect gamelists from cache
	allKeys := cache.ListKeys()
	var allFiles []string
	for _, k := range allKeys {
		if strings.HasSuffix(k, "_gamelist.txt") {
			allFiles = append(allFiles, k)
		}
	}
	files := filterAllowed(allFiles, attractCfg.Include, attractCfg.Exclude)
	if len(files) == 0 {
		fmt.Println("No gamelists found in cache")
		os.Exit(1)
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	fmt.Println("Attract mode running. Ctrl-C to exit.")

	playGame := func(gamePath, systemID string, ts float64) {
	Launch:
		for {
			name := filepath.Base(gamePath)
			name = strings.TrimSuffix(name, filepath.Ext(name))
			fmt.Printf("%s - %s <%s>\n", time.Now().Format("15:04:05"), name, gamePath)
			run.Run([]string{gamePath})

			// base playtime
			wait := parsePlayTime(attractCfg.PlayTime, r)

			// if a static timestamp is set, cut playtime earlier
			if ts > 0 {
				skipDuration := time.Duration(ts*float64(time.Second)) +
					time.Duration(attractCfg.SkipafterStatic)*time.Second
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
					if next, err := history.PlayNext(); err == nil && next != "" {
						_ = history.SetNowPlaying(next) // browsing only
						gamePath = next
						systemID = ""
						ts = 0
						continue Launch
					}
					return
				case <-skipCh:
					if next, err := history.PlayNext(); err == nil && next != "" {
						_ = history.SetNowPlaying(next) // browsing only
						gamePath = next
						systemID = ""
						ts = 0
						continue Launch
					}
					return
				case <-backCh:
					if prev, err := history.PlayBack(); err == nil && prev != "" {
						_ = history.SetNowPlaying(prev) // browsing only
						gamePath = prev
						systemID = ""
						ts = 0
						continue Launch
					}
				}
			}
			if next, err := history.PlayNext(); err == nil && next != "" {
				_ = history.SetNowPlaying(next) // browsing only
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
			if prev, err := history.PlayBack(); err == nil && prev != "" {
				_ = history.SetNowPlaying(prev) // browsing only
				playGame(prev, "", 0)
				continue
			}
		case <-skipCh:
			if next, err := history.PlayNext(); err == nil && next != "" {
				_ = history.SetNowPlaying(next) // browsing only
				playGame(next, "", 0)
				continue
			}
		default:
		}

		if len(files) == 0 {
			// Instead of exiting, reset from shadow
			fmt.Println("All systems exhausted — refreshing from cache masters")
			cache.ResetAll()

			allKeys = cache.ListKeys()
			allFiles = nil
			for _, k := range allKeys {
				if strings.HasSuffix(k, "_gamelist.txt") {
					allFiles = append(allFiles, k)
				}
			}
			files = filterAllowed(allFiles, attractCfg.Include, attractCfg.Exclude)

			if len(files) == 0 {
				fmt.Println("No gamelists even after reset, exiting.")
				return
			}
		}

		listKey := files[r.Intn(len(files))]
		lines := cache.GetList(listKey)
		if len(lines) == 0 {
			// drop empty system from rotation
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

		if disabled(systemID, gamePath, cfg) {
			lines = append(lines[:index], lines[index+1:]...)
			cache.SetList(listKey, lines)
			continue
		}

		// random picks → logged into history
		_ = history.Play(gamePath)
		playGame(gamePath, systemID, ts)

		// remove from RAM copy
		lines = append(lines[:index], lines[index+1:]...)
		cache.SetList(listKey, lines)
	}
}
