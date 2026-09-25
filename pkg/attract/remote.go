package attract

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"
)

// Remote control
//
// A running attract mode reads commands from a pipe, so it can be driven
// from outside ("gamesmenu.sh -next", SSH, Zaparoo, Home Assistant...),
// and writes a status file saying what it's doing ("gamesmenu.sh -status").

const (
	CommandPipe = "/tmp/gamesmenu_attract.cmd"
	StateFile   = "/tmp/gamesmenu_attract.status"
	PidFile     = "/tmp/gamesmenu_attract.pid"
)

// Commands a running attract mode accepts.
var Commands = []string{"next", "back", "play", "stay", "stop", "blacklist", "menu", "search", "quit"}

// startRemote creates the command pipe and returns the commands read
// from it. Attract mode keeps the pipe open itself, so writers never see
// it end, and a writer finding no reader knows attract mode isn't running.
func startRemote() <-chan string {
	out := make(chan string, 8)
	_ = os.Remove(CommandPipe)
	if err := syscall.Mkfifo(CommandPipe, 0666); err != nil {
		fmt.Printf("[Attract] no remote control: %v\n", err)
		return out
	}
	// Open it before returning, so commands work as soon as attract mode
	// has started.
	f, err := os.OpenFile(CommandPipe, os.O_RDWR, 0)
	if err != nil {
		fmt.Printf("[Attract] no remote control: %v\n", err)
		return out
	}
	_ = os.WriteFile(PidFile, []byte(fmt.Sprint(os.Getpid())), 0644)
	go func() {
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			if cmd := strings.ToLower(strings.TrimSpace(sc.Text())); cmd != "" {
				out <- cmd
			}
		}
	}()
	return out
}

// stopRemote removes the pipe and files when attract mode ends.
func stopRemote() {
	_ = os.Remove(CommandPipe)
	_ = os.Remove(PidFile)
	_ = os.Remove(StateFile)
}

// writeStatus records what attract mode is doing, for -status.
func writeStatus(state, system, game string) {
	_ = os.WriteFile(StateFile, []byte(fmt.Sprintf("state=%s\nsystem=%s\ngame=%s\nsince=%s\n",
		state, system, game, time.Now().Format("15:04:05"))), 0644)
}

// SendCommand sends a command to a running attract mode. It reports an
// error if attract mode isn't running.
func SendCommand(cmd string) error {
	f, err := os.OpenFile(CommandPipe, os.O_WRONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return fmt.Errorf("attract mode isn't running")
	}
	defer f.Close()
	_, err = f.WriteString(cmd + "\n")
	return err
}

// Running reports whether attract mode is running.
func Running() bool {
	b, err := os.ReadFile(PidFile)
	if err != nil {
		return false
	}
	c, err := os.ReadFile("/proc/" + strings.TrimSpace(string(b)) + "/cmdline")
	return err == nil && strings.Contains(string(c), "-attract")
}

// Status describes a running attract mode, or reports it isn't running.
func Status() string {
	b, err := os.ReadFile(PidFile)
	if err != nil {
		return "Attract mode isn't running."
	}
	pid := strings.TrimSpace(string(b))
	if c, err := os.ReadFile("/proc/" + pid + "/cmdline"); err != nil || !strings.Contains(string(c), "-attract") {
		return "Attract mode isn't running."
	}
	s, _ := os.ReadFile(StateFile)
	return "Attract mode is running (process " + pid + ").\n" + strings.TrimSpace(string(s))
}
