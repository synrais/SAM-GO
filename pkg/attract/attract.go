package attract

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/games"
	"github.com/synrais/SAM-GO/pkg/gamesdb"
	"github.com/synrais/SAM-GO/pkg/input"
	"github.com/synrais/SAM-GO/pkg/lists"
	"github.com/synrais/SAM-GO/pkg/mister"
)

// StartAttractMode picks and plays random games endlessly using the existing menu database.
func StartAttractMode(cfg *config.Config, files []gamesdb.FileInfo) error {
	fmt.Println("=== Starting Attract Mode ===")

	filtered := filterSystems(files, cfg)
	if len(filtered) == 0 {
		return fmt.Errorf("no games available after filtering")
	}

	// inline playtime parser
	minTime, maxTime := 40, 40
	raw := strings.TrimSpace(cfg.Attract.PlayTime)
	if raw != "" {
		if strings.Contains(raw, "-") {
			var a, b int
			fmt.Sscanf(raw, "%d-%d", &a, &b)
			if a > 0 {
				minTime = a
			}
			if b >= a {
				maxTime = b
			} else {
				maxTime = minTime
			}
		} else {
			var v int
			fmt.Sscanf(raw, "%d", &v)
			if v > 0 {
				minTime, maxTime = v, v
			}
		}
	}

	rand.Seed(time.Now().UnixNano())

	useBlack := listFilter(cfg.List.UseBlacklist, cfg.List.BlacklistInclude, cfg.List.BlacklistExclude)
	useStatic := listFilter(cfg.List.UseStaticlist, cfg.List.StaticlistInclude, cfg.List.StaticlistExclude)

	var det *Detector
	if cfg.Attract.UseStaticDetector {
		var err error
		if det, err = StartDetector(cfg); err != nil {
			fmt.Printf("[Attract] %v (continuing without it)\n", err)
		} else {
			fmt.Println("[Attract] Static detector running")
		}
	}

	mister.AutoInputFrom = "attract" // for the [BiosSkip] switches
	events := startInputDetectors(cfg)
	remote = startRemote()
	defer stopRemote()

	history := newTimeline()
	var next *gamesdb.FileInfo // from the history; nil = pick a random game

	// removeGame drops a game from the pool attract mode picks from.
	removeGame := func(path string) {
		for i := range filtered {
			if filtered[i].Path == path {
				filtered = append(filtered[:i], filtered[i+1:]...)
				return
			}
		}
	}
	// stepForward queues the next game in the history, if any.
	stepForward := func() {
		if g, ok := history.forward(); ok {
			next = &g
		}
	}

	for len(filtered) > 0 {
		var game gamesdb.FileInfo
		fromHistory := next != nil
		switch {
		case fromHistory:
			game, next = *next, nil
		case len(searchQueue) > 0: // search results play first
			game, searchQueue = searchQueue[0], searchQueue[1:]
			if len(searchQueue) == 0 {
				fmt.Println("[Search] Last search result, then back to the normal list")
			}
		default:
			game = filtered[rand.Intn(len(filtered))]
		}

		sys, err := games.GetSystem(game.SystemId)
		if err != nil || sys == nil {
			removeGame(game.Path)
			history.remove(game.Path)
			if fromHistory {
				stepForward()
			}
			continue
		}

		display := game.Name
		if game.Ext != "" {
			display += "." + game.Ext
		}

		// The blacklist can grow while we run (the detector adds to it).
		if useBlack(sys.Id) && lists.Names(lists.Blacklist, sys.Id)[lists.Key(game.Name)] {
			removeGame(game.Path)
			history.remove(game.Path)
			if fromHistory {
				stepForward()
			}
			continue
		}

		writeStatus("playing", sys.Name, display)
		if fromHistory {
			fmt.Printf("[Attract] Launching %s (%s) from history\n", display, sys.Name)
		} else {
			fmt.Printf("[Attract] Launching %s (%s)\n", display, sys.Name)
		}

		ensureMain("before launching " + display)
		if err := mister.LaunchGame(cfg, *sys, game.Path); err != nil {
			fmt.Printf("[Attract] failed to launch %s: %v\n", display, err)
			continue
		}
		if !fromHistory {
			history.add(game)
		}

		playTime := float64(minTime)
		if minTime != maxTime {
			playTime = float64(rand.Intn(maxTime-minTime+1) + minTime)
		}

		// Staticlist: leave a game SkipafterStatic seconds after the point
		// where its screen is known to go static.
		if useStatic(sys.Id) {
			if at, ok := lists.StaticTimes(sys.Id)[lists.Key(game.Name)]; ok {
				if limit := at + float64(cfg.List.SkipAfterStatic); limit < playTime {
					fmt.Printf("[Attract] Goes static at %.0fs, playing %.0fs\n", at, limit)
					playTime = limit
				}
			}
		}

		gen := 0
		if det != nil {
			gen = det.SetGame(sys.Id, game.Name)
		}

		// Back at the oldest game just keeps it playing.
		action := ""
		for {
			action = play(time.Duration(playTime*float64(time.Second)), det, gen, events, cfg)
			if action == "back" && !history.canGoBack() {
				fmt.Println("[Attract] This is the oldest game in the history")
				continue
			}
			break
		}

		switch action {
		case "play":
			if cfg.Attract.IdleRestart <= 0 || events == nil {
				fmt.Printf("[Attract] Playing %s, attract mode stopped\n", display)
				return nil
			}
			fmt.Printf("[Attract] Playing %s, attract mode resumes after %d min without input\n", display, cfg.Attract.IdleRestart)
			if det != nil {
				det.SetGame("", "") // pause: never skip or list a game being played
			}
			writeStatus("you're playing (resumes when idle)", sys.Name, display)
			switch cmd := waitIdle(events, time.Duration(cfg.Attract.IdleRestart)*time.Minute); cmd {
			case "menu", "search":
				openGamesMenu(cmd == "search")
				return nil
			case "stop":
				fmt.Println("[Attract] Stopped, back to the menu")
				_ = mister.LaunchMenu()
				return nil
			case "quit":
				return nil
			default:
				if query, ok := strings.CutPrefix(cmd, "find:"); ok {
					searchQueue = findGames(filtered, query)
					fmt.Printf("[Search] %d games for %q, playing them first\n", len(searchQueue), query)
				}
			}
			fmt.Println("[Attract] No input, attract mode resumes")
		case "stop":
			fmt.Println("[Attract] Stopped, back to the menu")
			_ = mister.LaunchMenu()
			return nil
		case "menu", "search":
			fmt.Printf("[Attract] Opening the games menu%s on the TV\n", map[bool]string{true: " (search)"}[action == "search"])
			openGamesMenu(action == "search")
			return nil
		case "quit": // from -launch / -random: leave the game running
			fmt.Println("[Attract] Stopped from outside")
			return nil
		case "back":
			if g, ok := history.back(); ok {
				next = &g
			}
		case "blacklist":
			if err := lists.Add(lists.Blacklist, sys.Id, game.Name); err == nil {
				fmt.Printf("[Attract] Added %s to the %s blacklist\n", display, sys.Id)
			} else {
				fmt.Printf("[Attract] Couldn't blacklist %s: %v\n", display, err)
			}
			removeGame(game.Path)
			history.remove(game.Path)
			stepForward()
		default: // "next", time up or a detector skip
			if query, ok := strings.CutPrefix(action, "find:"); ok {
				searchQueue = findGames(filtered, query)
				if len(searchQueue) == 0 {
					fmt.Printf("[Search] Nothing found for %q\n", query)
				} else {
					fmt.Printf("[Search] %d games for %q, playing them first\n", len(searchQueue), query)
				}
				next = nil // straight to the results
				break
			}
			stepForward()
		}
	}
	return fmt.Errorf("no games left to play")
}

