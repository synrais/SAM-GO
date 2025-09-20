package attract

import (
	"fmt"
	"os"
	"strings"
	"time"
	"math/rand"

	"github.com/synrais/SAM-GO/pkg/config"
)

// Generic function type for mapped inputs
type InputAction func()

// --- Attract Mode Input Map (grouped by device type) ---
func AttractInputMap(cfg *config.UserConfig, r *rand.Rand, timer *time.Timer, inputCh <-chan string) map[string]InputAction {
	return map[string]InputAction{

		// ----------------------------
		// Keyboard
		// ----------------------------
		"esc": func() {
			fmt.Println("[Attract] Exiting attract mode.")
			os.Exit(0)
		},
		"space": func() {
			fmt.Println("[Attract] Skipped current game.")
		},
		"`": func() {
			fmt.Println("[Attract] Entering search mode...")
			SearchAndPlay(inputCh)
			fmt.Println("[Attract] Resuming attract mode.")
		},
		"left": func() {
			if prev, ok := PlayBack(timer, cfg, r); ok {
				fmt.Println("[Attract] Keyboard ← back in history.")
				Run([]string{prev})
			}
		},
		"right": func() {
			if next, ok := Next(timer, cfg, r); ok {
				fmt.Println("[Attract] Keyboard → forward in history.")
				Run([]string{next})
			} else {
				fmt.Println("[Attract] No forward entry, starting new attract pick.")
				Play("", timer, cfg, r)
			}
		},

		// ----------------------------
		// Controller Buttons
		// ----------------------------
		"button1": func() {
			if next, ok := Next(timer, cfg, r); ok {
				fmt.Println("[Attract] Button1 → forward in history.")
				Run([]string{next})
			} else {
				fmt.Println("[Attract] No forward entry, starting new attract pick.")
				Play("", timer, cfg, r)
			}
		},
		"button2": func() {
			if prev, ok := PlayBack(timer, cfg, r); ok {
				fmt.Println("[Attract] Button2 ← back in history.")
				Run([]string{prev})
			}
		},

		// ----------------------------
		// Touch / Gestures
		// ----------------------------
		"swipe-right": func() {
			if next, ok := Next(timer, cfg, r); ok {
				fmt.Println("[Attract] Swipe → forward in history.")
				Run([]string{next})
			} else {
				fmt.Println("[Attract] No forward entry, starting new attract pick.")
				Play("", timer, cfg, r)
			}
		},
		"swipe-left": func() {
			if prev, ok := PlayBack(timer, cfg, r); ok {
				fmt.Println("[Attract] Swipe ← back in history.")
				Run([]string{prev})
			}
		},

		// ----------------------------
		// Analog Axis
		// ----------------------------
		"axis-right": func() {
			if next, ok := Next(timer, cfg, r); ok {
				fmt.Println("[Attract] Axis → forward in history.")
				Run([]string{next})
			} else {
				fmt.Println("[Attract] No forward entry, starting new attract pick.")
				Play("", timer, cfg, r)
			}
		},
		"axis-left": func() {
			if prev, ok := PlayBack(timer, cfg, r); ok {
				fmt.Println("[Attract] Axis ← back in history.")
				Run([]string{prev})
			}
		},
	}
}

// --- Search Mode Input Map (grouped by device type) ---
func SearchInputMap(sb *strings.Builder, candidates *[]GameEntry, idx *int, index []GameEntry, inputCh <-chan string) map[string]InputAction {
	return map[string]InputAction{

		// ----------------------------
		// Keyboard
		// ----------------------------
		"space": func() {
			sb.WriteRune(' ')
			fmt.Printf("[SEARCH] Current query: %q\n", sb.String())
		},
		"backspace": func() {
			s := sb.String()
			if len(s) > 0 {
				sb.Reset()
				sb.WriteString(s[:len(s)-1])
			}
			fmt.Printf("[SEARCH] Current query: %q\n", sb.String())
		},
		"enter": func() {
			query := sb.String()
			if query != "" {
				fmt.Printf("[SEARCH] Searching for: %q (%d titles)\n", query, len(index))
				*candidates = findMatches(query, "", index) // raw query
				if len(*candidates) > 0 {
					*idx = 0
					launchGame((*candidates)[*idx])
				} else {
					fmt.Println("[SEARCH] No match found")
				}
			}
			sb.Reset()
			fmt.Println("[SEARCH] Ready. Use left/right to browse, esc to exit.")
		},
		"esc": func() {
			fmt.Println("[SEARCH] Exiting search mode (Attract resumed).")
		},

		// ----------------------------
		// Navigation
		// ----------------------------
		"left": func() {
			if len(*candidates) > 0 && *idx > 0 {
				*idx--
				launchGame((*candidates)[*idx])
			}
		},
		"right": func() {
			if len(*candidates) > 0 && *idx < len(*candidates)-1 {
				*idx++
				launchGame((*candidates)[*idx])
			}
		},
	}
}
