package input

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/synrais/SAM-GO/pkg/config"
)

// RelayInputs starts listeners for keyboard, mouse and joystick input
// based on configuration. When a configured event is seen the matching
// action is executed. If no custom action is configured, the default
// behaviour uses the provided back and next callbacks.
func RelayInputs(cfg *config.UserConfig, back func(), next func()) {
	if cfg == nil {
		return
	}

	// ---------------- KEYBOARD ----------------
	if cfg.InputDetector.Keyboard {
		go func() {
			re := regexp.MustCompile(`<([^>]+)>`)
			for line := range StreamKeyboards() {
				fmt.Println("[KEY]", line) // raw debug

				// Extract <tokens> like <enter>, <escape>, etc.
				for _, m := range re.FindAllStringSubmatch(line, -1) {
					key := strings.ToLower(m[1])
					performAction(cfg.InputDetector.KeyboardMap, key, back, next)
				}

				// Plain characters (a, b, c…)
				clean := re.ReplaceAllString(line, "")
				for _, r := range clean {
					if r == '\n' || r == '\r' || r == ' ' {
						continue
					}
					performAction(cfg.InputDetector.KeyboardMap, string(r), back, next)
				}
			}
		}()
	}

	// ---------------- MOUSE ----------------
	if cfg.InputDetector.Mouse {
		go func() {
			for ev := range StreamMouse() {
				if ev.DX < 0 {
					performAction(cfg.InputDetector.MouseMap, "swipeleft", back, next)
				} else if ev.DX > 0 {
					performAction(cfg.InputDetector.MouseMap, "swiperight", back, next)
				}
				if ev.DY < 0 {
					performAction(cfg.InputDetector.MouseMap, "swipedown", back, next)
				} else if ev.DY > 0 {
					performAction(cfg.InputDetector.MouseMap, "swipeup", back, next)
				}
				for _, b := range ev.Buttons {
					switch b {
					case "L":
						performAction(cfg.InputDetector.MouseMap, "left", back, next)
					case "M":
						performAction(cfg.InputDetector.MouseMap, "middle", back, next)
					case "R":
						performAction(cfg.InputDetector.MouseMap, "right", back, next)
					}
				}
			}
		}()
	}

	// ---------------- JOYSTICK ----------------
	if cfg.InputDetector.Joystick {
		go func() {
			for line := range StreamJoysticks() {
				l := strings.ToLower(line)
				if bi := strings.Index(l, "buttons["); bi != -1 {
					ei := strings.Index(l[bi:], "]")
					if ei != -1 {
						part := l[bi+len("buttons[") : bi+ei]
						for _, item := range strings.Split(part, ",") {
							kv := strings.Split(strings.TrimSpace(item), "=")
							if len(kv) == 2 && kv[1] == "p" {
								performAction(cfg.InputDetector.JoystickMap, kv[0], back, next)
							}
						}
					}
				}
				if ai := strings.Index(l, "axes["); ai != -1 {
					ei := strings.Index(l[ai:], "]")
					if ei != -1 {
						part := l[ai+len("axes[") : ai+ei]
						for _, item := range strings.Split(part, ",") {
							kv := strings.Split(strings.TrimSpace(item), "=")
							if len(kv) == 2 {
								v := parseAxisValue(kv[1])
								if v < -20000 {
									performAction(cfg.InputDetector.JoystickMap, kv[0]+"-", back, next)
								} else if v > 20000 {
									performAction(cfg.InputDetector.JoystickMap, kv[0]+"+", back, next)
								}
							}
						}
					}
				}
			}
		}()
	}
}

func parseAxisValue(s string) int {
	end := strings.IndexAny(s, ", ")
	if end >= 0 {
		s = s[:end]
	}
	v, _ := strconv.Atoi(strings.TrimSpace(s))
	return v
}

func performAction(m map[string]string, key string, back, next func()) {
	// Backtick always (re)starts search mode
	if key == "`" {
		fmt.Println("[INFO] Starting search mode…")
		SearchAndPlay()
		fmt.Println("[INFO] Exited search mode.")
		return
	}

	// Mapped commands
	if m != nil {
		if cmd, ok := m[key]; ok {
			runCommand(cmd, back, next)
			return
		}
	}

	// Defaults: left/back, right/next
	switch key {
	case "left", "dpleft", "leftx-", "rightx-":
		if back != nil {
			back()
		}
	case "right", "dpright", "leftx+", "rightx+":
		if next != nil {
			next()
		}
	}
}

func runCommand(cmd string, back, next func()) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return
	}
	switch parts[0] {
	case "back":
		if back != nil {
			back()
		}
	case "next":
		if next != nil {
			next()
		}
	case "search":
		fmt.Println("[INFO] Starting search mode (via runCommand)…")
		SearchAndPlay()
		fmt.Println("[INFO] Exited search mode (via runCommand).")
	}
	c := exec.Command(parts[0], parts[1:]...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	_ = c.Start()
}