// play waits out a game's play time. It ends early when the static
// detector asks to skip, or on a bound input, and returns the action that
// ended it: "" (time up or skipped), "next", "back", "play", "stop" or
// "blacklist". "stay" keeps the game on until the next action.
func play(d time.Duration, det *Detector, gen int, events <-chan input.Event, cfg *config.Config) string {
	timer := time.NewTimer(d)
	defer timer.Stop()
	timeUp := timer.C

	var skip <-chan int
	if det != nil {
		skip = det.Skip()

	}

	watchdog := time.NewTicker(3 * time.Second)
	defer watchdog.Stop()

	// Search key: a single press starts typing a search (no screen, the
	// keyboard types it), a double press opens the games menu.
	var searchWait <-chan time.Time
	var pausedTimer <-chan time.Time
	typing, typed := false, ""

	for {
		select {
		case <-watchdog.C:
			if !mister.MainRunning() {
				ensureMain("while a game was playing")
				return "" // on to the next game
			}
		case <-timeUp:
			return ""
		case g := <-skip:
			if g == gen && timeUp != nil {
				fmt.Println("[Attract] Detector says skip, next game")
				return ""
			}
		case cmd := <-remote:
			if cmd == "stay" {
				if timeUp != nil {
					timeUp = nil
					fmt.Println("[Remote] stay: staying on this game")
				} else {
					timer.Reset(d)
					timeUp = timer.C
					fmt.Println("[Remote] stay: timer back on")
				}
				continue
			}
			fmt.Printf("[Remote] %s\n", cmd)
			return cmd
		case <-searchWait:
			// A single press of the Search key: type a search.
			searchWait = nil
			typing, typed = true, ""
			pausedTimer = timeUp
			timeUp = nil
			fmt.Println("[Search] Type a search, Enter to play it, Esc to cancel")
		case ev := <-events:
			if typing && ev.Kind == "keyboard" {
				switch ev.Name {
				case "enter":
					typing = false
					if strings.TrimSpace(typed) != "" {
						return "find:" + strings.TrimSpace(typed)
					}
					timeUp = pausedTimer
				case "esc":
					typing = false
					timeUp = pausedTimer
					fmt.Println("[Search] Cancelled")
				case "backspace":
					if typed != "" {
						typed = typed[:len(typed)-1]
					}
					fmt.Printf("[Search] %s_\n", typed)
				case "space":
					typed += " "
					fmt.Printf("[Search] %s_\n", typed)
				default:
					if len(ev.Name) == 1 {
						typed += ev.Name
						fmt.Printf("[Search] %s_\n", typed)
					}
				}
				continue
			}
			action := actionFor(cfg, ev)
			if action == "search" {
				// Twice quickly: the games menu. Once: type a search.
				if searchWait != nil {
					searchWait = nil
					fmt.Printf("[Input] %s twice: games menu\n", ev)
					return "menu"
				}
				searchWait = time.After(400 * time.Millisecond)
				continue
			}
			switch action {
			case "":
				fmt.Printf("[Input] %s (not bound)\n", ev)
				continue
			case "stay":
				if timeUp != nil {
					timeUp = nil // no time limit, no detector skips
					fmt.Printf("[Input] %s: staying on this game\n", ev)
				} else {
					timer.Reset(d)
					timeUp = timer.C
					fmt.Printf("[Input] %s: timer back on\n", ev)
				}
				continue
			}
			fmt.Printf("[Input] %s: %s\n", ev, action)
			return action
		}
	}
}

