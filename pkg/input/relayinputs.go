package input

import (
	"fmt"
	"regexp"
	"strings"
	"strconv"
)

// RelayInputs starts listeners for keyboard, mouse and joystick input.
// It forwards all normalized events into the provided callback channel.
// Other packages (search, attract, etc.) can consume them as they like.
func RelayInputs(out chan<- string) {
	// ---------------- KEYBOARD ----------------
	go func() {
		re := regexp.MustCompile(`<([^>]+)>`)
		for line := range StreamKeyboards() {
			fmt.Println("[KEY]", line)

			// Extract <tokens>
			for _, m := range re.FindAllStringSubmatch(line, -1) {
				key := strings.ToLower(m[1])
				out <- key
			}

			// Plain characters (letters, numbers…)
			clean := re.ReplaceAllString(line, "")
			for _, r := range clean {
				if r == '\n' || r == '\r' || r == ' ' {
					continue
				}
				out <- string(r)
			}
		}
	}()

	// ---------------- MOUSE ----------------
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

	// ---------------- JOYSTICK ----------------
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
							out <- kv[0]
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
								out <- kv[0] + "-"
							} else if v > 20000 {
								out <- kv[0] + "+"
							}
						}
					}
				}
			}
		}
	}()
}

func parseAxisValue(s string) int {
	end := strings.IndexAny(s, ", ")
	if end >= 0 {
		s = s[:end]
	}
	v, _ := strconv.Atoi(strings.TrimSpace(s))
	return v
}
