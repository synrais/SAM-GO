package main

import (
	"time"

	gc "github.com/rthornton128/goncurses"
)

// withSpinner runs work in the background and calls draw with the next
// spinner frame about 10 times a second until it finishes, then returns its
// error. Anything work sets is safe to read afterwards (the done channel
// hands it over); progress read while it runs needs its own lock.
func withSpinner(work func() error, draw func(spin string)) error {
	done := make(chan error, 1)
	go func() { done <- work() }()

	const frames = `|/-\`
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for i := 0; ; i++ {
		draw(string(frames[i%len(frames)]))
		_ = gc.Update()
		select {
		case err := <-done:
			return err
		case <-tick.C:
		}
	}
}

// clearScreen blanks the screen before the next window is drawn.
func clearScreen(stdscr *gc.Window) {
	stdscr.Clear()
	stdscr.Refresh()
}