// actionFor returns the attract action bound to an input, or what
// [Attract] OtherInput says to do with unbound input ("" = ignore).
func actionFor(cfg *config.Config, ev input.Event) string {
	if act := cfg.AttractControls[ev.Kind][strings.ToLower(ev.Name)]; act != "" {
		for _, known := range config.AttractActions {
			if act == known {
				return act
			}
		}
		fmt.Printf("[Input] Unknown action %q in SAM.ini for %s\n", act, ev.Name)
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(cfg.Attract.OtherInput)) {
	case "stop":
		return "stop"
	case "play":
		return "play"
	}
	return ""
}

// listFilter returns whether a list (Blacklist, Staticlist, Whitelist)
// applies to a system: its Use setting is on, the system is in its Include
// (empty = all) and not in its Exclude. Both accept groups.
func listFilter(use bool, include, exclude []string) func(systemID string) bool {
	inc, _ := games.ResolveSystems(include)
	exc, _ := games.ResolveSystems(exclude)
	return func(systemID string) bool {
		id := strings.ToLower(systemID)
		return use && (len(inc) == 0 || inc[id]) && !exc[id]
	}
}

// filterSystems keeps the games allowed by [Attract] Include and Exclude
// (system IDs and group names, see games.ResolveSystems), not ruled out by
// the [Attract.X] Folders/Files/Extensions/Paths rules, and allowed by the
// system's Whitelist and Blacklist when those are switched on in [List].
// An empty Include allows every system; Exclude always wins.
func filterSystems(files []gamesdb.FileInfo, cfg *config.Config) []gamesdb.FileInfo {
	include, unknownInc := games.ResolveSystems(cfg.Attract.Include)
	exclude, unknownExc := games.ResolveSystems(cfg.Attract.Exclude)
	for _, name := range append(unknownInc, unknownExc...) {
		fmt.Printf("[Attract] Unknown system or group in SAM.ini: %q\n", name)
	}

	// Include had names but none were recognised: allow nothing, like before,
	// rather than silently playing everything.
	includeAll := len(include) == 0 && len(unknownInc) == 0

	rules := gamesdb.NewRuleSet(cfg.AttractRules)
	useWhite := listFilter(cfg.List.UseWhitelist, cfg.List.WhitelistInclude, cfg.List.WhitelistExclude)
	useBlack := listFilter(cfg.List.UseBlacklist, cfg.List.BlacklistInclude, cfg.List.BlacklistExclude)

	// Each system's lists, read once. A system with no whitelist file isn't
	// limited by one.
	type sysLists struct{ white, black map[string]bool }
	cache := make(map[string]sysLists)
	listsFor := func(systemID string) sysLists {
		l, ok := cache[systemID]
		if !ok {
			if useWhite(systemID) {
				l.white = lists.Names(lists.Whitelist, systemID)
			}
			if useBlack(systemID) {
				l.black = lists.Names(lists.Blacklist, systemID)
			}
			cache[systemID] = l
		}
		return l
	}

	var out []gamesdb.FileInfo
	for _, f := range files {
		sys := strings.ToLower(f.SystemId)
		if !includeAll && !include[sys] {
			continue
		}
		if exclude[sys] || rules.Excludes(f) {
			continue
		}
		l := listsFor(f.SystemId)
		key := lists.Key(f.Name)
		if (l.white != nil && !l.white[key]) || l.black[key] {
			continue
		}
		out = append(out, f)
	}
	return out
}

