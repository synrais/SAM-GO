package input

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/synrais/SAM-GO/pkg/config"
)

// InputEvent is a normalized string event (like "left", "`", "enter").
type InputEvent string

// StreamEvents relays inputs (keyboard/mouse/joystick) into a channel of normalized InputEvents.
func StreamEvents(cfg *config.UserConfig) <-chan InputEvent {
	out := make(chan InputEvent, 10)

	// ---------------- KEYBOARD ----------------
	if cfg.InputDetector.Keyboard {
		go func() {
			re := regexp.MustCompile(`<([^>]+)>`)
			for line := range StreamKeyboards() {
				fmt.Println("[KEY]", line)

				// Extract <tokens> like <enter>, <escape>, <f9>, etc.
				for _, m := range re.FindAllStringSubmatch(line, -1) {
					key := strings.ToLower(m[1])
					out <- InputEvent(key)
				}

				// Also forward any plain characters (a, b, c…) as events
				clean := re.ReplaceAllString(line, "")
				for _, r := range clean {
					if r == '\n' || r == '\r' || r == ' ' {
						continue
					}
					out <- InputEvent(string(r))
				}
			}
		}()
	}

	// ---------------- MOUSE ----------------
	if cfg.InputDetector.Mouse {
		go func() {
			for ev := range StreamMouse() {
				if ev.DX < 0 {
					out <- "swipeleft"
				} else if ev.DX > 0 {
					out <- "swiperight"
				}
				if ev.DY < 0 {
					out <- "swipedown"
				} else if ev.DY > 0 {
					out <- "swipeup"
				}
				for _, b := range ev.Buttons {
					switch b {
					case "L":
						out <- "left"
					case "M":
						out <- "middle"
					case "R":
						out <- "right"
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
								out <- InputEvent(kv[0]) // e.g. "a", "b"
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
									out <- InputEvent(kv[0] + "-")
								} else if v > 20000 {
									out <- InputEvent(kv[0] + "+")
								}
							}
						}
					}
				}
			}
		}()
	}

	return out
}

// parseAxisValue extracts an int from joystick axis string.
func parseAxisValue(s string) int {
	end := strings.IndexAny(s, ", ")
	if end >= 0 {
		s = s[:end]
	}
	v, _ := strconv.Atoi(strings.TrimSpace(s))
	return v
}
