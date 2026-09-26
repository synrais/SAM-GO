package attract

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/synrais/SAMenu/pkg/config"
	"github.com/synrais/SAMenu/pkg/games"
	"github.com/synrais/SAMenu/pkg/gamesdb"
	"github.com/synrais/SAMenu/pkg/input"
	"github.com/synrais/SAMenu/pkg/mister"
	"github.com/synrais/SAMenu/pkg/video"
)

// StartAttractMode picks and plays random games endlessly using the existing menu database.
func StartAttractMode(cfg *config.Config, files []gamesdb.FileInfo) error {
	fmt.Println("=== Starting Attract Mode ===")
	if cfg.Attract.Mute {
		_ = mister.SetMute(true)
		defer func() { _ = mister.SetMute(false) }() // however attract mode ends
	}

	filtered := filterSystems(files, cfg)
	if o := cfg.Attract.Orientation; o != "" && !strings.EqualFold(o, "Both") {
		known, mras := 0, 0
		for _, f := range files {
			if strings.EqualFold(f.Ext, "mra") {
				mras++
				if f.Rotation != "" {
					known++
				}
			}
		}
		if mras > 0 && known == 0 {
			fmt.Println("[Attract] Orientation is set, but the games database has no arcade orientations yet: rebuild it (Options -> Game Database) so arcade games can play")
		} else if mras > 0 {
			fmt.Printf("[Attract] Arcade orientation: %s\n", o)
		}
	}
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

	history := loadTimeline() // carries on from the last session this boot
	defer history.save()
	var next *gamesdb.FileInfo // from the history; nil = pick a random game
	gamesSinceVideo, videoOrder := 0, 0

	// How games are picked: [Attract] Selection, NoRepeats, MixSystems,
	// OneVersion and [Weights] (picker.go). Building the picker sorts the
	// whole library, which takes a few seconds on the MiSTer, so it's built
	// in the background while the first game (a quick pick) loads.
	fmt.Printf("[Attract] Picking: %s\n", cfg.Attract.Selection)
	pickerReady := make(chan *Picker, 1)
	go func(pool []gamesdb.FileInfo) {
		pickerReady <- NewPicker(cfg, pool, false)
	}(append([]gamesdb.FileInfo(nil), filtered...))
	var picker *Picker
	var removedEarly []string // removed before the picker was ready
	quickDone := false        // the first game has been quick-picked
	getPicker := func(wait bool) *Picker {
		if picker == nil {
			if wait {
				picker = <-pickerReady
			} else {
				select {
				case picker = <-pickerReady:
				default:
					return nil
				}
			}
			for _, p := range removedEarly {
				picker.Remove(p)
			}
			removedEarly = nil
		}
		return picker
	}

	// removeGame drops a game from the pool attract mode picks from.
	removeGame := func(path string) {
		if p := getPicker(false); p != nil {
			p.Remove(path)
		} else {
			removedEarly = append(removedEarly, path)
		}
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
		// [Video] AttractEvery: a video between games, every N games.
		if every := cfg.Video.AttractEvery; every > 0 && gamesSinceVideo >= every && next == nil && len(searchQueue) == 0 {
			gamesSinceVideo = 0
			switch action := playAttractVideo(cfg, det, events, &videoOrder); action {
			case "stop":
				fmt.Println("[Attract] Stopped, back to the menu")
				_ = mister.LaunchMenu()
				return nil
			case "menu", "search":
				openGamesMenu(action == "search")
				return nil
			case "quit":
				fmt.Println("[Attract] Stopped from outside")
				return nil
			}
		}

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
		case !quickDone && getPicker(false) == nil:
			// The first game of this session, while the picker's still
			// being built.
			game, quickDone = quickPick(filtered), true
		default:
			g, ok := getPicker(true).Next()
			if !ok {
				filtered = nil // nothing left to pick
				continue
			}
			game = g
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

		display := game.FileName()

		// The blacklist can grow while we run (the detector adds to it).
		if useBlack(sys.Id) && ListNames(Blacklist, sys.Id)[ListKey(game.Name)] {
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

		waitWhileBusy()
		ensureMain("before launching " + display)
		if err := mister.LaunchGame(cfg, *sys, game.Path); err != nil {
			fmt.Printf("[Attract] failed to launch %s: %v\n", display, err)
			continue
		}
		if !fromHistory {
			history.add(game)
		}
		gamesSinceVideo++

		playTime := float64(minTime)
		if minTime != maxTime {
			playTime = float64(rand.Intn(maxTime-minTime+1) + minTime)
		}

		// Staticlist: leave a game SkipafterStatic seconds after the point
		// where its screen is known to go static.
		if useStatic(sys.Id) {
			if at, ok := StaticTimes(sys.Id)[ListKey(game.Name)]; ok {
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
			// Attract mode ends; the idle watcher (Startup -> When idle,
			// with games in Where) starts it again once the game is left
			// alone, carrying on with this history.
			fmt.Printf("[Attract] Playing %s, attract mode stopped\n", display)
			return nil
		case "stop":
			fmt.Println("[Attract] Stopped, back to the menu")
			_ = mister.LaunchMenu()
			return nil
		case "menu", "search":
			if action == "search" {
				fmt.Println("[Attract] Opening SAMenu (search) on the TV")
			} else {
				fmt.Println("[Attract] Opening SAMenu on the TV")
			}
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
			if err := AddToList(Blacklist, sys.Id, game.Name); err == nil {
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
	// keyboard types it), a double press opens SAMenu.
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
				// Twice quickly: SAMenu. Once: type a search.
				if searchWait != nil {
					searchWait = nil
					fmt.Printf("[Input] %s twice: SAMenu\n", ev)
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
		fmt.Printf("[Input] Unknown action %q in SAMenu.ini for %s\n", act, ev.Name)
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
		fmt.Printf("[Attract] Unknown system or group in SAMenu.ini: %q\n", name)
	}

	// Include had names but none were recognised: allow nothing, like before,
	// rather than silently playing everything.
	includeAll := len(include) == 0 && len(unknownInc) == 0

	rules := gamesdb.NewRuleSet(cfg.AttractRules)
	skipTags := gamesdb.NewTagFilter(cfg.Attract.SkipTags) // Beta, Proto...
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
				l.white = ListNames(Whitelist, systemID)
			}
			if useBlack(systemID) {
				l.black = ListNames(Blacklist, systemID)
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
		if exclude[sys] || rules.Excludes(f) || skipTags.Matches(f.Name) {
			continue
		}
		// [Attract] Orientation: arcade games only.
		if strings.EqualFold(f.Ext, "mra") && !gamesdb.RotationMatches(f.Rotation, cfg.Attract.Orientation) {
			continue
		}
		// Lists: the title's key is only worked out for systems that have
		// a list (normalising every title is slow on the MiSTer).
		if l := listsFor(f.SystemId); l.white != nil || len(l.black) > 0 {
			key := ListKey(f.Name)
			if (l.white != nil && !l.white[key]) || l.black[key] {
				continue
			}
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

// playAttractVideo plays one video from [Video] Playlist between games. The
// attract controls work on it: next (or the video ending) goes on to the
// next game; stop, SAMenu and remote stop/quit end attract mode.
func playAttractVideo(cfg *config.Config, det *Detector, events <-chan input.Event, order *int) string {
	vids := video.Videos(cfg.Video.Playlist)
	if len(vids) == 0 {
		fmt.Printf("[Video] No videos in %s, skipping\n", filepath.Join(video.Folder, cfg.Video.Playlist))
		return ""
	}
	var file string
	if strings.EqualFold(cfg.Video.Playback, "In order") {
		file = vids[*order%len(vids)]
		*order++
	} else {
		file = vids[rand.Intn(len(vids))]
	}
	name := filepath.Base(file)
	if det != nil {
		det.SetGame("", "") // the detector only watches games
	}

	// Only the menu core shows the Linux screen.
	fmt.Printf("[Video] Playing %s\n", name)
	writeStatus("playing video", "Video", name)
	video.EnsureMenuCore() // loads the menu core over the game

	bg, err := video.StartBackground(file)
	if err != nil {
		fmt.Printf("[Video] Couldn't play %s: %v\n", name, err)
		return ""
	}
	defer bg.Stop()

	for {
		select {
		case <-bg.Done:
			return ""
		case cmd := <-remote:
			switch cmd {
			case "next":
				fmt.Println("[Remote] next: skipping the video")
				return ""
			case "stop", "menu", "search", "quit":
				fmt.Printf("[Remote] %s\n", cmd)
				return cmd
			}
		case ev := <-events:
			switch action := actionFor(cfg, ev); action {
			case "next":
				fmt.Printf("[Input] %s: skipping the video\n", ev)
				return ""
			case "stop":
				fmt.Printf("[Input] %s: stop\n", ev)
				return "stop"
			case "search":
				fmt.Printf("[Input] %s: SAMenu\n", ev)
				return "menu"
			}
		}
	}
}

// openGamesMenu opens SAMenu on the TV, in a background helper
// ("SAMenu -openmenu"), since attract mode ends straight after and the
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

// quickPick picks the first game while the full picker is still being
// built: a few random games are drawn from the pool, then one of their
// systems is chosen evenly and one of its games, so a system with a huge
// library doesn't get the first game just for being huge.
func quickPick(pool []gamesdb.FileInfo) gamesdb.FileInfo {
	bySystem := map[string][]gamesdb.FileInfo{}
	var systems []string
	for i := 0; i < 64; i++ {
		f := pool[rand.Intn(len(pool))]
		if bySystem[f.SystemId] == nil {
			systems = append(systems, f.SystemId)
		}
		bySystem[f.SystemId] = append(bySystem[f.SystemId], f)
	}
	from := bySystem[systems[rand.Intn(len(systems))]]
	return from[rand.Intn(len(from))]
}

// waitWhileBusy holds the next launch while a script or updater runs
// (e.g. update_all): loading a core would end it half way.
func waitWhileBusy() {
	what, busy := mister.Busy()
	if !busy {
		return
	}
	fmt.Printf("[Attract] Waiting: something is running (%s)\n", what)
	writeStatus("waiting for a script to finish", "", what)
	for busy {
		time.Sleep(2 * time.Second)
		_, busy = mister.Busy()
	}
	fmt.Println("[Attract] Finished, carrying on")
}