// startInputDetectors starts the detectors turned on in [InputDetector]
// and returns their events (nil if all are off, which never delivers).
func startInputDetectors(cfg *config.Config) <-chan input.Event {
	opts := input.Options{
		Keyboard: cfg.InputDetector.Keyboard,
		Mouse:    cfg.InputDetector.Mouse,
		Joystick: cfg.InputDetector.Joystick,
	}
	if !opts.Keyboard && !opts.Mouse && !opts.Joystick {
		return nil
	}
	return input.Start(opts)
}

// remote is the command pipe of the running attract mode (see remote.go).
var remote <-chan string

// waitIdle returns once no input has arrived for d. Every press resets the
// wait, and none of them trigger attract actions. A remote command ends
// the wait early and is returned ("" when idle).
func waitIdle(events <-chan input.Event, d time.Duration) string {
	timer := time.NewTimer(d)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			return ""
		case cmd := <-remote:
			if cmd != "stay" && cmd != "blacklist" {
				return cmd
			}
		case <-events:
			if !timer.Stop() {
				<-timer.C
			}
			timer.Reset(d)
		}
	}
}

// ensureMain restarts MiSTer's main program if it has died (a bad file can
// crash it, leaving the core running with no menu or controls).
func ensureMain(when string) {
	restarted, err := mister.EnsureMain()
	switch {
	case err != nil:
		fmt.Printf("[Attract] MiSTer main program had stopped (%s) and couldn't be restarted: %v\n", when, err)
	case restarted:
		fmt.Printf("[Attract] MiSTer main program had stopped (%s), restarted it. Its messages: /tmp/MiSTer.log\n", when)
	}
}

// openGamesMenu opens the games menu on the TV, in a background helper
// ("gamesmenu -openmenu"), since attract mode ends straight after and the
// menu that opens takes over from it.
func openGamesMenu(search bool) {
	mode := "menu"
	if search {
		mode = "search"
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	cmd := exec.Command(exe, "-openmenu", mode)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err == nil {
		_ = cmd.Process.Release()
	}
}

// searchQueue holds the games a typed search found, played before the
// normal random list resumes.
var searchQueue []gamesdb.FileInfo

// findGames returns the games whose names contain every word of query.
func findGames(pool []gamesdb.FileInfo, query string) []gamesdb.FileInfo {
	words := strings.Fields(strings.ToLower(query))
	var out []gamesdb.FileInfo
	for _, g := range pool {
		name := strings.ToLower(g.Name)
		all := true
		for _, w := range words {
			if !strings.Contains(name, w) {
				all = false
				break
			}
		}
		if all {
			out = append(out, g)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}
