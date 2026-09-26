package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"time"
)

// -------------------------
// Timestamped background log
// -------------------------
//
// Attract mode started in the background (from the menu, -attract -bg, the
// idle watcher or boot) writes its log to /tmp/SAMenu_attract.log. Each
// line gets the time and the seconds since it was asked to start, e.g.
//
//	07:02:11.4  +2.3s  [Attract] Picking: Balanced
//
// so slow steps show up. The starter passes the start time in
// SAMENU_LOG_T0 (Unix nanoseconds).

const logT0Env = "SAMENU_LOG_T0"

// logStamp is how a log line starts: the time and the seconds since t0.
func logStamp(t0 time.Time) string {
	now := time.Now()
	return fmt.Sprintf("%s.%d  +%.1fs  ", now.Format("15:04:05"), now.Nanosecond()/1e8, now.Sub(t0).Seconds())
}

// timestampOutput puts the time on every line this process prints, when it
// was started with SAMENU_LOG_T0 set.
func timestampOutput() {
	ns, err := strconv.ParseInt(os.Getenv(logT0Env), 10, 64)
	if err != nil {
		return
	}
	t0 := time.Unix(0, ns)
	out := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return
	}
	os.Stdout, os.Stderr = w, w
	go func() {
		sc := bufio.NewScanner(r)
		for sc.Scan() {
			fmt.Fprintf(out, "%s%s\n", logStamp(t0), sc.Text())
		}
	}()
}

// startedBy names what's starting attract mode in the background, for the
// first log line.
func startedBy() string {
	switch {
	case menuLock != nil:
		return "SAMenu's Options menu"
	case len(os.Args) > 1 && os.Args[1] == "-idlewatch":
		return "the idle watcher"
	case len(os.Args) > 1 && os.Args[1] == "-boot":
		return "startup"
	default:
		return "-attract -bg"
	}
}
