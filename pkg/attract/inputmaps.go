package attract

import (
	"fmt"
	"os"
	"strings"
	"math/rand"

	"github.com/synrais/SAM-GO/pkg/config"
)

// Generic function type for mapped inputs
type InputAction func()

// --- Attract Mode Input Map (grouped by device type) ---
func AttractInputMap(cfg *config.UserConfig, r *rand.Rand, inputCh <-chan string) map[string]InputAction {
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
			if _, ok := Next(cfg, r); ok {
				// ticker reset handled inside Next
			}
		},
		"`": func() {
			fmt.Println("[Attract] Entering search mode...")
			SearchAndPlay(inputCh)
			fmt.Println("[Attract] Resuming attract mode.")
		},
		"left": func() {
			if _, ok := Back(cfg, r); ok {
				fmt.Println("[Attract] Keyboard ← back in history.")
			}
		},
		"right": func() {
			if _, ok := Next(cfg, r); ok {
				fmt.Println("[Attract] Keyboard → forward/new game.")
			}
		},

		// ----------------------------
		// Test Menus (Hotkeys 1–9)
		// ----------------------------
		"1": func() { GameMenu1() },
		"2": func() { GameMenu2() },
		"3": func() { GameMenu3() },
		"4": func() { GameMenu4() },
		"5": func() { GameMenu5() },
		"6": func() { GameMenu6() },
		"7": func() { GameMenu7() },
		"8": func() { GameMenu8() },
		"9": func() { GameMenu9() },

		// ----------------------------
		// Controller Buttons
		// ----------------------------
		"button1": func() {
			if _, ok := Next(cfg, r); ok {
				fmt.Println("[Attract] Button1 → forward/new game.")
			}
		},
		"button2": func() {
			if _, ok := Back(cfg, r); ok {
				fmt.Println("[Attract] Button2 ← back in history.")
			}
		},

		// ----------------------------
		// Touch / Gestures
		// ----------------------------
		"swipe-right": func() {
			if _, ok := Next(cfg, r); ok {
				fmt.Println("[Attract] Swipe → forward/new game.")
			}
		},
		"swipe-left": func() {
			if _, ok := Back(cfg, r); ok {
				fmt.Println("[Attract] Swipe ← back in history.")
			}
		},

		// ----------------------------
		// Analog Axis
		// ----------------------------
		"axis-right": func() {
			if _, ok := Next(cfg, r); ok {
				fmt.Println("[Attract] Axis → forward/new game.")
			}
		},
		"axis-left": func() {
			if _, ok := Back(cfg, r); ok {
				fmt.Println("[Attract] Axis ← back in history.")
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
