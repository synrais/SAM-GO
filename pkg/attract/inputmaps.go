package attract

import (
	"fmt"
	"os"

	"github.com/synrais/SAM-GO/pkg/config"
)

// Generic function type for mapped inputs
type InputAction func()

// --- Attract Mode Input Map (grouped by device type) ---
func AttractInputMap(cfg *config.UserConfig, inputCh <-chan string) map[string]InputAction {
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
			if prev, ok := PlayBack(); ok {
				fmt.Println("[Attract] Keyboard ← back in history.")
				Run([]string{prev})
			}
		},
		"right": func() {
			if next, ok := Next(); ok {
				fmt.Println("[Attract] Keyboard → forward in history.")
				Run([]string{next})
			}
		},

		// ----------------------------
		// Controller Buttons
		// ----------------------------
		"button1": func() {
			if next, ok := Next(); ok {
				fmt.Println("[Attract] Button1 → forward in history.")
				Run([]string{next})
			}
		},
		"button2": func() {
			if prev, ok := PlayBack(); ok {
				fmt.Println("[Attract] Button2 ← back in history.")
				Run([]string{prev})
			}
		},

		// ----------------------------
		// Touch / Gestures
		// ----------------------------
		"swipe-right": func() {
			if next, ok := Next(); ok {
				fmt.Println("[Attract] Swipe → forward in history.")
				Run([]string{next})
			}
		},
		"swipe-left": func() {
			if prev, ok := PlayBack(); ok {
				fmt.Println("[Attract] Swipe ← back in history.")
				Run([]string{prev})
			}
		},

		// ----------------------------
		// Analog Axis
		// ----------------------------
		"axis-right": func() {
			if next, ok := Next(); ok {
				fmt.Println("[Attract] Axis → forward in history.")
				Run([]string{next})
			}
		},
		"axis-left": func() {
			if prev, ok := PlayBack(); ok {
				fmt.Println("[Attract] Axis ← back in history.")
				Run([]string{prev})
			}
		},
	}
}


// --- Search Mode Input Map ---
func SearchInputMap(sb *strings.Builder, candidates *[]GameEntry, idx *int, index []GameEntry, inputCh <-chan string) map[string]InputAction {
	return map[string]InputAction{
		"space": func() {
			sb.WriteRune(' ')
			fmt.Printf("[SEARCH] Current query: %q\n", sb.String())
		},
		"enter": func() {
			qn, qext := NormalizeQuery(sb.String())
			if qn != "" {
				fmt.Printf("[SEARCH] Searching... (%d titles)\n", len(index))
				*candidates = findMatches(qn, qext, index)
				if len(*candidates) > 0 {
					*idx = 0
					launchGame((*candidates)[*idx])
				} else {
					fmt.Println("[SEARCH] No match found")
				}
			}
			sb.Reset()
			fmt.Println("[SEARCH] Ready. Use ←/→ to browse, ESC to exit.")
		},
		"esc": func() {
			fmt.Println("[SEARCH] Exiting search mode (Attract resumed).")
			// caller breaks loop
		},
		"backspace": func() {
			s := sb.String()
			if len(s) > 0 {
				sb.Reset()
				sb.WriteString(s[:len(s)-1])
			}
			fmt.Printf("[SEARCH] Current query: %q\n", sb.String())
		},
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
